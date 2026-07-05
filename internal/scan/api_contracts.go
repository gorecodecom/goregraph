package scan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	codeHelperAPIRE = regexp.MustCompile(`\b(Get|Post|Put|Patch|Delete)Helper\s*\(\s*["']([^"']+)["']`)
	codeFetchAPIRE  = regexp.MustCompile(`\bfetch\s*\(\s*["']([^"']+)["']`)
	codeMethodRE    = regexp.MustCompile(`\bmethod\s*:\s*["']([A-Za-z]+)["']`)
)

func extractAPIContracts(file FileRecord, lines []string) []APIContractRecord {
	if isLowSignalCodeFile(file.Path) {
		return nil
	}
	switch file.Language {
	case "javascript", "typescript":
	default:
		return nil
	}

	var records []APIContractRecord
	for i, line := range lines {
		if match := codeHelperAPIRE.FindStringSubmatch(line); len(match) == 3 {
			records = append(records, apiContract(file, strings.ToUpper(match[1]), match[2], line, i+1, "helper-call"))
			continue
		}
		if match := codeFetchAPIRE.FindStringSubmatch(line); len(match) == 2 {
			method := "GET"
			if methodMatch := codeMethodRE.FindStringSubmatch(line); len(methodMatch) == 2 {
				method = strings.ToUpper(methodMatch[1])
			}
			records = append(records, apiContract(file, method, match[1], line, i+1, "fetch-call"))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].File != records[j].File {
			return records[i].File < records[j].File
		}
		if records[i].Line != records[j].Line {
			return records[i].Line < records[j].Line
		}
		return records[i].Path < records[j].Path
	})
	return records
}

func apiContract(file FileRecord, method, path, caller string, line int, reason string) APIContractRecord {
	return APIContractRecord{
		Language:        file.Language,
		App:             codeFileApp(file.Path),
		Package:         codeFilePackage(file.Path),
		HTTPMethod:      method,
		Path:            normalizeCodeRoutePath(path),
		Caller:          strings.TrimSpace(caller),
		File:            file.Path,
		Line:            line,
		Confidence:      "EXTRACTED",
		ConfidenceScore: 0.9,
		Reason:          reason,
	}
}

func renderAPIContractsReport(records []APIContractRecord) string {
	var b strings.Builder
	b.WriteString("# GoreGraph API Contracts\n\n")
	if len(records) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, record := range records {
		b.WriteString(fmt.Sprintf("- %s `%s` from `%s:%d` (app `%s`, %s)\n",
			record.HTTPMethod,
			record.Path,
			record.File,
			record.Line,
			record.App,
			record.Reason,
		))
	}
	return b.String()
}
