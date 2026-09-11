package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/pathutil"
	"github.com/gorecodecom/goregraph/internal/scan"
)

const (
	// MaxSourceReadRequestBytes bounds the serialized caller request.
	MaxSourceReadRequestBytes = 64 * 1024
	// MaxSourceReadResultBytes includes JSON structure and cumulative receipts.
	MaxSourceReadResultBytes   = 24 * 1024
	maxSourceReadReceiptBytes  = 4096
	maxSourceReadReceiptRanges = 64
)

// SourceReadRange is an inclusive pair of one-based source line numbers.
type SourceReadRange [2]int

// UnmarshalJSON rejects missing or extra endpoints rather than truncating them.
func (r *SourceReadRange) UnmarshalJSON(body []byte) error {
	var values []int
	if err := json.Unmarshal(body, &values); err != nil {
		return err
	}
	if len(values) != 2 {
		return fmt.Errorf("source range requires exactly two integers")
	}
	*r = SourceReadRange{values[0], values[1]}
	return nil
}

// ReadSourceRequest names indexed source paths relative to Root.
type ReadSourceRequest struct {
	Root  string                  `json:"-"`
	Files []SourceReadFileRequest `json:"files"`
}

// SourceReadFileRequest selects bounded ranges or matching lines with prior receipts.
type SourceReadFileRequest struct {
	Path   string                 `json:"path"`
	Ranges []SourceReadRange      `json:"ranges,omitempty"`
	Find   *SourceReadFindRequest `json:"find,omitempty"`
	Seen   []string               `json:"seen,omitempty"`
}

// ReadSourceResult contains one entry per canonical source path, in request order.
type ReadSourceResult struct {
	Files []SourceReadFileResult `json:"files"`
}

// SourceReadFileResult describes actual delivery, prior delivery and EOF gaps.
type SourceReadFileResult struct {
	Path            string                      `json:"path"`
	Sections        []SourceReadSection         `json:"sections"`
	SkippedRanges   []SourceReadRange           `json:"skipped_ranges"`
	OutputLimited   bool                        `json:"output_limited,omitempty"`
	Receipt         string                      `json:"receipt,omitempty"`
	EOFRanges       []SourceReadRange           `json:"eof_ranges,omitempty"`
	IgnoredReceipts int                         `json:"ignored_receipts,omitempty"`
	Find            *SourceReadFindResult       `json:"find,omitempty"`
	FindResults     []SourceReadBatchFindResult `json:"find_results,omitempty"`
}

// SourceReadSection contains only the numbered lines in its inclusive bounds.
type SourceReadSection struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// ReadSource returns unseen indexed source selected by ranges or find without new read
// authority. Receipts are caller-reported delivery state, not authorization.
func ReadSource(request ReadSourceRequest) (ReadSourceResult, error) {
	if err := validateSourceReadRequest(request); err != nil {
		return ReadSourceResult{}, err
	}
	if request.Root == "" {
		request.Root = "."
	}
	var result ReadSourceResult
	err := scan.WithExistingOutputRead(context.Background(), request.Root, func() error {
		var err error
		result, err = readSource(request)
		return err
	})
	if err != nil {
		return ReadSourceResult{}, err
	}
	return result, nil
}

func validateSourceReadRequest(request ReadSourceRequest) error {
	if len(request.Files) == 0 || len(request.Files) > 16 {
		return fmt.Errorf("source read requires 1 to 16 file entries")
	}
	intervals, lines, receipts := 0, 0, 0
	for requestIndex, file := range request.Files {
		if !safeSourceReadPath(file.Path) || (len(file.Ranges) == 0) == (file.Find == nil) {
			return fmt.Errorf("source read requires a safe relative path and exactly one of ranges or find")
		}
		if file.Find != nil {
			if err := validateSourceReadFind(*file.Find); err != nil {
				return fmt.Errorf("files[%d] path %q: %w", requestIndex, file.Path, err)
			}
		}
		intervals += len(file.Ranges)
		receipts += len(file.Seen)
		if intervals > 32 || receipts > 64 {
			return fmt.Errorf("source read exceeds 32 ranges or 64 receipts")
		}
		for _, r := range file.Ranges {
			if r[0] < 1 || r[1] < r[0] || r[1]-r[0] >= 500 || r[1] > MaxContextSourceFileBytes+1 {
				return fmt.Errorf("source range must contain 1 to 500 valid lines")
			}
			lines += r[1] - r[0] + 1
		}
		for _, receipt := range file.Seen {
			if _, _, err := parseSourceReadReceipt(receipt); err != nil {
				return err
			}
		}
	}
	if intervals > 32 || lines > 1000 || receipts > 64 {
		return fmt.Errorf("source read exceeds 32 ranges, 1000 lines or 64 receipts")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if len(body) > MaxSourceReadRequestBytes {
		return fmt.Errorf("source read request exceeds 64 KiB")
	}
	return nil
}

type sourceReadBatchFile struct {
	file        sourceFile
	resolved    string
	redacted    []string
	finds       []sourceReadBatchFind
	findResult  *SourceReadFindResult
	findResults []SourceReadBatchFindResult
	path        string
	ranges      []SourceReadRange
	seen        []string
	deferred    bool
}

type sourceReadBatchFind struct {
	requestIndex int
	request      SourceReadFindRequest
	limit        int
}

func readSource(request ReadSourceRequest) (ReadSourceResult, error) {
	loaded, err := loadContextIndex(ContextRequest{Root: request.Root})
	if err != nil {
		return ReadSourceResult{}, err
	}
	absoluteRoot, err := filepath.Abs(request.Root)
	if err != nil {
		return ReadSourceResult{}, err
	}
	root, err := pathutil.Resolve(absoluteRoot)
	if err != nil {
		return ReadSourceResult{}, err
	}
	var batch []*sourceReadBatchFile
	byPath := map[string]*sourceReadBatchFile{}
	for requestIndex, entry := range request.Files {
		resolved, err := resolveSourceReadPath(root, loaded, entry.Path)
		if err != nil {
			return ReadSourceResult{}, fmt.Errorf("read %q: %w", entry.Path, err)
		}
		item := byPath[resolved]
		if item == nil {
			relative, err := filepath.Rel(root, resolved)
			if err != nil {
				return ReadSourceResult{}, err
			}
			item = &sourceReadBatchFile{resolved: resolved, path: filepath.ToSlash(relative)}
			byPath[resolved] = item
			batch = append(batch, item)
		} else if (len(item.finds) > 0) != (entry.Find != nil) {
			return ReadSourceResult{}, fmt.Errorf("files[%d] path %q resolves to %q: cannot combine ranges and find for one canonical file; use separate requests", requestIndex, entry.Path, item.path)
		}
		if entry.Find != nil {
			limit := entry.Find.MaxMatches
			if limit == 0 {
				limit = 10
			}
			item.finds = append(item.finds, sourceReadBatchFind{requestIndex: requestIndex, request: *entry.Find, limit: limit})
		}
		item.ranges = append(item.ranges, entry.Ranges...)
		item.seen = append(item.seen, entry.Seen...)
	}
	for _, item := range batch {
		item.file, err = readSourceFile(item.resolved)
		if err != nil {
			return ReadSourceResult{}, err
		}
		item.redacted, err = redactSourceReadFile(item.file)
		if err != nil {
			return ReadSourceResult{}, err
		}
	}
	for {
		result, intervals, lines, err := deliverSourceReadBatch(batch)
		if err != nil {
			return ReadSourceResult{}, err
		}
		body, err := json.Marshal(result)
		if err != nil {
			return ReadSourceResult{}, err
		}
		if intervals <= 32 && lines <= 1000 && len(body)+1 <= MaxSourceReadResultBytes {
			return result, nil
		}
		if reduceSourceReadFindPage(batch) {
			continue
		}
		if deferSourceReadBatchFile(batch, result) {
			continue
		}
		if intervals > 32 || lines > 1000 {
			return ReadSourceResult{}, fmt.Errorf("source read exceeds 32 ranges or 1000 lines; reduce find before/after or split the request")
		}
		return ReadSourceResult{}, fmt.Errorf("source read result exceeds 24 KiB and cannot be paged further; reduce ranges or find before/after, or split the request")
	}
}

func deliverSourceReadBatch(batch []*sourceReadBatchFile) (ReadSourceResult, int, int, error) {
	result := ReadSourceResult{Files: make([]SourceReadFileResult, 0, len(batch))}
	intervals, lines := 0, 0
	for _, item := range batch {
		if item.deferred {
			result.Files = append(result.Files, SourceReadFileResult{Path: item.path, Sections: []SourceReadSection{}, SkippedRanges: []SourceReadRange{}, OutputLimited: true})
			continue
		}
		selected := *item
		selected.ranges = append([]SourceReadRange(nil), item.ranges...)
		selected.findResult = nil
		selected.findResults = nil
		for _, find := range item.finds {
			request := find.request
			request.MaxMatches = find.limit
			ranges, findResult := findSourceReadRanges(item.redacted, request)
			requestedLimit := find.request.MaxMatches
			if requestedLimit == 0 {
				requestedLimit = 10
			}
			findResult.OutputLimited = find.limit < requestedLimit && findResult.MatchCount > len(findResult.MatchLines)
			selected.ranges = append(selected.ranges, ranges...)
			if len(item.finds) == 1 {
				selected.findResult = findResult
			} else {
				selected.findResults = append(selected.findResults, SourceReadBatchFindResult{RequestIndex: find.requestIndex, Result: *findResult})
			}
		}
		if len(item.finds) > 1 {
			selected.ranges = mergeSourceReadRanges(selected.ranges)
		}
		file, err := deliverSourceReadFile(selected)
		if err != nil {
			return ReadSourceResult{}, 0, 0, err
		}
		intervals += len(file.Sections)
		for _, section := range file.Sections {
			lines += section.EndLine - section.StartLine + 1
		}
		result.Files = append(result.Files, file)
	}
	return result, intervals, lines, nil
}

func reduceSourceReadFindPage(batch []*sourceReadBatchFile) bool {
	for itemIndex := len(batch) - 1; itemIndex >= 0; itemIndex-- {
		if batch[itemIndex].deferred {
			continue
		}
		for findIndex := len(batch[itemIndex].finds) - 1; findIndex >= 0; findIndex-- {
			if batch[itemIndex].finds[findIndex].limit > 1 {
				batch[itemIndex].finds[findIndex].limit--
				return true
			}
		}
	}
	return false
}

func deferSourceReadBatchFile(batch []*sourceReadBatchFile, result ReadSourceResult) bool {
	delivered := 0
	for _, file := range result.Files {
		if len(file.Sections) > 0 {
			delivered++
		}
	}
	if delivered < 2 {
		return false
	}
	for index := len(batch) - 1; index >= 0; index-- {
		if !batch[index].deferred && len(result.Files[index].Sections) > 0 {
			batch[index].deferred = true
			return true
		}
	}
	return false
}

func deliverSourceReadFile(item sourceReadBatchFile) (SourceReadFileResult, error) {
	result := SourceReadFileResult{Path: item.path, Sections: []SourceReadSection{}, SkippedRanges: []SourceReadRange{}, Find: item.findResult, FindResults: item.findResults}
	fingerprint := sourceReadFingerprint(item.file)
	var seen []SourceReadRange
	for _, receipt := range item.seen {
		received, ranges, err := parseSourceReadReceipt(receipt)
		if err != nil {
			return SourceReadFileResult{}, err
		}
		if received != fingerprint {
			result.IgnoredReceipts++
			continue
		}
		for _, r := range ranges {
			if r[1] > len(item.file.Lines) {
				return SourceReadFileResult{}, fmt.Errorf("receipt range exceeds current EOF")
			}
		}
		seen = append(seen, ranges...)
	}
	seen = mergeSourceReadRanges(seen)
	var requested []SourceReadRange
	for _, r := range mergeSourceReadRanges(item.ranges) {
		if r[1] > len(item.file.Lines) {
			result.EOFRanges = append(result.EOFRanges, SourceReadRange{max(r[0], len(item.file.Lines)+1), r[1]})
			r[1] = len(item.file.Lines)
		}
		if r[0] <= r[1] {
			requested = append(requested, r)
		}
	}
	missing, skipped := subtractSourceReadRanges(requested, seen)
	result.SkippedRanges = skipped
	for _, r := range missing {
		result.Sections = append(result.Sections, SourceReadSection{StartLine: r[0], EndLine: r[1], Content: renderNumberedSource(item.redacted, r[0], r[1])})
	}
	delivered := mergeSourceReadRanges(append(seen, requested...))
	if len(delivered) > maxSourceReadReceiptRanges {
		return SourceReadFileResult{}, fmt.Errorf("cumulative receipt exceeds 64 intervals")
	}
	if len(delivered) > 0 {
		result.Receipt = makeSourceReadReceipt(fingerprint, delivered)
	}
	return result, nil
}

func redactSourceReadFile(file sourceFile) ([]string, error) {
	// Give the redactor explicit line prefixes so numeric property keys cannot
	// be mistaken for renderer metadata; retain preceding multiline context.
	numbered := renderNumberedSource(file.Lines, 1, len(file.Lines))
	redacted := strings.Split(redactContextConfigurationValues(file.Path, numbered), "\n")
	if len(redacted) != len(file.Lines) {
		return nil, fmt.Errorf("source redaction changed line boundaries")
	}
	for i, line := range redacted {
		content, ok := strings.CutPrefix(line, strconv.Itoa(i+1)+"\t")
		if !ok {
			return nil, fmt.Errorf("source redaction changed line boundaries")
		}
		redacted[i] = content
	}
	return redacted, nil
}

func safeSourceReadPath(path string) bool {
	if strings.TrimSpace(path) == "" || isPortableAbsolutePath(path) || strings.ContainsAny(path, "\\\x00") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func sourceReadGeneratedPath(path string) bool {
	if !contextSourceFileAllowed(path) {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(strings.ToLower(part), ".goregraph-lock-") {
			return true
		}
		switch strings.ToLower(part) {
		case ".git", ".goregraph-workspace", "goregraph-out":
			return true
		}
	}
	return false
}

func resolveSourceReadPath(root string, loaded loadedContextIndex, path string) (string, error) {
	if sourceReadGeneratedPath(path) {
		return "", fmt.Errorf("generated and Git paths are not source")
	}
	resolved, err := resolveSourcePath(loadedContextIndex{ScopeRoot: root}, sourceCandidate{Path: path})
	if err != nil {
		return "", err
	}
	scope, err := pathutil.Resolve(loaded.ScopeRoot)
	if err != nil {
		return "", err
	}
	if !pathIsWithin(scope, resolved) || sourceReadGeneratedPath(resolved) {
		return "", fmt.Errorf("source path is unsafe")
	}
	outputRoot := filepath.Dir(filepath.Dir(loaded.Path))
	if canonical, err := pathutil.Resolve(outputRoot); err == nil && pathIsWithin(canonical, resolved) {
		return "", fmt.Errorf("generated output is not source")
	}
	candidates := make([]sourceCandidate, 0, len(loaded.Index.SourceHashes)+len(loaded.Index.Facts))
	for path := range loaded.Index.SourceHashes {
		candidate := sourceCandidate{Path: path}
		if loaded.Workspace {
			// A hash key is workspace-relative; '.' retains workspace confinement.
			candidate.Project = "."
		}
		candidates = append(candidates, candidate)
	}
	for _, fact := range loaded.Index.Facts {
		if fact.File != "" && fact.Line > 0 && eligibleContextPathFact(fact, true) {
			candidates = append(candidates, sourceCandidate{Project: fact.Project, Path: fact.File})
		}
	}
	// Check likely identities first; ordinary exact reads need no unrelated stats.
	for pass := 0; pass < 2; pass++ {
		for _, candidate := range candidates {
			if !safeSourceReadPath(candidate.Path) || sourceReadGeneratedPath(candidate.Path) {
				continue
			}
			project := ""
			if loaded.Workspace {
				project = candidate.Project
			}
			lexical := filepath.Join(scope, project, candidate.Path)
			direct := lexical == resolved || lexical == filepath.Join(root, path)
			if direct != (pass == 0) {
				continue
			}
			candidatePath, err := resolveSourcePath(loaded, candidate)
			if err != nil || candidatePath != resolved {
				continue
			}
			if err := rejectSourceReadOutput(scope, resolved); err != nil {
				return "", err
			}
			return resolved, nil
		}
	}
	return "", fmt.Errorf("path is not indexed source")
}

// Hash records lack a project field, so check ancestor configurations rather
// than treating a workspace hash as belonging to the workspace root project.
func rejectSourceReadOutput(scope, resolved string) error {
	for dir := filepath.Dir(resolved); pathIsWithin(scope, dir); dir = filepath.Dir(dir) {
		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}
		output, err := pathutil.Resolve(filepath.Join(dir, cfg.OutputDir))
		if err == nil && pathIsWithin(output, resolved) {
			return fmt.Errorf("generated output is not source")
		}
		if dir == scope {
			break
		}
	}
	return nil
}

func sourceReadFingerprint(file sourceFile) string {
	sum := sha256.Sum256([]byte("goregraph-source-read-r1\x00" + filepath.Clean(file.Path) + "\x00" + file.Hash))
	return hex.EncodeToString(sum[:])
}

func makeSourceReadReceipt(fingerprint string, ranges []SourceReadRange) string {
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = strconv.Itoa(r[0]) + "-" + strconv.Itoa(r[1])
	}
	return "r1:" + fingerprint + ":" + strings.Join(parts, ",")
}

func parseSourceReadReceipt(receipt string) (string, []SourceReadRange, error) {
	invalid := fmt.Errorf("malformed or excessive source read receipt")
	if len(receipt) > maxSourceReadReceiptBytes {
		return "", nil, invalid
	}
	parts := strings.Split(receipt, ":")
	if len(parts) != 3 || parts[0] != "r1" || len(parts[1]) != 64 || strings.ToLower(parts[1]) != parts[1] {
		return "", nil, invalid
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", nil, invalid
	}
	intervals := strings.Split(parts[2], ",")
	if len(intervals) > maxSourceReadReceiptRanges {
		return "", nil, invalid
	}
	ranges := make([]SourceReadRange, 0, len(intervals))
	for _, interval := range intervals {
		endpoints := strings.Split(interval, "-")
		if len(endpoints) != 2 {
			return "", nil, invalid
		}
		start, err1 := strconv.Atoi(endpoints[0])
		end, err2 := strconv.Atoi(endpoints[1])
		if err1 != nil || err2 != nil || start < 1 || end < start || end > MaxContextSourceFileBytes+1 || strconv.Itoa(start) != endpoints[0] || strconv.Itoa(end) != endpoints[1] {
			return "", nil, invalid
		}
		ranges = append(ranges, SourceReadRange{start, end})
	}
	return parts[1], mergeSourceReadRanges(ranges), nil
}

func mergeSourceReadRanges(ranges []SourceReadRange) []SourceReadRange {
	result := append([]SourceReadRange{}, ranges...)
	sort.Slice(result, func(i, j int) bool {
		if result[i][0] != result[j][0] {
			return result[i][0] < result[j][0]
		}
		return result[i][1] < result[j][1]
	})
	merged := result[:0]
	for _, r := range result {
		if len(merged) > 0 && r[0] <= merged[len(merged)-1][1]+1 {
			merged[len(merged)-1][1] = max(merged[len(merged)-1][1], r[1])
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func subtractSourceReadRanges(requested, seen []SourceReadRange) ([]SourceReadRange, []SourceReadRange) {
	missing, skipped := []SourceReadRange{}, []SourceReadRange{}
	for _, r := range requested {
		cursor := r[0]
		for _, previous := range seen {
			start, end := max(cursor, previous[0]), min(r[1], previous[1])
			if start > end {
				continue
			}
			if cursor < start {
				missing = append(missing, SourceReadRange{cursor, start - 1})
			}
			skipped = append(skipped, SourceReadRange{start, end})
			cursor = end + 1
		}
		if cursor <= r[1] {
			missing = append(missing, SourceReadRange{cursor, r[1]})
		}
	}
	return missing, skipped
}

func attachSourceReadReceipt(section *ContextSourceSection, file sourceFile) {
	if file.Path == "" || file.Hash == "" || section.StartLine < 1 || section.EndLine < section.StartLine || section.EndLine > len(file.Lines) {
		return
	}
	section.ReadReceipt = makeSourceReadReceipt(sourceReadFingerprint(file), []SourceReadRange{{section.StartLine, section.EndLine}})
}
