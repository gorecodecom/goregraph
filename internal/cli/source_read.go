package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gorecodecom/goregraph/internal/agent"
)

func runSourceRead(args []string, stdout, stderr io.Writer) int {
	const help = `Usage: goregraph read <root> --request '<JSON>'

Read bounded indexed source, returning only lines not in supplied receipts.
Each file requires either nonempty files[].ranges OR files[].find, never both.
Repeated find entries, including aliases, share one canonical file result with
independent selectors. Do not mix ranges and find for that file; use separate
requests. This CLI grants no read permission.
Use exact ranges wherever caller authority is limited to specific line ranges.

Range batch (replace RECEIPT with a complete read_receipt or prior receipt):
  goregraph read . --request '{"files":[{"path":"src/Handler.java","ranges":[[10,30]],"seen":["RECEIPT"]},{"path":"src/Model.java","ranges":[[1,40]]}]}'
Find plus context in an authorized file:
  goregraph read . --request '{"files":[{"path":"src/Handler.java","find":{"pattern":"handle|validate","before":2,"after":5,"max_matches":4}}]}'
Independent finds in the same authorized file:
  goregraph read . --request '{"files":[{"path":"src/Handler.java","find":{"pattern":"handle","max_matches":2}},{"path":"src/Handler.java","find":{"pattern":"validate","start_line":20,"max_matches":3}}]}'

CLI-only shorthand: {"path":"src/Handler.java","start_line":10,"end_line":30}
normalizes to "ranges":[[10,30]]. With find, a file-level start_line normalizes to
files[].find.start_line only when the nested cursor is zero or omitted; for example
{"path":"src/Handler.java","start_line":10,"find":{"pattern":"handle"}}.
Shorthand endpoints must be positive JSON integers; no missing range endpoint,
nonempty ranges, find plus end_line, or two nonzero cursor locations are accepted.
Canonical files[].ranges and files[].find.start_line are preferred. Shorthand
has identical read authority, validation, receipt handling, and limits.
After one complete, strictly decoded request object, the CLI tolerates exactly one
redundant trailing ]} pair. No other trailing token or second JSON value is accepted.
If strict decoding fails only because a JSON string contains a backslash before a
non-JSON escape character, the CLI treats that backslash literally and decodes
again. Unknown fields, malformed standard escapes and later validation still fail.

find.pattern is a nonempty Go regexp (RE2), at most 1024 bytes. For literal text,
escape regexp metacharacters and double backslashes in JSON; for example,
"pattern":"handle\\(" matches a literal handle(. Invalid regex is rejected.
Matching is per
line after whole-file configuration redaction, without rendered line numbers;
patterns cannot match across lines. before/after default to 0 (allowed 0..100).
max_matches defaults to 10 when 0 or omitted (otherwise 1..32);
files[].find.start_line defaults to 1 when 0 or omitted (otherwise 1..2097153).
Context windows clamp to EOF and
merge overlapping or adjacent windows before receipt subtraction.

Output files contain canonical root-relative path, sections (start_line, end_line,
content), skipped_ranges, and cumulative receipt. eof_ranges reports requested
ranges past EOF; ignored_receipts counts changed content/path receipts. Find adds
find.match_lines (selected line numbers, [] if none), match_count (all matching
lines at/after start_line, including already-seen lines), and next_start_line only
if more matches remain. Copy output files[].find.next_start_line to request
files[].find.start_line and the cumulative receipt to seen.
When find windows would exceed the interval, line, or 24 KiB result limit, the
reader reduces match pages in reverse request order while retaining at least one
match per selector. A reduced result has output_limited:true and next_start_line;
resume only selectors whose remaining matches matter. Exact ranges stay atomic.
If the aggregate response remains too large, later whole-file results are returned
with file-level output_limited:true and empty sections. Repeat the same selectors
with cumulative receipts from files already delivered; exact ranges are never split.
When multiple find entries resolve to one canonical file, find is omitted and
find_results contains {"request_index":0,"result":{"match_lines":[10],"match_count":1}}
per selector in request order. request_index is the original zero-based files[]
index; result has the same metadata as single find. Each selector keeps its own
pattern, cursor, match limit and context window. Copy each result.next_start_line
to its original selector's find.start_line. Ranges and seen receipts are unioned
for the canonical file, so overlapping source is delivered once. Match metadata is
navigation, not newly delivered source. No matches returns empty sections;
existing valid receipts are retained. Copy context
source_sections.read_receipt into seen and carry later cumulative receipts forward.
Missing output locks fail without writes; a separately authorized GoreGraph build
or context command can initialize legacy locks.
Limits: 16 raw file entries (aliases count), 32 requested ranges or merged find
windows per batch, 500 lines/original
range, 1000 requested lines total, 64 receipts, 64 intervals/receipt, 64 KiB request
JSON, 24 KiB result JSON including newline. Find pages shrink automatically where
possible, then whole files page automatically. Requests still fail atomically when
one file's exact ranges or minimum find page cannot fit; reduce ranges, find
before/after, or split that request.
`
	if len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) != 3 || args[1] != "--request" || strings.HasPrefix(args[0], "--") {
		fmt.Fprintln(stderr, "error: usage: goregraph read <root> --request '<JSON>'")
		return 2
	}
	if len(args[2]) > agent.MaxSourceReadRequestBytes {
		fmt.Fprintln(stderr, "error: source read request exceeds 64 KiB")
		return 2
	}
	request, err := decodeSourceReadRequest(args[2])
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	request.Root = args[0]
	result, err := agent.ReadSource(request)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}
