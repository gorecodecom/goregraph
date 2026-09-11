// Package answercheck checks explicit Markdown file citations against a supplied
// delivery ledger. It never reads source files or certifies factual claims.
package answercheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits bound caller-supplied data and work without reading the filesystem.
const (
	MaxRequestBytes = 1 << 20
	MaxAnswerBytes  = 512 << 10
	MaxFiles        = 1024
	MaxRanges       = 4096
	MaxReferences   = 4096
	MaxPathBytes    = 4096
	MaxLine         = 2147483647
)

// Range is an inclusive pair of numbered source-line endpoints.
type Range [2]int

// File records one discovered identity and source ranges actually delivered.
// RedactedRanges cover visible keys or structure, not concealed values.
type File struct {
	Path           string  `json:"path"`
	Ranges         []Range `json:"ranges,omitempty"`
	RedactedRanges []Range `json:"redacted_ranges,omitempty"`
}

// Request supplies the answer, optional absolute workspace root and delivery ledger.
type Request struct {
	Answer      string `json:"answer"`
	Root        string `json:"root,omitempty"`
	Files       []File `json:"files"`
	RepairPaths bool   `json:"repair_paths,omitempty"`
}

// Finding describes a mechanical citation issue at a one-based answer line.
type Finding struct {
	AnswerLine int     `json:"answer_line"`
	Code       string  `json:"code"`
	Path       string  `json:"path,omitempty"`
	Ranges     []Range `json:"ranges,omitempty"`
	Message    string  `json:"message"`
}

// Repair records one exact path-token substitution.
type Repair struct {
	AnswerLine int    `json:"answer_line"`
	From       string `json:"from"`
	To         string `json:"to"`
}

// Reference records the normalized identity, requested ranges and evidence coverage.
type Reference struct {
	AnswerLine int     `json:"answer_line"`
	Path       string  `json:"path"`
	Ranges     []Range `json:"ranges,omitempty"`
	Coverage   string  `json:"coverage"`
}

// Result reports syntax and ledger coverage, never semantic certification.
type Result struct {
	Valid             bool        `json:"valid"`
	Answer            string      `json:"answer"`
	Findings          []Finding   `json:"findings"`
	Repairs           []Repair    `json:"repairs"`
	CheckedReferences []Reference `json:"checked_references"`
	CheckedRanges     int         `json:"checked_ranges"`
	SemanticValidity  string      `json:"semantic_validity"`
	Limitations       []string    `json:"limitations"`
}

// UnmarshalJSON requires exactly two positive, ordered JSON integer endpoints.
func (r *Range) UnmarshalJSON(data []byte) error {
	var endpoints []int
	if err := json.Unmarshal(data, &endpoints); err != nil {
		return err
	}
	if len(endpoints) != 2 {
		return fmt.Errorf("range requires exactly two endpoints")
	}
	*r = Range{endpoints[0], endpoints[1]}
	if !validRange(*r) {
		return fmt.Errorf("range endpoints must satisfy 1 <= start <= end <= %d", MaxLine)
	}
	return nil
}

// Decode strictly decodes one bounded JSON request and validates its ledger.
func Decode(reader io.Reader) (Request, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if err != nil {
		return Request{}, err
	}
	if len(data) > MaxRequestBytes {
		return Request{}, fmt.Errorf("request exceeds %d bytes", MaxRequestBytes)
	}
	if !utf8.Valid(data) {
		return Request{}, fmt.Errorf("request must be UTF-8")
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return Request{}, fmt.Errorf("request must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Request{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Request{}, fmt.Errorf("request must contain exactly one JSON object")
	}
	if err := validate(request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func validRange(r Range) bool { return r[0] > 0 && r[1] >= r[0] && r[1] <= MaxLine }

func validate(request Request) error {
	if len(request.Answer) > MaxAnswerBytes || !utf8.ValidString(request.Answer) {
		return fmt.Errorf("answer must be UTF-8 and at most %d bytes", MaxAnswerBytes)
	}
	if len(request.Files) > MaxFiles {
		return fmt.Errorf("ledger exceeds %d files", MaxFiles)
	}
	if request.Root != "" && (!strings.HasPrefix(request.Root, "/") || unsafePath(request.Root) || path.Clean(request.Root) != request.Root) {
		return fmt.Errorf("root must be a clean absolute POSIX workspace path")
	}
	seen := make(map[string]bool, len(request.Files))
	count := 0
	for _, file := range request.Files {
		if file.Path == "" || strings.HasPrefix(file.Path, "/") || unsafePath(file.Path) || abbreviated(file.Path) || path.Clean(file.Path) != file.Path {
			return fmt.Errorf("ledger path must be an exact clean root-relative file path: %q", file.Path)
		}
		if seen[file.Path] {
			return fmt.Errorf("duplicate ledger path %q", file.Path)
		}
		seen[file.Path] = true
		count += len(file.Ranges) + len(file.RedactedRanges)
		if count > MaxRanges {
			return fmt.Errorf("ledger exceeds %d ranges", MaxRanges)
		}
		for _, ranges := range [][]Range{file.Ranges, file.RedactedRanges} {
			for _, r := range ranges {
				if !validRange(r) {
					return fmt.Errorf("invalid ledger range for %q", file.Path)
				}
			}
		}
	}
	return nil
}

var (
	fileExtension           = regexp.MustCompile(`(?i)\.(?:java|go|ts|tsx|js|jsx|mjs|cjs|json|ya?ml|xml|properties|toml|md|txt|sql|kt|kts|swift|py|rb|rs|c|h|cc|cpp|hpp|cs|sh|bash|zsh|vue|svelte|css|scss|html|gradle|proto|graphql|lock|mod|sum)(?:[:#]|$)`)
	markdownLink            = regexp.MustCompile(`\[([^\]\n]*)\]\((<[^>\n]*>|[^)\n]*)\)`)
	inlineCode              = regexp.MustCompile("`+[^`\\n]+`+")
	adjacentMarker          = regexp.MustCompile(`(?i)^[\s,;:(]*((?:Z\.|Zeilen?\b|lines?\b)\s*)`)
	nextLinkSeparator       = regexp.MustCompile(`(?i)^,\s*(?:(?:and|und)\s+)?`)
	rangePrefix             = regexp.MustCompile(`^[0-9]+(?:\s*[-–—]\s*L?[0-9]+)?(?:\s*(?:,|;|/|\bund\b|\band\b)\s*L?[0-9]+(?:\s*[-–—]\s*L?[0-9]+)?)*`)
	rangePart               = regexp.MustCompile(`^([0-9]+)(?:\s*[-–—]\s*L?([0-9]+))?$`)
	rangeSeparator          = regexp.MustCompile(`\s*(?:,|;|/|\bund\b|\band\b)\s*`)
	tableSeparator          = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)+\|?\s*$`)
	referenceLink           = regexp.MustCompile(`\[([^\]\n]+)\]\[[^\]\n]*\]`)
	tableNumbers            = regexp.MustCompile(`[0-9]+`)
	httpStatus              = regexp.MustCompile(`(?i)\bHTTP\s+[0-9]+(?:/[0-9]+)*`)
	lineMarker              = regexp.MustCompile(`(?i)(?:\bZ\.|\bZeilen?\b|\blines?\b)\s*`)
	unsupportedContinuation = regexp.MustCompile(`(?i)^(?:\.\.|\.\s*[0-9]|…|(?:to|through|bis)\b|;\s*(?:[0-9+;,.–—-]|Z\.|lines?\b|Zeilen?\b))`)
)

type occurrence struct {
	start  int
	token  string
	ranges []Range
	code   string
}
type replacement struct {
	start, end int
	value      string
}

// Check validates only explicit file identities and source ranges against the
// caller's ledger. Valid does not authenticate that ledger or certify semantics.
func Check(request Request) (Result, error) {
	if err := validate(request); err != nil {
		return Result{}, err
	}
	result := Result{Answer: request.Answer, Findings: []Finding{}, Repairs: []Repair{}, CheckedReferences: []Reference{}, SemanticValidity: "not_verified", Limitations: []string{
		"Caller-provided discovery and delivery ledger is not authenticated; receipts, match metadata and EOF metadata are not delivered source.",
		"Checks explicit inline-code file tokens, inline Markdown link destinations, adjacent Z./Zeilen/lines citations and recognized table citation columns; not exhaustive Markdown or freeform claim parsing.",
		"Fenced and indented code, external HTTP(S) links and receipt strings are excluded; metadata-only references establish identity only.",
		"Redacted ranges prove visible keys or structure only, never hidden values. Semantic correctness requires independent review.",
	}}
	ledger := make(map[string]File, len(request.Files))
	for _, file := range request.Files {
		file.Ranges = merge(file.Ranges)
		file.RedactedRanges = merge(file.RedactedRanges)
		ledger[file.Path] = file
	}
	lines := strings.SplitAfter(request.Answer, "\n")
	offset := 0
	fence := byte(0)
	fenceLength := 0
	var edits []replacement
	var citationColumns []int
	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)
		fenceText := strings.TrimLeft(line, " ")
		if len(line)-len(fenceText) <= 3 && len(fenceText) >= 3 && (fenceText[0] == '`' || fenceText[0] == '~') {
			n := 0
			for n < len(fenceText) && fenceText[n] == fenceText[0] {
				n++
			}
			if n >= 3 {
				if fence == 0 {
					fence = fenceText[0]
					fenceLength = n
				} else if fence == fenceText[0] && n >= fenceLength && strings.TrimSpace(fenceText[n:]) == "" {
					fence = 0
				}
				offset += len(line)
				continue
			}
		}
		if fence != 0 || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			offset += len(line)
			continue
		}
		if i+1 < len(lines) && tableSeparator.MatchString(strings.TrimSpace(lines[i+1])) {
			citationColumns = nil
			for col, cell := range tableCells(line) {
				lower := strings.ToLower(cell)
				if strings.Contains(lower, "zeilen") || strings.Contains(lower, "lines") || strings.Contains(lower, "citation") || strings.TrimSpace(lower) == "z." {
					citationColumns = append(citationColumns, col)
				}
			}
			offset += len(line)
			continue
		}
		if tableSeparator.MatchString(trimmed) {
			offset += len(line)
			continue
		}
		if !strings.Contains(line, "|") {
			citationColumns = nil
		}
		occurrences := extract(line, ledger)
		if unsupportedLine(line, ledger) {
			result.Findings = append(result.Findings, Finding{AnswerLine: lineNumber, Code: "unsupported_citation", Message: "unsupported or multiline Markdown reference syntax requires manual review"})
		}
		if len(result.CheckedReferences)+len(occurrences) > MaxReferences {
			return Result{}, fmt.Errorf("answer exceeds %d explicit references", MaxReferences)
		}
		if len(citationColumns) > 0 && len(occurrences) > 0 {
			var ranges []Range
			syntaxBad := false
			cells := tableCells(line)
			for _, col := range citationColumns {
				if col >= len(cells) {
					syntaxBad = true
					continue
				}
				r, bad := tableRanges(cells[col])
				ranges = append(ranges, r...)
				syntaxBad = syntaxBad || bad
			}
			if len(ranges) > 0 || syntaxBad {
				if len(occurrences) != 1 {
					result.Findings = append(result.Findings, Finding{AnswerLine: lineNumber, Code: "ambiguous_citation", Message: "table source ranges cannot be bound to exactly one file"})
				} else {
					occurrences[0].ranges = append(occurrences[0].ranges, ranges...)
					if syntaxBad {
						occurrences[0].code = "unsupported_citation"
					}
				}
			}
		}
		for _, o := range occurrences {
			rawPath, ownRanges, code := splitCitation(o.token)
			ranges := append(ownRanges, o.ranges...)
			if o.code != "" {
				code = o.code
			}
			ref := Reference{AnswerLine: lineNumber, Path: rawPath, Ranges: ranges, Coverage: "unverified"}
			resolved, identityCode := resolve(rawPath, request.Root, ledger, ranges)
			if identityCode != "" {
				code = identityCode
			}
			if identityCode == "" {
				ref.Path = resolved
				if code == "" {
					file := ledger[resolved]
					ref.Coverage = "identity_only"
					if len(ranges) > 0 {
						ref.Coverage = "delivered"
						all := merge(append(append([]Range{}, file.Ranges...), file.RedactedRanges...))
						for _, r := range ranges {
							if !covered(r, all) {
								code = "undelivered_range"
								ref.Coverage = "undelivered"
								break
							}
							if !covered(r, file.Ranges) {
								ref.Coverage = "includes_redacted"
							}
						}
					}
				}
				normalizedPath := rawPath
				if strings.HasPrefix(rawPath, "/") {
					normalizedPath = strings.TrimPrefix(rawPath, strings.TrimSuffix(request.Root, "/")+"/")
				}
				if normalizedPath != resolved {
					if request.RepairPaths && code == "" {
						edits = append(edits, replacement{offset + o.start, offset + o.start + len(rawPath), resolved})
						result.Repairs = append(result.Repairs, Repair{AnswerLine: lineNumber, From: rawPath, To: resolved})
					} else if code == "" {
						code = "incomplete_path"
					}
				}
			}
			result.CheckedReferences = append(result.CheckedReferences, ref)
			result.CheckedRanges += len(ranges)
			if result.CheckedRanges > MaxRanges {
				return Result{}, fmt.Errorf("answer exceeds %d cited ranges", MaxRanges)
			}
			if code != "" {
				result.Findings = append(result.Findings, Finding{AnswerLine: lineNumber, Code: code, Path: rawPath, Ranges: ranges, Message: findingMessage(code)})
			}
		}
		offset += len(line)
	}
	if len(result.CheckedReferences) == 0 {
		result.Findings = append(result.Findings, Finding{AnswerLine: 1, Code: "no_references", Message: "no supported explicit file references were checked"})
	}
	if len(edits) > 0 {
		var repaired strings.Builder
		previous := 0
		for _, edit := range edits {
			repaired.WriteString(request.Answer[previous:edit.start])
			repaired.WriteString(edit.value)
			previous = edit.end
		}
		repaired.WriteString(request.Answer[previous:])
		result.Answer = repaired.String()
	}
	result.Valid = len(result.Findings) == 0
	return result, nil
}

func extract(line string, ledger map[string]File) []occurrence {
	var occurrences []occurrence
	occupied := make([]bool, len(line))
	for _, m := range markdownLink.FindAllStringSubmatchIndex(line, -1) {
		for i := m[0]; i < m[1]; i++ {
			occupied[i] = true
		}
		start, end := m[4], m[5]
		if line[start] == '<' {
			start++
			end--
		}
		token := line[start:end]
		lowerToken := strings.ToLower(token)
		if strings.HasPrefix(lowerToken, "https://") || strings.HasPrefix(lowerToken, "http://") || strings.HasPrefix(lowerToken, "mailto:") || strings.HasPrefix(token, "#") {
			continue
		}
		o := occurrence{start: start, token: token}
		o.ranges, o.code = adjacentRanges(line[m[1]:])
		occurrences = append(occurrences, o)
	}
	for _, m := range inlineCode.FindAllStringIndex(line, -1) {
		if occupied[m[0]] {
			continue
		}
		start, end := m[0], m[1]
		for start < end && line[start] == '`' {
			start++
		}
		for end > start && line[end-1] == '`' {
			end--
		}
		token := line[start:end]
		if !candidate(token, ledger) {
			continue
		}
		o := occurrence{start: start, token: token}
		o.ranges, o.code = adjacentRanges(line[m[1]:])
		occurrences = append(occurrences, o)
	}
	sort.Slice(occurrences, func(i, j int) bool { return occurrences[i].start < occurrences[j].start })
	return occurrences
}

func candidate(token string, ledger map[string]File) bool {
	lower := strings.ToLower(token)
	if strings.HasPrefix(lower, "read_receipt:") || strings.HasPrefix(lower, "source_receipt:") || strings.HasPrefix(lower, "receipt:") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return false
	}
	p, _, _ := splitCitation(token)
	if _, ok := ledger[p]; ok {
		return true
	}
	for name := range ledger {
		if path.Base(p) == path.Base(name) {
			return true
		}
	}
	return fileExtension.MatchString(token)
}

func splitCitation(token string) (string, []Range, string) {
	index := strings.IndexAny(token, ":#")
	if index < 0 {
		return token, nil, ""
	}
	p, suffix := token[:index], token[index:]
	if strings.Contains(token, "://") {
		return token, nil, "unsafe_path"
	}
	if strings.HasPrefix(suffix, "#L") {
		suffix = suffix[2:]
	} else if strings.HasPrefix(suffix, ":") {
		suffix = suffix[1:]
	} else {
		return p, nil, "unsupported_citation"
	}
	ranges, err := parseRanges(suffix)
	if err != nil {
		return p, nil, "invalid_range"
	}
	return p, ranges, ""
}

func adjacentRanges(tail string) ([]Range, string) {
	m := adjacentMarker.FindStringIndex(tail)
	if m == nil {
		return nil, ""
	}
	ranges, bad := prefixRanges(tail[m[1]:])
	if bad {
		return ranges, "unsupported_citation"
	}
	return ranges, ""
}

func prefixRanges(value string) ([]Range, bool) {
	m := rangePrefix.FindStringIndex(value)
	if m == nil {
		return nil, true
	}
	ranges, err := parseRanges(value[:m[1]])
	if err != nil {
		return nil, true
	}
	rawRest := value[m[1]:]
	if rawRest != "" {
		r, _ := utf8.DecodeRuneInString(rawRest)
		if unicode.IsLetter(r) {
			return ranges, true
		}
	}
	rest := strings.TrimSpace(rawRest)
	if unsupportedContinuation.MatchString(rest) {
		return ranges, true
	}
	if separator := nextLinkSeparator.FindStringIndex(rest); separator != nil {
		if link := markdownLink.FindStringIndex(rest[separator[1]:]); link != nil && link[0] == 0 {
			return ranges, false
		}
	}
	if strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "–") || strings.HasPrefix(rest, "—") || strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, ",") {
		return ranges, true
	}
	if rest != "" {
		r, _ := utf8.DecodeRuneInString(rest)
		if unicode.IsDigit(r) {
			return ranges, true
		}
	}
	return ranges, false
}

func tableRanges(cell string) ([]Range, bool) {
	// Explicit markers bind their following ranges; earlier numbers can be labels
	// such as HTTP status codes. Otherwise a declared citation column supplies the
	// binding. Semicolons separate independent symbol/citation groups.
	cell = inlineCode.ReplaceAllStringFunc(cell, func(code string) string {
		token := strings.TrimSpace(strings.Trim(code, "`"))
		if token != "" && (token[0] >= '0' && token[0] <= '9' || lineMarker.MatchString(token) && tableNumbers.MatchString(token)) {
			return token
		}
		return " "
	})
	cell = httpStatus.ReplaceAllString(cell, " ")
	var ranges []Range
	for _, segment := range strings.Split(cell, ";") {
		markers := lineMarker.FindAllStringIndex(segment, -1)
		if len(markers) > 0 {
			for _, marker := range markers {
				tail := strings.TrimSpace(segment[marker[1]:])
				if tableNumbers.FindStringIndex(tail) == nil {
					continue
				}
				parsed, bad := prefixRanges(tail)
				ranges = append(ranges, parsed...)
				if bad {
					return ranges, true
				}
			}
			continue
		}
		parsed, bad := unmarkedTableRanges(segment)
		ranges = append(ranges, parsed...)
		if bad {
			return ranges, true
		}
	}
	return ranges, false
}

func unmarkedTableRanges(cell string) ([]Range, bool) {
	var ranges []Range
	for cursor := 0; cursor < len(cell); {
		next := tableNumbers.FindStringIndex(cell[cursor:])
		if next == nil {
			break
		}
		start := cursor + next[0]
		if start > 0 {
			r, _ := utf8.DecodeLastRuneInString(cell[:start])
			if unicode.IsLetter(r) || r == '_' {
				cursor += next[1]
				continue
			}
		}
		match := rangePrefix.FindStringIndex(cell[start:])
		if match == nil {
			return ranges, true
		}
		parsed, bad := prefixRanges(cell[start:])
		ranges = append(ranges, parsed...)
		if bad {
			return ranges, true
		}
		cursor = start + match[1]
	}
	return ranges, false
}

func unsupportedLine(line string, ledger map[string]File) bool {
	// A local citation split over lines cannot be normalized safely. Complete
	// supported links and code spans are removed before looking for delimiters.
	remaining := markdownLink.ReplaceAllString(line, "")
	remaining = inlineCode.ReplaceAllString(remaining, "")
	if strings.Contains(remaining, "`") || strings.Contains(remaining, "](") {
		return true
	}
	for _, match := range referenceLink.FindAllStringSubmatch(line, -1) {
		if candidate(strings.Trim(match[1], "`"), ledger) {
			return true
		}
	}
	return false
}

func parseRanges(value string) ([]Range, error) {
	parts := rangeSeparator.Split(strings.TrimSpace(value), -1)
	if len(parts) > MaxRanges {
		return nil, fmt.Errorf("too many ranges")
	}
	var ranges []Range
	for _, part := range parts {
		m := rangePart.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("malformed range")
		}
		start, err := strconv.ParseInt(m[1], 10, 32)
		if err != nil {
			return nil, err
		}
		end := start
		if m[2] != "" {
			end, err = strconv.ParseInt(m[2], 10, 32)
			if err != nil {
				return nil, err
			}
		}
		r := Range{int(start), int(end)}
		if !validRange(r) {
			return nil, fmt.Errorf("invalid range")
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func tableCells(line string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
}

func unsafePath(p string) bool {
	if len(p) > MaxPathBytes || strings.TrimSpace(p) != p || strings.ContainsAny(p, "\\%?#<>") || strings.Contains(p, ":") || strings.Contains(p, "//") {
		return true
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." {
			return true
		}
	}
	return false
}

func abbreviated(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == "..." || part == "…" {
			return true
		}
	}
	return false
}

func resolve(p, root string, ledger map[string]File, citedRanges []Range) (string, string) {
	if p == "" || unsafePath(p) {
		return "", "unsafe_path"
	}
	if strings.HasPrefix(p, "/") {
		prefix := root + "/"
		if root == "/" {
			prefix = "/"
		}
		if root == "" || !strings.HasPrefix(p, prefix) {
			return "", "unsafe_path"
		}
		p = strings.TrimPrefix(p, prefix)
	}
	if _, ok := ledger[p]; ok {
		return p, ""
	}
	if !abbreviated(p) && strings.Contains(p, "/") {
		return "", "unknown_path"
	}
	var matches []string
	for name := range ledger {
		if path.Base(p) == path.Base(name) && (!strings.Contains(p, "/") || matchAbbreviation(strings.Split(p, "/"), strings.Split(name, "/"))) {
			matches = append(matches, name)
		}
	}
	if len(matches) > 1 && len(citedRanges) > 0 {
		matchingCoverage := make([]string, 0, len(matches))
		for _, name := range matches {
			file := ledger[name]
			available := merge(append(append([]Range{}, file.Ranges...), file.RedactedRanges...))
			coversEveryRange := true
			for _, citedRange := range citedRanges {
				if !covered(citedRange, available) {
					coversEveryRange = false
					break
				}
			}
			if coversEveryRange {
				matchingCoverage = append(matchingCoverage, name)
			}
		}
		if len(matchingCoverage) == 1 {
			return matchingCoverage[0], ""
		}
	}
	if len(matches) > 1 {
		return "", "ambiguous_path"
	}
	if len(matches) == 0 {
		return "", "unknown_path"
	}
	return matches[0], ""
}

func matchAbbreviation(pattern, parts []string) bool {
	// A component wildcard can stand for one or more already-discovered components.
	dp := make([]bool, len(parts)+1)
	dp[0] = true
	for _, component := range pattern {
		next := make([]bool, len(parts)+1)
		if component == "..." || component == "…" {
			seen := false
			for j := 1; j <= len(parts); j++ {
				seen = seen || dp[j-1]
				next[j] = seen
			}
		} else {
			for j := 1; j <= len(parts); j++ {
				next[j] = dp[j-1] && component == parts[j-1]
			}
		}
		dp = next
	}
	return dp[len(parts)]
}

func merge(ranges []Range) []Range {
	out := append([]Range{}, ranges...)
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	merged := make([]Range, 0, len(out))
	for _, r := range out {
		if len(merged) > 0 && int64(r[0]) <= int64(merged[len(merged)-1][1])+1 {
			if r[1] > merged[len(merged)-1][1] {
				merged[len(merged)-1][1] = r[1]
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func covered(want Range, available []Range) bool {
	for _, r := range available {
		if r[0] > want[0] {
			return false
		}
		if r[0] <= want[0] && r[1] >= want[1] {
			return true
		}
	}
	return false
}

func findingMessage(code string) string {
	switch code {
	case "unknown_path":
		return "file identity is absent from the supplied discovery ledger"
	case "ambiguous_path":
		return "abbreviated file identity matches multiple discovered files; no repair applied"
	case "incomplete_path":
		return "use the exact root-relative discovered path or enable safe path repair"
	case "unsafe_path":
		return "path is escaping, malformed, contains unsafe characters or uses an unsupported URL scheme"
	case "invalid_range":
		return "citation range must have positive ordered bounded integer endpoints"
	case "undelivered_range":
		return "at least one cited range is not fully covered by delivered source or redacted-key ranges"
	default:
		return "citation syntax is unsupported or cannot be safely bound; manual review required"
	}
}
