package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gorecodecom/goregraph/internal/answercheck"
)

func runAnswerCheck(args []string, stdout, stderr io.Writer) int {
	const help = `Usage: goregraph answer-check --request '<JSON>' | --request-file <path>

Check explicit Markdown file identities and line citations against a caller-provided
ledger. This command does not read source files, grant permission, authenticate
that ledger, or prove semantic correctness. Independent semantic review remains
required. The only file opened is an explicitly supplied --request-file JSON file.

Request: {"answer":"Uses ` + "`src/Handler.java:10-20`" + `.","root":"/workspace",
"files":[{"path":"src/Handler.java","ranges":[[10,20]],"redacted_ranges":[]}],
"repair_paths":true}

files[].path is an exact discovered root-relative file. Ranges are inclusive
numbered source actually delivered to the caller; receipts, matches and EOF
metadata do not establish delivery. Redacted ranges establish keys, never values.
Metadata-only files may omit ranges. Unknown fields and trailing JSON are rejected.
Only uniquely resolvable basename or /.../ or /…/ path tokens can be repaired;
claims, line ranges, code fences, receipt strings and external URLs stay unchanged.
Supports inline-code file references, inline Markdown link targets, adjacent
Z./Zeilen/lines ranges, and recognized table citation columns. This is a bounded
parser, not exhaustive Markdown or semantic validation. Zero references fails.

Limits: 1 MiB request, 512 KiB answer, 1024 ledger files, 4096 ledger ranges,
4096 references, 4096 cited ranges, 4096 bytes/path, lines 1..2147483647.
Exit codes: 0 valid within stated scope; 1 findings (JSON report) or output error;
2 usage, request decoding or limits error. semantic_validity is always not_verified.
`
	if len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) != 2 || (args[0] != "--request" && args[0] != "--request-file") {
		fmt.Fprintln(stderr, "error: usage: goregraph answer-check --request '<JSON>' | --request-file <path>")
		return 2
	}
	var reader io.Reader
	if args[0] == "--request" {
		reader = strings.NewReader(args[1])
	} else {
		info, err := os.Stat(args[1])
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		if !info.Mode().IsRegular() {
			fmt.Fprintln(stderr, "error: request file must be a regular JSON file")
			return 2
		}
		file, err := os.Open(args[1])
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		defer file.Close()
		info, err = file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			fmt.Fprintln(stderr, "error: request file must be a regular JSON file")
			return 2
		}
		reader = file
	}
	request, err := answercheck.Decode(reader)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	result, err := answercheck.Check(request)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if !result.Valid {
		return 1
	}
	return 0
}
