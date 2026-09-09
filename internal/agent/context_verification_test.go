package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
)

func TestVerificationRequestRetainsExactBoundedOmission(t *testing.T) {
	pack := ContextPack{SourceOmissions: []ContextSourceOmission{{
		Project: "service", Path: "src/handler.ts", StartLine: 4, EndLine: 9,
		Role: "call_chain", Reason: "required source evidence is missing",
	}}}
	got := contextVerificationRequests(pack)
	if len(got) != 1 || got[0].Path != "src/handler.ts" || got[0].StartLine != 4 || got[0].EndLine != 9 {
		t.Fatalf("requests = %#v", got)
	}
}

func TestVerificationRequestsRejectUnsafeAndUnboundedPaths(t *testing.T) {
	pack := ContextPack{SourceOmissions: []ContextSourceOmission{
		{Path: "../secret", StartLine: 1, EndLine: 2, Reason: "missing evidence"},
		{Path: `C:\\secret`, StartLine: 1, EndLine: 2, Reason: "missing evidence"},
		{Path: "src/no-range.ts", Reason: "missing evidence"},
		{Path: "src/reversed.ts", StartLine: 9, EndLine: 4, Reason: "missing evidence"},
	}}
	if got := contextVerificationRequests(pack); len(got) != 0 {
		t.Fatalf("unsafe verification requests = %#v", got)
	}
}

func TestVerificationRequestsDeduplicateSortAndCap(t *testing.T) {
	pack := ContextPack{SourceOmissions: []ContextSourceOmission{
		{Project: "z", Path: "src/z.ts", StartLine: 4, EndLine: 8, Reason: "z"},
		{Project: "a", Path: "src/c.ts", StartLine: 2, EndLine: 3, Reason: "c"},
		{Project: "a", Path: "src/a.ts", StartLine: 8, EndLine: 9, Reason: "a"},
		{Project: "a", Path: "src/b.ts", StartLine: 5, EndLine: 6, Reason: "b"},
		{Project: "a", Path: "src/a.ts", StartLine: 8, EndLine: 9, Reason: "duplicate"},
	}}
	want := []ContextVerificationRequest{
		{Project: "a", Path: "src/a.ts", StartLine: 8, EndLine: 9, Reason: "a"},
		{Project: "a", Path: "src/b.ts", StartLine: 5, EndLine: 6, Reason: "b"},
		{Project: "a", Path: "src/c.ts", StartLine: 2, EndLine: 3, Reason: "c"},
	}
	if got := contextVerificationRequests(pack); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %#v, want %#v", got, want)
	}
}

func TestVerificationRequestsSkipOperationallyUnavailableSource(t *testing.T) {
	pack := ContextPack{SourceOmissions: []ContextSourceOmission{
		{Path: "src/missing.ts", StartLine: 1, EndLine: 2, Reason: "source file is missing"},
		{Path: "src/unreadable.ts", StartLine: 1, EndLine: 2, Reason: "source file is unreadable"},
		{Path: "src/unsafe.ts", StartLine: 1, EndLine: 2, Reason: "source path escapes project root"},
	}}
	if got := contextVerificationRequests(pack); len(got) != 0 {
		t.Fatalf("unavailable source became verification requests: %#v", got)
	}
}

func TestStrictContextRequestDoesNotExposeAdaptiveMetadata(t *testing.T) {
	request, err := normalizeContextRequest(ContextRequest{Query: "GET /users"})
	if err != nil {
		t.Fatal(err)
	}
	if request.ProtocolVersion != StrictV1 {
		t.Fatalf("default protocol = %q", request.ProtocolVersion)
	}
	pack := adaptiveContextMetadata(ContextPack{
		ProtocolVersion: request.ProtocolVersion,
		SourceOmissions: []ContextSourceOmission{{Path: "src/user.ts", StartLine: 1, EndLine: 2}},
	})
	if len(pack.VerificationRequests) != 0 {
		t.Fatalf("strict pack exposed adaptive verification requests: %#v", pack)
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"protocol_version", "generation", "health", "verification_requests"} {
		if strings.Contains(string(body), `"`+field+`"`) {
			t.Fatalf("strict pack exposed %q: %s", field, body)
		}
	}
}

func TestAdaptiveContextMetadataIncludesProtocolGenerationAndBoundedRequests(t *testing.T) {
	pack := adaptiveContextMetadata(ContextPack{
		ProtocolVersion: AdaptiveV2,
		Generation:      "generation-7",
		SourceOmissions: []ContextSourceOmission{{
			Project: "service", Path: "src/user.ts", StartLine: 3, EndLine: 7,
			Reason: "required source evidence is missing",
		}},
	})
	if pack.Generation != "generation-7" || len(pack.VerificationRequests) != 1 {
		t.Fatalf("adaptive metadata = %#v", pack)
	}
}

func TestAdaptiveContextMetadataDoesNotInventGeneration(t *testing.T) {
	pack := adaptiveContextMetadata(ContextPack{
		ProtocolVersion: AdaptiveV2,
		Freshness:       "2026-09-09T08:00:00Z",
	})
	if pack.Generation != "" {
		t.Fatalf("generation = %q, want empty without manifest identity", pack.Generation)
	}
}

func TestLoadContextProjectionHealthUsesSelectedManifest(t *testing.T) {
	outputRoot := t.TempDir()
	agentDir := filepath.Join(outputRoot, "agent")
	if err := os.Mkdir(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := scan.OutputManifest{
		GenerationID:     "generation-7",
		AnalysisCoverage: "partial",
		BuildIdentity:    scan.CurrentBuildIdentity(config.Defaults(), scan.DefaultBuildOptions(), "ignore", "source"),
		Tool:             scan.ToolName,
		Schema:           scan.SchemaVersion,
		Agent: scan.ProjectionStatus{
			InputFingerprint: "fingerprint-7",
			GeneratedAt:      "2026-09-09T08:00:00Z",
			Complete:         true,
		},
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputRoot, "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	health := loadContextProjectionHealth(filepath.Join(agentDir, "context-index.json"))
	if health.GenerationID != "generation-7" || health.Integrity != "valid" ||
		health.Freshness != "unknown" || health.Coverage != "partial" {
		t.Fatalf("health = %#v", health)
	}
}

func TestAdaptivePartialAnalysisIsExplicitlyUncertain(t *testing.T) {
	health := scan.ProjectionHealth{
		GenerationID: "generation-7", Integrity: "valid", Freshness: "unknown", Coverage: "partial",
	}
	pack := applyAdaptiveContextHealth(ContextPack{ProtocolVersion: AdaptiveV2}, health)
	if pack.Generation != "generation-7" || pack.Health == nil ||
		!contextUncertaintyExists(pack.Uncertainties, "analysis", adaptivePartialAnalysisReason) {
		t.Fatalf("adaptive health metadata = %#v", pack)
	}
}

func TestContextIdentitySeparatesProtocolsAndGenerations(t *testing.T) {
	strict := contextIdentityForProtocol(StrictV1, "generation-1", []string{"route"}, nil, nil)
	adaptive := contextIdentityForProtocol(AdaptiveV2, "generation-1", []string{"route"}, nil, nil)
	newGeneration := contextIdentityForProtocol(StrictV1, "generation-2", []string{"route"}, nil, nil)
	if strict == adaptive || strict == newGeneration || adaptive == newGeneration {
		t.Fatalf("context identities were mixed: %q %q %q", strict, adaptive, newGeneration)
	}
}

func TestNormalizeContextRequestRejectsUnknownProtocol(t *testing.T) {
	if _, err := normalizeContextRequest(ContextRequest{Query: "GET /users", ProtocolVersion: "unknown"}); err == nil {
		t.Fatal("accepted unknown context protocol")
	}
}

func TestAdaptiveFallbackUsesStableReasonCodes(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		pack   ContextPack
		want   string
	}{
		{name: "ambiguous entrypoint", reason: "matching endpoint provider is ambiguous", want: ContextFallbackAmbiguousEntrypoint},
		{name: "unsupported", reason: "all selected context scopes have incomplete coverage", want: ContextFallbackUnsupportedAnalysis},
		{name: "budget", reason: finalContextBudgetFallbackReason, want: ContextFallbackBudgetExhausted},
		{name: "conflict", reason: "current declaration contradicts indexed evidence", want: ContextFallbackEvidenceConflict},
		{name: "stale", reason: "indexed symbol is absent from current source", want: ContextFallbackIndexStale},
		{name: "unreadable", pack: ContextPack{SourceOmissions: []ContextSourceOmission{{Reason: "source file is unreadable"}}}, want: ContextFallbackSourceUnreadable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pack := test.pack
			pack.ProtocolVersion = AdaptiveV2
			pack.FallbackRequired = true
			pack.FallbackReason = test.reason
			got := adaptiveContextMetadata(pack)
			if got.FallbackReason != test.want {
				t.Fatalf("fallback reason = %q, want %q", got.FallbackReason, test.want)
			}
		})
	}
}

func TestAdaptiveMissingIndexReturnsBoundedFallback(t *testing.T) {
	pack, err := BuildContext(ContextRequest{
		Root: t.TempDir(), Query: "GET /users", ProtocolVersion: AdaptiveV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.FallbackRequired || pack.FallbackReason != ContextFallbackIndexMissing ||
		pack.ProtocolVersion != AdaptiveV2 || pack.Generation != "" || pack.Health == nil ||
		len(pack.VerificationRequests) != 0 {
		t.Fatalf("missing-index fallback = %#v", pack)
	}
}

func TestAdaptivePreviousContextMismatchDoesNotInventStaleness(t *testing.T) {
	root := writeSourceBackedContextFixture(t, false)
	pack, err := BuildContext(ContextRequest{
		Root: root, Query: "DELETE /cadasters/{cadasterId}/regulations/{objectId}",
		ProtocolVersion: AdaptiveV2, PreviousContextID: "000000000000000000000000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.FallbackReason == ContextFallbackIndexStale || pack.DuplicateOf != "" || pack.ContextID == "" {
		t.Fatalf("mismatched previous context = %#v", pack)
	}
}

func TestAdaptiveHealthUsesActualStaleSignal(t *testing.T) {
	pack := applyAdaptiveContextHealth(ContextPack{ProtocolVersion: AdaptiveV2}, scan.ProjectionHealth{
		Integrity: "valid", Freshness: "stale", Coverage: "complete",
	})
	if got := adaptiveHealthFallbackReason(pack); got != ContextFallbackIndexStale {
		t.Fatalf("fallback reason = %q, want %q", got, ContextFallbackIndexStale)
	}
}

func TestResolveSourcePathRetainsMissingRootCause(t *testing.T) {
	_, err := resolveSourcePath(
		loadedContextIndex{ScopeRoot: t.TempDir() + "-missing"},
		sourceCandidate{Path: "src/user.ts"},
	)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolve source error = %v, want os.ErrNotExist", err)
	}
	if got := stableContextSourceOmissionReason(err); got != "source file is missing" {
		t.Fatalf("stable omission = %q", got)
	}
}
