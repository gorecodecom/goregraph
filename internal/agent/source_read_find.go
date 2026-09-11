package agent

import (
	"fmt"
	"regexp"
)

// SourceReadFindRequest selects matching redacted lines and bounded context.
// Zero MaxMatches and StartLine use defaults of ten and one respectively.
type SourceReadFindRequest struct {
	Pattern    string `json:"pattern"`
	Before     int    `json:"before,omitempty"`
	After      int    `json:"after,omitempty"`
	MaxMatches int    `json:"max_matches,omitempty"`
	StartLine  int    `json:"start_line,omitempty"`
}

// SourceReadFindResult describes navigation across all remaining matching lines,
// independently of which source lines were delivered or previously seen.
type SourceReadFindResult struct {
	MatchLines    []int `json:"match_lines"`
	MatchCount    int   `json:"match_count"`
	NextStartLine int   `json:"next_start_line,omitempty"`
	OutputLimited bool  `json:"output_limited,omitempty"`
}

// SourceReadBatchFindResult preserves one selector's independent navigation.
// RequestIndex is its zero-based position in the original request's files array.
// Multiple selectors for a canonical file use find_results instead of find.
type SourceReadBatchFindResult struct {
	RequestIndex int                  `json:"request_index"`
	Result       SourceReadFindResult `json:"result"`
}

func validateSourceReadFind(find SourceReadFindRequest) error {
	if len(find.Pattern) == 0 || len(find.Pattern) > 1024 {
		return fmt.Errorf("source find pattern requires 1 to 1024 bytes")
	}
	if find.Before < 0 || find.Before > 100 || find.After < 0 || find.After > 100 {
		return fmt.Errorf("source find before/after must be between 0 and 100")
	}
	if find.MaxMatches < 0 || find.MaxMatches > 32 {
		return fmt.Errorf("source find max_matches must be 0 (default 10) or 1 to 32")
	}
	if find.StartLine < 0 || find.StartLine > MaxContextSourceFileBytes+1 {
		return fmt.Errorf("source find start_line is outside valid source bounds")
	}
	if _, err := regexp.Compile(find.Pattern); err != nil {
		return fmt.Errorf("invalid source find pattern: %w; find.pattern is a Go regexp (RE2), not literal text; escape regexp metacharacters to match them literally and double backslashes in JSON (for example, use \"\\\\(\" to match a literal parenthesis)", err)
	}
	return nil
}

func findSourceReadRanges(lines []string, find SourceReadFindRequest) ([]SourceReadRange, *SourceReadFindResult) {
	pattern := regexp.MustCompile(find.Pattern) // Validated before any source reads.
	limit := find.MaxMatches
	if limit == 0 {
		limit = 10
	}
	start := max(1, find.StartLine)
	result := &SourceReadFindResult{MatchLines: []int{}}
	var ranges []SourceReadRange
	for i := start - 1; i < len(lines); i++ {
		if !pattern.MatchString(lines[i]) {
			continue
		}
		result.MatchCount++
		if len(result.MatchLines) >= limit {
			continue
		}
		line := i + 1
		result.MatchLines = append(result.MatchLines, line)
		ranges = append(ranges, SourceReadRange{max(1, line-find.Before), min(len(lines), line+find.After)})
	}
	if result.MatchCount > len(result.MatchLines) {
		result.NextStartLine = result.MatchLines[len(result.MatchLines)-1] + 1
	}
	return mergeSourceReadRanges(ranges), result
}
