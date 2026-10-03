package scan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceQualityUsesReconciledContracts(t *testing.T) {
	project := workspaceIndexProject{
		record:   WorkspaceProjectRecord{Path: "web/store"},
		evidence: []EvidenceRecord{{ID: "evidence:call", File: "src/api.ts", Start: EvidenceLocation{Line: 8}}},
	}
	matched := WorkspaceContractMatchRecord{APIProject: project.record.Path, APIHTTPMethod: "GET", APIPath: "/orders", APIFile: "src/api.ts", APILine: 8, Issue: contractIssueMatched}
	if diagnostics := buildWorkspaceProjectDiagnostics(project, []WorkspaceContractMatchRecord{matched}); len(diagnostics) != 0 {
		t.Fatalf("resolved workspace call retained stale project diagnostics: %#v", diagnostics)
	}
	missing := matched
	missing.Issue = contractIssueMissingRoute
	mismatch := matched
	mismatch.Issue, mismatch.APIPath = contractIssueMethodMismatch, "/invoices"
	other := missing
	other.APIProject = "web/other"
	project.capabilities = []CapabilityRecord{{ID: CapabilityCalls, Project: project.record.Path, Language: "javascript", Coverage: CoverageFailed}}
	diagnostics := buildWorkspaceProjectDiagnostics(project, []WorkspaceContractMatchRecord{matched, missing, mismatch, other})
	if len(diagnostics) != 3 {
		t.Fatalf("workspace quality dropped real issues or included another consumer: %#v", diagnostics)
	}
	for _, code := range []string{contractIssueMissingRoute, contractIssueMethodMismatch, "analyzer_failed"} {
		findCanonicalDiagnostic(t, diagnostics, code)
	}
	unresolved := findCanonicalDiagnostic(t, diagnostics, contractIssueMissingRoute)
	if unresolved.Resolution != ResolutionUnresolved || len(unresolved.EvidenceIDs) != 1 || unresolved.EvidenceIDs[0] != WorkspaceEvidenceID(project.record.Path, "evidence:call") {
		t.Fatalf("missing-route diagnosis lost its status or evidence: %#v", unresolved)
	}
	families := BuildDiagnosticFamilies(project.record.Path, diagnostics)
	if len(families) != 3 {
		t.Fatalf("workspace diagnostic families: %#v", families)
	}
	for _, family := range families {
		if family.Service != project.record.Path || family.UnresolvedCount != 1 {
			t.Fatalf("incorrect workspace family ownership/accounting: %#v", family)
		}
	}
}

func TestCanonicalDiagnosticsDoNotFlagMatchedRoutes(t *testing.T) {
	if diagnostics := BuildCanonicalDiagnostics([]ContractMatchRecord{{Issue: contractIssueMatched}}, nil); len(diagnostics) != 0 {
		t.Fatalf("matched route emitted an open diagnostic: %#v", diagnostics)
	}
}

func TestWorkspaceQualityGermanExplanationsInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered dashboard quality tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed: %s", output)
	}
	html := workspaceQualityHTML()
	previewDir := t.TempDir()
	if evidenceDir := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR"); evidenceDir != "" {
		previewDir = evidenceDir
	}
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(previewDir, "quality-preview.html")
	if err := os.WriteFile(preview, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	encodedPreview, _ := json.Marshal(preview)
	encodedDir, _ := json.Marshal(previewDir)
	source := strings.Join([]string{
		`const {chromium}=require('playwright'),{pathToFileURL}=require('url'),path=require('path'),preview=` + string(encodedPreview) + `,evidenceDir=` + string(encodedDir) + `;`,
		`(async()=>{const options={headless:true};if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)options.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;const browser=await chromium.launch(options);try{const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];page.on('pageerror',error=>errors.push(error.message));page.on('console',message=>{if(message.type()==='error')errors.push(message.text());});await page.goto(pathToFileURL(preview).href+'#quality');await page.getByRole('heading',{name:'Datenqualität',exact:true}).waitFor();await page.locator('.diagnostic-list summary').click();const text=await page.locator('#content').innerText();for(const expected of ['Für diese Fähigkeit nicht vorgesehen','Für Konfigurationsdateien','Statisch erkennbare Muster','Analyse fehlgeschlagen','Der aktive Analyzer unterstützt','HTTP-Methode weicht','Vergleiche die HTTP-Methode','Veraltet','json: Für diese Fähigkeit nicht vorgesehen'])if(!text.includes(expected))throw new Error('Missing German explanation: '+expected);for(const english of ['Supported static patterns','The active analyzer','A related backend route','Compare the client method'])if(text.includes(english))throw new Error('Untranslated explanation: '+english);if(!text.includes('api_clients')||!text.includes('method_mismatch')||!text.includes('/orders'))throw new Error('Technical identities changed');await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(evidenceDir,'quality-desktop.png'),fullPage:true});await page.getByRole('button',{name:'Abweichende Aufrufe'}).click();await page.getByRole('heading',{name:'Schnittstellen',exact:true}).waitFor();if(!(await page.locator('#content').innerText()).includes('/orders'))throw new Error('Diagnostic navigation lost the call');await page.locator('[data-area="quality"]').click();await page.getByRole('heading',{name:'Datenqualität',exact:true}).waitFor();await page.locator('[data-module-scope="all"]').click();if(!(await page.locator('#content').innerText()).includes('Für Konfigurationsdateien'))throw new Error('Workspace scope lost explanations');await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(evidenceDir,'quality-mobile.png'),fullPage:true});if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw new Error('Dashboard overflows narrow viewport');if(errors.length)throw new Error('Browser errors: '+errors.join('; '));console.log('German quality explanations, workspace scope, diagnostic navigation and narrow viewport passed');}finally{await browser.close();}})().catch(error=>{console.error(error);process.exit(1);});`,
	}, "\n")
	if output, err := nodeScriptCommand(node, source).CombinedOutput(); err != nil {
		t.Fatalf("rendered quality check failed: %v\n%s", err, output)
	}
}

func workspaceQualityHTML() string {
	capabilities := []CapabilityRecord{
		{ID: CapabilityAPIClients, Project: "web/store", Language: "javascript", Coverage: CoveragePartial, SourceClass: "code", Reason: "Supported static patterns emit file-and-line-backed facts."},
		{ID: CapabilityAPIClients, Project: "web/store", Language: "json", Coverage: CoverageUnavailable, SourceClass: "configuration", ExpectedUnavailable: true, Reason: "The active analyzer does not emit this capability yet."},
		{ID: CapabilityRelations, Project: "web/store", Language: "json", Coverage: CoverageUnavailable, SourceClass: "configuration", ExpectedUnavailable: true},
		{ID: CapabilityCalls, Project: "web/store", Language: "javascript", Coverage: CoverageFailed},
		{ID: CapabilityAPIClients, Project: "web/store", Language: "go", Coverage: CoverageUnavailable, SourceClass: "code"},
	}
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion:      SchemaVersion,
		Nodes:              []WorkspaceServiceNodeRecord{{ID: "service:web", Label: "Storefront", Project: "web/store", Indexed: true}},
		Capabilities:       capabilities,
		Health:             ProjectionHealth{Integrity: "valid", Freshness: "stale", Coverage: "partial"},
		DiagnosticFamilies: []DiagnosticFamilyRecord{{Code: contractIssueMethodMismatch, Service: "web/store", RoutePattern: "/orders", AffectedCount: 1, RootCause: "A related backend route exists, but its HTTP method does not match the client contract.", SuggestedCheck: "Compare the client method with the backend route."}},
	}
	traces := WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []WorkspaceEndpointTraceRecord{{ID: "trace:orders", FromProject: "web/store", Method: "POST", Path: "/orders", Status: "MISMATCH"}}}
	usages := WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion, Coverage: []SymbolCoverageRecord{{Project: "web/store", Language: "json", Capability: "direct_usages", Coverage: CoverageUnavailable}}}
	return RenderWorkspaceDashboardHTMLWithCodeExplorer(WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/synthetic"}, serviceMap, traces, WorkspaceSymbolIndexRecord{}, usages)
}
