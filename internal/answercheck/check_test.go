package answercheck

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckDeliveredEvidenceAndSafeRepair(t *testing.T) {
	files := []File{{Path: "service/src/Handler.java", Ranges: []Range{{10, 20}, {21, 30}}}, {Path: "config/app.yml"}}
	cases := []struct {
		name, answer, want string
		valid              bool
		repairs, refs      int
		code               string
	}{
		{"unique basename", "Uses `Handler.java:10-30`.", "Uses `service/src/Handler.java:10-30`.", true, 1, 1, ""},
		{"ellipsis", "`service/.../Handler.java#L12-L14`", "`service/src/Handler.java#L12-L14`", true, 1, 1, ""},
		{"full target short label", "[`Handler.java`](/work/service/src/Handler.java:10)", "", true, 0, 1, ""},
		{"metadata identity", "`config/app.yml`", "", true, 0, 1, ""},
		{"metadata source claim", "`config/app.yml:1`", "", false, 0, 1, "undelivered_range"},
		{"past EOF", "`service/src/Handler.java:31`", "", false, 0, 1, "undelivered_range"},
		{"missing", "`Unknown.java:10`", "", false, 0, 1, "unknown_path"},
		{"escaping", "`../Handler.java:10`", "", false, 0, 1, "unsafe_path"},
		{"zero range", "`service/src/Handler.java:0`", "", false, 0, 1, "invalid_range"},
		{"overflow", "`service/src/Handler.java:99999999999999999999999`", "", false, 0, 1, "invalid_range"},
		{"status label", "HTTP 403/404 for `service/src/Handler.java`", "", true, 0, 1, ""},
		{"adjacent German", "`service/src/Handler.java` (Zeilen 10–30)", "", true, 0, 1, ""},
		{"adjacent unsupported", "`service/src/Handler.java` lines 10, 999", "", false, 0, 1, "undelivered_range"},
		{"no references", "Everything works.", "", false, 0, 0, "no_references"},
		{"code fences ignored", "```go\nHandler.java:999\n```", "", false, 0, 0, "no_references"},
		{"receipt ignored", "`read_receipt:Handler.java:10`", "", false, 0, 0, "no_references"},
		{"URL unchanged", "[docs](https://example.test/Handler.java:999) `config/app.yml`", "", true, 0, 1, ""},
		{"unsafe link", "[source](file:///etc/Handler.java:10)", "", false, 0, 1, "unsafe_path"},
		{"unknown anchor", "`service/src/Handler.java#symbol`", "", false, 0, 1, "unsupported_citation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Check(Request{Answer: tc.answer, Root: "/work", Files: files, RepairPaths: true})
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = tc.answer
			}
			if got.Valid != tc.valid || got.Answer != want || len(got.Repairs) != tc.repairs || len(got.CheckedReferences) != tc.refs {
				t.Fatalf("got %+v; want valid=%v answer=%q repairs=%d refs=%d", got, tc.valid, want, tc.repairs, tc.refs)
			}
			if tc.code != "" && !hasCode(got, tc.code) {
				t.Fatalf("missing %s: %+v", tc.code, got)
			}
			if got.SemanticValidity != "not_verified" {
				t.Fatalf("semantic validity falsely certified: %+v", got)
			}
		})
	}
}

func TestTableCoverageAndAmbiguity(t *testing.T) {
	request := Request{Answer: "| File | Lines | Claim |\n| --- | --- | --- |\n| `Handler.java` | 10–12, 14 | HTTP 403/404 |\n", Files: []File{{Path: "src/Handler.java", Ranges: []Range{{10, 12}, {14, 14}}}}, RepairPaths: true}
	got, err := Check(request)
	if err != nil || !got.Valid || len(got.CheckedReferences) != 1 || len(got.CheckedReferences[0].Ranges) != 2 {
		t.Fatalf("table: %+v %v", got, err)
	}
	request.Answer = "`src/Handler.java:10-14`"
	got, _ = Check(request)
	if got.Valid || !hasCode(got, "undelivered_range") {
		t.Fatalf("gap accepted: %+v", got)
	}
	request.Answer = "`Handler.java:10`"
	request.Files = append(request.Files, File{Path: "other/Handler.java", Ranges: []Range{{10, 10}}})
	got, _ = Check(request)
	if got.Valid || len(got.Repairs) != 0 || got.Answer != request.Answer || !hasCode(got, "ambiguous_path") {
		t.Fatalf("ambiguous repair: %+v", got)
	}
}

func TestAmbiguousBasenameUsesUniqueRangeCoverage(t *testing.T) {
	request := Request{
		Answer:      "`SecurityConfig.java:72`",
		Files:       []File{{Path: "service-a/SecurityConfig.java", Ranges: []Range{{72, 72}}}, {Path: "service-b/SecurityConfig.java", Ranges: []Range{{79, 79}}}},
		RepairPaths: true,
	}
	got, err := Check(request)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.Answer != "`service-a/SecurityConfig.java:72`" || len(got.Repairs) != 1 || len(got.CheckedReferences) != 1 || got.CheckedReferences[0].Path != "service-a/SecurityConfig.java" {
		t.Fatalf("unique range coverage did not disambiguate path: %+v", got)
	}

	request.Answer = "`SecurityConfig.java`"
	got, _ = Check(request)
	if got.Valid || !hasCode(got, "ambiguous_path") {
		t.Fatalf("metadata-only basename must remain ambiguous: %+v", got)
	}

	request.Answer = "`SecurityConfig.java:72`"
	request.Files[1].Ranges = []Range{{72, 72}}
	got, _ = Check(request)
	if got.Valid || !hasCode(got, "ambiguous_path") {
		t.Fatalf("range covered by multiple files must remain ambiguous: %+v", got)
	}
}

func TestRedactedCoverageIsDistinguished(t *testing.T) {
	got, err := Check(Request{Answer: "`src/F.go:1-3`", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}, RedactedRanges: []Range{{2, 3}}}}})
	if err != nil || !got.Valid || got.CheckedReferences[0].Coverage != "includes_redacted" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestDecodeRejectsMalformedAndBoundedRequests(t *testing.T) {
	for _, input := range []string{`{"answer":"x","other":true}`, `{"answer":"x"} {}`, `{"answer":"x","files":[{"path":"F.go","ranges":[[1]]}]}`, `{"answer":"x","files":[{"path":"F.go","ranges":[[1,2,3]]}]}`, `{"answer":"x","files":[{"path":"F.go","ranges":[[0,1]]}]}`, `{"answer":"x","files":[{"path":"../F.go"}]}`, `null`, strings.Repeat(" ", MaxRequestBytes+1)} {
		if _, err := Decode(strings.NewReader(input)); err == nil {
			t.Errorf("accepted invalid input %.100q", input)
		}
	}
	got, err := Decode(strings.NewReader(`{"answer":"` + "`src/F.go:1`" + `","files":[{"path":"src/F.go","ranges":[[1,1]]}]}`))
	if err != nil || got.Answer != "`src/F.go:1`" {
		t.Fatalf("%+v %v", got, err)
	}
}

func hasCode(result Result, code string) bool {
	for _, f := range result.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestRejectsUnsupportedOrUnsafeCitationForms(t *testing.T) {
	for _, answer := range []string{
		"`src/F.go:1` and `src/\nF.go:2`",
		"`src/F.go:1` and [F.go][source]",
		"`src/F.go:1` and [bad](src/\nF.go:2)",
		"`src/F.go:1` lines 1-",
		"`src/F.go:1` lines 1, xyz",
		"`src/F.go:1` lines 1foo",
		"`src/F.go:1` and `src//F.go:1`",
		"`src/F.go:1` and `src/\u202eF.go:1`",
	} {
		got, err := Check(Request{Answer: answer, Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 10}}}}})
		if err != nil || got.Valid || got.Answer != answer {
			t.Errorf("unsafe/unsupported citation accepted or changed: %q %+v %v", answer, got, err)
		}
	}
}

func TestAbsoluteAbbreviationNeedsRepair(t *testing.T) {
	request := Request{Root: "/work", Answer: "[source](/work/.../F.go:1)", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}}}
	got, err := Check(request)
	if err != nil || got.Valid || !hasCode(got, "incomplete_path") {
		t.Fatalf("%+v %v", got, err)
	}
	request.RepairPaths = true
	got, err = Check(request)
	if err != nil || !got.Valid || got.Answer != "[source](src/F.go:1)" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestTableProseRangesAndBytePreservation(t *testing.T) {
	answer := "Before  \r\n| Datei | Symbol und belegte Zeilen | Status |\r\n|---|---|---|\r\n| `module/.../F.go` | First Z. 1–2 und 4; next Z. 8/9 | HTTP 403/404 |\r\nAfter  \r\n"
	got, err := Check(Request{Answer: answer, Files: []File{{Path: "module/src/F.go", Ranges: []Range{{1, 2}, {4, 4}, {8, 9}}}}, RepairPaths: true})
	want := "Before  \r\n| Datei | Symbol und belegte Zeilen | Status |\r\n|---|---|---|\r\n| `module/src/F.go` | First Z. 1–2 und 4; next Z. 8/9 | HTTP 403/404 |\r\nAfter  \r\n"
	if err != nil || !got.Valid || got.Answer != want || got.CheckedRanges != 4 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRequestAndCitationLimits(t *testing.T) {
	cases := []Request{
		{Answer: strings.Repeat("x", MaxAnswerBytes+1)},
		{Files: make([]File, MaxFiles+1)},
		{Files: []File{{Path: "src/F.go", Ranges: make([]Range, MaxRanges+1)}}},
		{Files: []File{{Path: "src/F.go", Ranges: []Range{{1, MaxLine + 1}}}}},
		{Files: []File{{Path: "src/F.go"}, {Path: "src/F.go"}}},
		{Root: "relative"},
		{Files: []File{{Path: "src/\nF.go"}}},
		{Files: []File{{Path: "src/.../F.go"}}},
		{Answer: strings.Repeat("`src/F.go` ", MaxReferences+1), Files: []File{{Path: "src/F.go"}}},
	}
	for i, request := range cases {
		if _, err := Check(request); err == nil {
			t.Errorf("case %d exceeded bounds or malformed ledger accepted", i)
		}
	}
	got, err := Check(Request{Answer: "`src/F.go:1-2147483647`", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, MaxLine}}}}})
	if err != nil || !got.Valid {
		t.Fatalf("interval union should not enumerate lines: %+v %v", got, err)
	}
}

func TestTableSymbolLabelsSemicolonsAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		cell  string
		count int
		valid bool
	}{
		{"Class 1–2; `method2` 4; Other 8/9", 4, true},
		{"1–2; 4; 8–9", 3, true},
		{"discovered only; no source lines delivered", 0, true},
		{"HTTP 403/404; method 1–2", 1, true},
		{"method 1–2; next 5", 2, false},
	} {
		got, err := Check(Request{Answer: "| File | Symbol / lines |\n|---|---|\n| `src/F.go` | " + tc.cell + " |\n", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 2}, {4, 4}, {8, 9}}}}})
		if err != nil || got.Valid != tc.valid || got.CheckedRanges != tc.count {
			t.Errorf("%s: %+v %v", tc.cell, got, err)
		}
	}
}

func TestReceiptNamedFilesRemainExplicitReferences(t *testing.T) {
	for _, name := range []string{"ReceiptHandler.java", "read_receipt.go"} {
		got, err := Check(Request{Answer: "`src/" + name + ":1`", Files: []File{{Path: "src/" + name, Ranges: []Range{{1, 1}}}}})
		if err != nil || !got.Valid || len(got.CheckedReferences) != 1 {
			t.Fatalf("file name mistaken for receipt metadata: %+v %v", got, err)
		}
	}
}

func TestRepairs32TablePathsUsingOnlySuppliedLedger(t *testing.T) {
	request := Request{Root: "/this/workspace/does/not/exist", RepairPaths: true}
	var answer, want strings.Builder
	answer.WriteString("| File | Lines |\n|---|---|\n")
	want.WriteString("| File | Lines |\n|---|---|\n")
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("Handler%d.java", i)
		full := fmt.Sprintf("module%d/src/main/%s", i, name)
		token := name
		switch i % 4 {
		case 1:
			token = fmt.Sprintf("module%d/.../%s", i, name)
		case 2:
			token = fmt.Sprintf("module%d/…/%s", i, name)
		case 3:
			token = fmt.Sprintf(".../src/.../%s", name)
		}
		request.Files = append(request.Files, File{Path: full, Ranges: []Range{{1, 3}}})
		fmt.Fprintf(&answer, "| `%s` | 1–3 |\n", token)
		fmt.Fprintf(&want, "| `%s` | 1–3 |\n", full)
	}
	request.Answer = answer.String()
	got, err := Check(request)
	if err != nil || !got.Valid || len(got.Repairs) != 32 || len(got.CheckedReferences) != 32 || got.CheckedRanges != 32 || got.Answer != want.String() {
		t.Fatalf("ledger-only 32-row repair: %+v %v", got, err)
	}
}

func TestExplicitTableMarkersExcludeStatusCodeLabels(t *testing.T) {
	got, err := Check(Request{Answer: "| File | Lines |\n|---|---|\n| `src/F.go` | 401 Z. 10–12; 404 Z. 20–21; success lines 30–31 |\n", Files: []File{{Path: "src/F.go", Ranges: []Range{{10, 12}, {20, 21}, {30, 31}}}}})
	if err != nil || !got.Valid || got.CheckedRanges != 3 {
		t.Fatalf("status codes are not source lines: %+v %v", got, err)
	}
}

func TestReviewRejectsHiddenNumericCitationsAndContinuations(t *testing.T) {
	for _, answer := range []string{
		"| File | Lines |\n|---|---|\n| `src/F.go` | `999` |\n",
		"`src/F.go` lines 1..999",
		"`src/F.go` lines 1.999",
		"`src/F.go` lines 1 through 999",
		"`src/F.go` lines 1 to 999",
		"`src/F.go` lines 1 bis 999",
	} {
		got, err := Check(Request{Answer: answer, Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}}})
		if err != nil || got.Valid {
			t.Errorf("silently dropped numeric citation in %q: %+v %v", answer, got, err)
		}
	}
}

func TestExtensionlessLedgerIdentitiesAreChecked(t *testing.T) {
	for _, token := range []string{"/work/src/Dockerfile:999", "src/Dockerfile:999", "Dockerfile:999"} {
		got, err := Check(Request{Root: "/work", Answer: "`src/F.go:1` [build](" + token + ")", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}, {Path: "src/Dockerfile", Ranges: []Range{{1, 1}}}}, RepairPaths: true})
		if err != nil || got.Valid || len(got.CheckedReferences) != 2 || !hasCode(got, "undelivered_range") {
			t.Errorf("extensionless citation omitted: %s %+v %v", token, got, err)
		}
	}
}

func TestReviewRejectsUnknownLocalLinksAndMarkedCodeCitations(t *testing.T) {
	for _, answer := range []string{
		"`src/F.go:1` [missing](/work/src/Unknown.custom:999)",
		"`src/F.go:1` [escape](../../Dockerfile)",
		"`src/F.go:1` [unsafe](javascript:alert(1))",
		"| File | Lines |\n|---|---|\n| `src/F.go` | `Z. 999` |\n",
		"| File | Lines |\n|---|---|\n| `src/F.go` | `lines 999` |\n",
		"| File | Lines |\n|---|---|\n| `src/F.go` | Z. 1..999 |\n",
	} {
		got, err := Check(Request{Root: "/work", Answer: answer, Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}}})
		if err != nil || got.Valid {
			t.Errorf("unsupported local/numeric citation silently omitted: %q %+v %v", answer, got, err)
		}
	}
}

func TestReviewPaddedNumericCodeCitationsRemainChecked(t *testing.T) {
	for _, cell := range []string{"` 999 `", "`` 999 ``", "` Z. 999 `", "` lines 999 `"} {
		got, err := Check(Request{Answer: "| File | Lines |\n|---|---|\n| `src/F.go` | " + cell + " |\n", Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}}})
		if err != nil || got.Valid || got.CheckedRanges != 1 || !hasCode(got, "undelivered_range") {
			t.Errorf("padded numeric citation omitted: %q %+v %v", cell, got, err)
		}
	}
}

func TestReviewExternalURLSchemesAreCaseInsensitiveAndUnchanged(t *testing.T) {
	for _, url := range []string{"HTTPS://example.test/docs", "HtTpS://example.test/F.go:999", "HTTP://example.test/docs", "hTtP://example.test/docs", "MAILTO:someone@example.test"} {
		answer := "`src/F.go:1` [docs](" + url + ")"
		got, err := Check(Request{Answer: answer, Files: []File{{Path: "src/F.go", Ranges: []Range{{1, 1}}}}, RepairPaths: true})
		if err != nil || !got.Valid || len(got.CheckedReferences) != 1 || got.Answer != answer || len(got.Repairs) != 0 {
			t.Errorf("external URL changed or validated as local: %q %+v %v", url, got, err)
		}
	}
}

func TestAdjacentRangesEndBeforeNextMarkdownLink(t *testing.T) {
	for _, separator := range []string{", und ", ", and ", ", "} {
		t.Run(separator, func(t *testing.T) {
			answer := "[One.java](/workspace/src/One.java:23), Zeilen 23–41" + separator +
				"[Two.java](/workspace/src/Two.java:24), lines 24–49."
			got, err := Check(Request{
				Root: "/workspace", Answer: answer,
				Files: []File{
					{Path: "src/One.java", Ranges: []Range{{23, 41}}},
					{Path: "src/Two.java", Ranges: []Range{{24, 49}}},
				},
			})
			if err != nil || !got.Valid || got.Answer != answer || len(got.CheckedReferences) != 2 || got.CheckedRanges != 4 {
				t.Fatalf("link boundary rejected: %+v %v", got, err)
			}
		})
	}
}

func TestAdjacentLinkBoundaryPreservesRangeValidation(t *testing.T) {
	for _, tc := range []struct {
		name, tail, code string
	}{
		{"malformed list", "1-3, xyz, and [Next.java](src/Next.java:1)", "unsupported_citation"},
		{"empty list item", "1-3, , and [Next.java](src/Next.java:1)", "unsupported_citation"},
		{"trailing comma", "1-3,", "unsupported_citation"},
		{"trailing slash", "1-3/", "unsupported_citation"},
		{"trailing dash", "1-3-", "unsupported_citation"},
		{"incomplete link", "1-3, and [Next.java](", "unsupported_citation"},
		{"malformed later range", "1-3, 5-, and [Next.java](src/Next.java:1)", "unsupported_citation"},
		{"undelivered later range", "1-3, 5-7, and [Next.java](src/Next.java:1)", "undelivered_range"},
		{"undelivered next link", "1-3, and [Next.java](src/Next.java:99)", "undelivered_range"},
		{"semicolon negative", "1-3; -5, and [Next.java](src/Next.java:1)", "unsupported_citation"},
		{"semicolon empty item", "1-3; ; 5, and [Next.java](src/Next.java:1)", "unsupported_citation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer := "[One.java](src/One.java), lines " + tc.tail
			got, err := Check(Request{Answer: answer, Files: []File{
				{Path: "src/One.java", Ranges: []Range{{1, 3}}},
				{Path: "src/Next.java", Ranges: []Range{{1, 1}}},
			}})
			if err != nil || got.Valid || got.Answer != answer || !hasCode(got, tc.code) {
				t.Fatalf("expected %s: %+v %v", tc.code, got, err)
			}
		})
	}
}

func TestAdjacentSemicolonRangesCheckEveryInterval(t *testing.T) {
	for _, tc := range []struct {
		name, tail string
		ranges     []Range
		valid      bool
		count      int
		code       string
	}{
		{"all delivered", "Zeilen 24–35; 45–59; 61–65.", []Range{{24, 35}, {45, 59}, {61, 65}}, true, 3, ""},
		{"later uncovered", "Zeilen 24–35; 45–59; 61–65.", []Range{{24, 35}, {45, 59}}, false, 3, "undelivered_range"},
		{"mixed separators", "lines 24–35; 45–59, 61–65.", []Range{{24, 35}, {45, 59}, {61, 65}}, true, 3, ""},
		{"malformed later", "lines 24–35; 45-", []Range{{24, 35}, {45, 59}}, false, 2, "unsupported_citation"},
		{"negative later", "lines 24–35; -45", []Range{{24, 35}}, false, 1, "unsupported_citation"},
		{"empty later group", "lines 24–35; ; 45", []Range{{24, 35}}, false, 1, "unsupported_citation"},
		{"unsupported later continuation", "lines 24–35; 45 through 999", []Range{{24, 35}, {45, 45}}, false, 2, "unsupported_citation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer := "[F.go](src/F.go), " + tc.tail
			got, err := Check(Request{Answer: answer, Files: []File{{Path: "src/F.go", Ranges: tc.ranges}}, RepairPaths: true})
			if err != nil || got.Valid != tc.valid || got.CheckedRanges != tc.count || got.Answer != answer || len(got.CheckedReferences) != 1 {
				t.Fatalf("semicolon groups: %+v %v", got, err)
			}
			if tc.code != "" && !hasCode(got, tc.code) {
				t.Fatalf("missing %s: %+v", tc.code, got)
			}
		})
	}
}
