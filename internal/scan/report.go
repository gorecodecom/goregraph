package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func renderReport(root string, files []FileRecord, skipped int) string {
	langs := map[string]int{}
	for _, file := range files {
		langs[file.Language]++
	}
	var langNames []string
	for lang := range langs {
		langNames = append(langNames, lang)
	}
	sort.Strings(langNames)

	var b strings.Builder
	b.WriteString("# GoreGraph Report\n\n")
	b.WriteString("## Project Summary\n\n")
	b.WriteString(fmt.Sprintf("- Project: %s\n", filepath.Base(root)))
	b.WriteString(fmt.Sprintf("- Files scanned: %d\n", len(files)))
	b.WriteString(fmt.Sprintf("- Files skipped: %d\n", skipped))
	b.WriteString("\n## Languages\n\n")
	for _, lang := range langNames {
		b.WriteString(fmt.Sprintf("- %s: %d\n", lang, langs[lang]))
	}
	if len(langNames) == 0 {
		b.WriteString("- none: 0\n")
	}
	b.WriteString("\n## Files\n\n")
	for _, file := range files {
		b.WriteString(fmt.Sprintf("- `%s` (%s, %d bytes)\n", file.Path, file.Language, file.Size))
	}
	return b.String()
}

func renderModulesReport(files []FileRecord) string {
	type moduleStats struct {
		files     int
		languages map[string]int
	}
	modules := map[string]moduleStats{}
	for _, file := range files {
		name := topLevelModule(file.Path)
		stats := modules[name]
		if stats.languages == nil {
			stats.languages = map[string]int{}
		}
		stats.files++
		stats.languages[file.Language]++
		modules[name] = stats
	}

	var names []string
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# GoreGraph Modules\n\n")
	if len(names) == 0 {
		b.WriteString("- none\n")
		return b.String()
	}
	for _, name := range names {
		stats := modules[name]
		b.WriteString(fmt.Sprintf("- `%s` - %d files", name, stats.files))
		langs := sortedLanguageCounts(stats.languages)
		if len(langs) > 0 {
			b.WriteString(" - ")
			b.WriteString(strings.Join(langs, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderEntrypointsReport(files []FileRecord, symbols []SymbolRecord) string {
	entrypoints := map[string][]string{}
	for _, symbol := range symbols {
		if symbol.Kind == "function" && symbol.Name == "main" && isGoMainFile(symbol.File, symbols) {
			entrypoints[symbol.File] = append(entrypoints[symbol.File], "Go main function")
		}
		if symbol.Kind == "script" && symbol.File == "package.json" {
			entrypoints[symbol.File] = append(entrypoints[symbol.File], "script "+symbol.Name)
		}
		if symbol.Kind == "entrypoint" {
			entrypoints[symbol.File] = append(entrypoints[symbol.File], symbol.Name)
		}
	}
	for _, file := range files {
		if filepath.Base(file.Path) == "package.json" && len(entrypoints[file.Path]) == 0 {
			entrypoints[file.Path] = append(entrypoints[file.Path], "package scripts")
		}
		if file.Language == "php" && filepath.Base(file.Path) == "index.php" {
			entrypoints[file.Path] = append(entrypoints[file.Path], "PHP front controller")
		}
	}

	var paths []string
	for path := range entrypoints {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("# GoreGraph Entrypoints\n\n")
	if len(paths) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, path := range paths {
		reasons := entrypoints[path]
		sort.Strings(reasons)
		b.WriteString(fmt.Sprintf("- `%s` - %s\n", path, strings.Join(reasons, ", ")))
	}
	return b.String()
}

func renderTestMapReport(relations []RelationRecord) string {
	var tests []RelationRecord
	for _, relation := range relations {
		if relation.Type == "tests" {
			tests = append(tests, relation)
		}
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].To != tests[j].To {
			return tests[i].To < tests[j].To
		}
		return tests[i].From < tests[j].From
	})

	var b strings.Builder
	b.WriteString("# GoreGraph Test Map\n\n")
	if len(tests) == 0 {
		b.WriteString("- none detected\n")
		return b.String()
	}
	for _, relation := range tests {
		b.WriteString(fmt.Sprintf("- `%s` tests `%s`\n", relation.From, relation.To))
	}
	return b.String()
}

func topLevelModule(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "."
	}
	if len(parts) == 1 {
		return "."
	}
	return parts[0] + "/"
}

func sortedLanguageCounts(counts map[string]int) []string {
	var languages []string
	for language := range counts {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	var result []string
	for _, language := range languages {
		result = append(result, fmt.Sprintf("%s: %d", language, counts[language]))
	}
	return result
}

func isGoMainFile(path string, symbols []SymbolRecord) bool {
	for _, symbol := range symbols {
		if symbol.File == path && symbol.Kind == "package" && symbol.Name == "main" {
			return true
		}
	}
	return false
}
