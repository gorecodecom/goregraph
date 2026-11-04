package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestReviewHTTPKeepsRegistrationAndNamedCallables(t *testing.T) {
	root := t.TempDir()
	const source = `package main
import "net/http"
func main() {
	http.HandleFunc("/items/delete", authorize(deleteItem))
}
func authorize(next http.HandlerFunc) http.HandlerFunc { return next }
func deleteItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { return }
}
`
	writeContextSourceFile(t, root, filepath.Join("Keeper", "main.go"), source)
	file, err := readSourceFile(filepath.Join(root, "Keeper", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadedContextIndex{
		ScopeRoot: root, Workspace: true,
		Index: scan.AgentContextIndexRecord{
			Facts:        []scan.AgentContextFactRecord{{ID: "route", Project: "Keeper", File: "main.go", Kind: "route", Line: 4, Path: "/items/delete"}},
			SourceHashes: map[string]string{"Keeper/main.go": file.Hash},
		},
	}
	request, err := normalizeContextRequest(ContextRequest{
		Root: filepath.Join(root, "Keeper"), Mode: "review", Query: "HTTP routes and authorization",
	})
	if err != nil {
		t.Fatal(err)
	}
	pack, err := buildReviewContext(loaded, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.SourceSections) != 3 || len(pack.Review[0].Sections) != 3 ||
		!strings.Contains(pack.Review[0].Gap, "effective access controls") {
		t.Fatalf("unexpected route review: %+v", pack)
	}
	for i, want := range []string{"authorize(deleteItem)", "func authorize", "func deleteItem"} {
		if !strings.Contains(pack.SourceSections[i].Content, want) || pack.SourceSections[i].ReadReceipt == "" {
			t.Fatalf("section %d: %+v", i, pack.SourceSections[i])
		}
	}
	loaded.Index.SourceHashes["Keeper/main.go"] = "stale"
	if got := reviewHTTPSections(loaded, request.Query); len(got) != 0 {
		t.Fatalf("stale route produced evidence: %+v", got)
	}
}
