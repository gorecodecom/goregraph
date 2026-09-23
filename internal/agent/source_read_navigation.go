package agent

import "fmt"

// sourceReadNextRequest preserves each selector's cursor and cumulative delivery
// state. It is optional navigation, never permission to read additional source.
func sourceReadNextRequest(batch []*sourceReadBatchFile, result ReadSourceResult) *ReadSourceRequest {
	next := &ReadSourceRequest{}
	for index, item := range batch {
		file := result.Files[index]
		seen := item.seen
		if file.Receipt != "" {
			seen = []string{file.Receipt}
		}
		if item.deferred && len(item.finds) == 0 {
			next.Files = append(next.Files, SourceReadFileRequest{Path: item.path, Ranges: item.ranges, Seen: seen})
		}
		for findIndex, selector := range item.finds {
			find := selector.request
			if !item.deferred {
				metadata := file.Find
				if len(item.finds) > 1 {
					metadata = &file.FindResults[findIndex].Result
				}
				if metadata == nil || metadata.NextStartLine == 0 {
					continue
				}
				find.StartLine = metadata.NextStartLine
				// Retain the page size that fitted this batch rather than repeating
				// an oversized request on every continuation.
				find.MaxMatches = selector.limit
			}
			next.Files = append(next.Files, SourceReadFileRequest{Path: item.path, Find: &find, Seen: seen})
			// Receipts apply to the canonical file, so carry them once per batch.
			seen = nil
		}
	}
	if len(next.Files) == 0 {
		return nil
	}
	return next
}

func sourceReadCitations(path string, sections []SourceReadSection) []string {
	var citations []string
	for _, section := range sections {
		location := fmt.Sprintf("%s:%d", path, section.StartLine)
		if section.EndLine != section.StartLine {
			location += fmt.Sprintf("-%d", section.EndLine)
		}
		citations = append(citations, location)
	}
	return citations
}
