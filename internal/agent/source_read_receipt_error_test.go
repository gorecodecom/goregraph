package agent

import (
	"strings"
	"testing"
)

func TestSourceReadReceiptErrorIdentifiesFileAndReceiptBeforeSourceAccess(t *testing.T) {
	valid := makeSourceReadReceipt(strings.Repeat("a", 64), []SourceReadRange{{1, 4}})
	invalid := "r1:" + strings.Repeat("b", 62) + ":1-4"
	result, err := ReadSource(ReadSourceRequest{Root: t.TempDir(), Files: []SourceReadFileRequest{
		{Path: "First.java", Ranges: []SourceReadRange{{5, 6}}, Seen: []string{valid}},
		{Path: "Second.java", Ranges: []SourceReadRange{{5, 6}}, Seen: []string{valid, invalid}},
	}})
	if err == nil || len(result.Files) != 0 {
		t.Fatalf("invalid receipt was accepted: %+v %v", result, err)
	}
	for _, part := range []string{"files[1]", "Second.java", "seen[1]", "copy the exact", "64-character"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("missing actionable %q in %v", part, err)
		}
	}
	if strings.Contains(err.Error(), invalid) {
		t.Fatal("error unnecessarily echoed the malformed receipt")
	}
}
