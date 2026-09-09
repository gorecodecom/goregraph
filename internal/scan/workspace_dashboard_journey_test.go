package scan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDashboardExposesHealthAndJourneyNavigation(t *testing.T) {
	html := RenderWorkspaceDashboardHTMLWithModels(
		WorkspaceGraphRecord{SchemaVersion: SchemaVersion},
		WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion},
		WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, nil, nil,
	)
	for _, want := range []string{
		`id="workspace-health"`, `id="health-integrity"`, `id="health-freshness"`,
		`id="health-coverage"`, `id="journey-back"`, "function navigateJourney(",
		"function returnFromJourney(", "function renderDashboardHealth(",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing journey/health contract %q", want)
		}
	}
}

func TestWorkspaceDashboardCompletesSyntheticInvestigationJourneys(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered dashboard journey tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed for rendered dashboard journey tests: %s", strings.TrimSpace(string(output)))
	}

	flow := BuildCanonicalFeatureFlow(WorkspaceFeatureFlowRecord{
		ID: "flow:orders", FrontendProject: "web/store", FrontendCaller: "ordersApi.cancel",
		FrontendFile: "src/ordersApi.ts", FrontendLine: 18, HTTPMethod: "DELETE", Path: "/api/orders/{id}",
		BackendProject: "services/orders", BackendService: "orders", BackendController: "OrderController",
		BackendMethod: "cancel", BackendFile: "src/OrderController.java", BackendLine: 42,
		Confidence: "RESOLVED", Reason: "matched consumer contract to provider route",
		Tests:                []TestMapRecord{{TestFile: "src/OrderControllerTest.java", TestMethod: "cancelPublishesAfterCommit", Confidence: "EXACT"}},
		TestLinks:            []TestLinkRecord{{ID: "test-link:cancel", Relation: "direct", TestFile: "src/OrderControllerTest.java", TestName: "cancelPublishesAfterCommit", Confidence: "EXACT", Reason: "calls the selected endpoint"}},
		VerificationCommands: []VerificationCommandRecord{{Tool: "maven", WorkingDirectory: "services/orders", Args: []string{"-Dtest=OrderControllerTest#cancelPublishesAfterCommit", "test"}, Display: "mvn -Dtest=OrderControllerTest#cancelPublishesAfterCommit test", Confidence: "EXACT", Reason: "detected Maven test"}},
	})
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion, Generated: "2026-09-09T09:00:00Z",
		Health: ProjectionHealth{GenerationID: "generation-a", Integrity: "valid", Freshness: "stale", Coverage: "partial", Reasons: []string{"projection_inputs_changed", "analysis_incomplete"}},
		Nodes: []WorkspaceServiceNodeRecord{
			{ID: "service:web", Label: "Storefront", Project: "web/store", Indexed: true},
			{ID: "service:orders", Label: "Orders", Project: "services/orders", Indexed: true},
			{ID: "service:orders-legacy", Label: "Legacy Orders candidate", Project: "services/orders-legacy", Indexed: false},
		},
		WorkspaceCoverage: WorkspaceCoverageSummaryRecord{KnownProjects: 3, IndexedProjects: 2, ReferencedServices: 2, IndexedReferencedServices: 1, NextScans: []NextScanRecord{{Service: "orders-legacy", Project: "services/orders-legacy", AffectedContracts: 1, Command: `goregraph scan "services/orders-legacy"`, Reason: "Referenced provider candidate is not indexed."}}},
		FeatureFlows:      []WorkspaceFeatureFlowRecord{flow},
		ImpactSummaries: []ImpactSummaryRecord{{
			ID: "impact:orders", TargetID: "flow:orders", TargetLabel: "DELETE /api/orders/{id}", RiskLevel: "medium",
			RiskReasons:         []string{"Public mutation with one direct consumer."},
			DirectConsumers:     []ImpactItemRecord{{ID: "consumer:web", Relationship: "direct_consumer", Kind: "api_call", Project: "web/store", Symbol: "ordersApi.cancel", Confidence: "RESOLVED", Reason: "matched contract"}},
			DependentTests:      []ImpactItemRecord{{ID: "test:cancel", Relationship: "test", Kind: "test", Project: "services/orders", Symbol: "cancelPublishesAfterCommit", Confidence: "EXACT", Reason: "calls endpoint"}},
			CoverageUncertainty: []string{"Two provider candidates matched this route; the legacy candidate is not indexed.", "One generated client bundle was excluded from analysis."},
		}},
	}
	traces := WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion, Traces: []WorkspaceEndpointTraceRecord{
		{
			ID: "trace:orders", Route: "DELETE /api/orders/{id}", FromProject: "web/store", ToProject: "services/orders", Status: "RESOLVED",
			Steps: []WorkspaceEndpointTraceStepRecord{
				{ID: "step:client", Kind: "api_call", Label: "ordersApi.cancel", Project: "web/store", File: "src/ordersApi.ts", Line: 18},
				{ID: "step:handler", Kind: "controller", Label: "OrderController.cancel", Project: "services/orders", File: "src/OrderController.java", Line: 42},
			},
		},
		{ID: "trace:orders-legacy", Route: "DELETE /api/orders/{id}", FromProject: "web/store", ToProject: "services/orders-legacy", Status: "UNRESOLVED", Risk: "ambiguous provider candidate", Steps: []WorkspaceEndpointTraceStepRecord{{ID: "step:legacy-client", Kind: "api_call", Label: "ordersApi.cancel", Project: "web/store", File: "src/ordersApi.ts", Line: 18}}},
	}}
	html := RenderWorkspaceDashboardHTMLWithModels(WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/synthetic"}, serviceMap, traces, nil, nil)
	encodedHTML, err := json.Marshal(html)
	if err != nil {
		t.Fatal(err)
	}
	evidenceDir := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR")
	if evidenceDir != "" {
		evidenceDir = filepath.Clean(evidenceDir)
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	encodedEvidenceDir, err := json.Marshal(evidenceDir)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Join([]string{
		`const {chromium}=require("playwright"),path=require("path"),html=` + string(encodedHTML) + `,evidenceDir=` + string(encodedEvidenceDir) + `,launchOptions={headless:true};if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)launchOptions.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;`,
		`(async()=>{const browser=await chromium.launch(launchOptions),errors=[];try{const page=await browser.newPage({viewport:{width:1440,height:900}});page.on("console",message=>{if(message.type()==="error")errors.push(message.text());});await page.setContent(html,{waitUntil:"load"});const health=await page.locator("#workspace-health").innerText();await page.locator('[data-view-mode="feature-flow"]').focus();await page.keyboard.press("Enter");await page.locator("#workspace-search").fill("orders");await page.locator('[data-kind-filter="resolved"]').click();await page.locator('[data-select-id="flow:orders"]').focus();await page.keyboard.press("Enter");const feature=await page.locator("#workspace-workbench").innerText();if(evidenceDir)await page.screenshot({path:path.join(evidenceDir,"dashboard-desktop-feature.png"),fullPage:true});await page.locator('[data-feature-endpoint="trace:orders"]').focus();await page.keyboard.press("Enter");const endpoint=await page.locator("#details").innerText();const targetMode=await page.locator("main").getAttribute("data-active-view");const backFocused=await page.locator("#journey-back").evaluate(element=>document.activeElement===element);if(evidenceDir)await page.screenshot({path:path.join(evidenceDir,"dashboard-endpoint-journey-back.png"),fullPage:true});await page.keyboard.press("Enter");const returnedMode=await page.locator("main").getAttribute("data-active-view");const returned=await page.locator("#workspace-workbench").innerText();const returnedFocus=await page.locator('[data-feature-endpoint="trace:orders"]').evaluate(element=>document.activeElement===element);const returnedQuery=await page.locator("#workspace-search").inputValue();const returnedFilter=await page.locator('[data-kind-filter="resolved"]').getAttribute("aria-pressed");await page.setViewportSize({width:900,height:800});if(evidenceDir)await page.screenshot({path:path.join(evidenceDir,"dashboard-narrow-feature.png"),fullPage:true});await page.evaluate(()=>{document.body.style.zoom="200%";});const zoomVisible=await page.locator("#workspace-health").isVisible();if(evidenceDir)await page.screenshot({path:path.join(evidenceDir,"dashboard-200-percent.png"),fullPage:true});process.stdout.write(JSON.stringify({health,feature,endpoint,targetMode,backFocused,returnedMode,returned,returnedFocus,returnedQuery,returnedFilter,zoomVisible,errors}));}finally{await browser.close();}})().catch(error=>{console.error(error);process.exit(1);});`,
	}, "\n")
	output, err := nodeScriptCommand(node, source).CombinedOutput()
	if err != nil {
		t.Fatalf("rendered dashboard journeys failed: %v\n%s", err, output)
	}
	var result struct {
		Health, Feature, Endpoint, TargetMode, ReturnedMode, Returned string
		ReturnedQuery, ReturnedFilter                                 string
		BackFocused, ReturnedFocus, ZoomVisible                       bool
		Errors                                                        []string
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode rendered journey result: %v\n%s", err, output)
	}
	for _, evidence := range []struct{ body, want string }{
		{result.Health, "FRESHNESS\nstale"}, {result.Health, "COVERAGE\npartial"},
		{result.Feature, "OrderControllerTest#cancelPublishesAfterCommit"},
		{result.Feature, "Two provider candidates matched this route"},
		{result.Feature, "One generated client bundle was excluded"},
		{result.Endpoint, "OrderController.cancel"}, {result.Endpoint, "src/OrderController.java"},
		{result.Returned, "mvn -Dtest=OrderControllerTest#cancelPublishesAfterCommit test"},
	} {
		if !strings.Contains(evidence.body, evidence.want) {
			t.Fatalf("rendered evidence missing %q in %q", evidence.want, evidence.body)
		}
	}
	if result.TargetMode != "endpoints" || result.ReturnedMode != "feature-flow" || result.ReturnedQuery != "orders" || result.ReturnedFilter != "true" || !result.BackFocused || !result.ReturnedFocus || !result.ZoomVisible || len(result.Errors) != 0 {
		t.Fatalf("journey state = %#v", result)
	}
}

func TestWorkspaceDashboardKeepsOldOfflineTabsAndExplainsMissingShard(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered dashboard asset lifecycle tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed for rendered dashboard asset lifecycle tests: %s", strings.TrimSpace(string(output)))
	}

	graph := WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/synthetic"}
	serviceMap := WorkspaceServiceMapRecord{
		SchemaVersion: SchemaVersion,
		Health:        ProjectionHealth{GenerationID: "generation-old", Integrity: "valid", Freshness: "current", Coverage: "complete"},
		Nodes:         []WorkspaceServiceNodeRecord{{ID: "service:orders", Label: "Orders", Project: "services/orders", Indexed: true}},
	}
	symbols := WorkspaceSymbolIndexRecord{SchemaVersion: SchemaVersion, Symbols: []CanonicalSymbolRecord{{
		ID: "symbol:orders", Project: "services/orders", Language: "java", Kind: "class", Name: "OrderService", QualifiedName: "example.OrderService",
		DeclarationFile: "src/OrderService.java", DeclarationLine: 10, Analyzer: "java", Confidence: ConfidenceExact, Coverage: CoverageComplete,
	}}}
	usage := func(generated, id, source string) WorkspaceSymbolUsageIndexRecord {
		return WorkspaceSymbolUsageIndexRecord{SchemaVersion: SchemaVersion, Generated: generated, Usages: []CanonicalSymbolUsageRecord{{
			ID: id, ProviderSymbolID: "symbol:orders", ConsumerProject: "services/orders", ConsumerSymbolID: "symbol:orders", Category: SymbolUsageDirectReference,
			Language: "java", RelationKind: "calls", SourceFile: source, SourceLine: 21, Confidence: ConfidenceExact, Resolution: SymbolResolutionExact, Reason: "synthetic direct call", Analyzer: "java",
		}}}
	}
	oldArtifacts := buildWorkspaceDashboardArtifacts(graph, serviceMap, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, APICatalogRecord{SchemaVersion: SchemaVersion}, symbols, usage("old", "usage:old", "src/OldOrderConsumer.java"))
	newArtifacts := buildWorkspaceDashboardArtifacts(graph, serviceMap, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, APICatalogRecord{SchemaVersion: SchemaVersion}, symbols, usage("new", "usage:new", "src/NewOrderConsumer.java"))
	if oldArtifacts.AssetByProject["services/orders"] == newArtifacts.AssetByProject["services/orders"] {
		t.Fatal("synthetic rebuild did not produce a new immutable shard identity")
	}
	type browserArtifacts struct {
		HTML      string            `json:"html"`
		Assets    map[string]string `json:"assets"`
		ByProject map[string]string `json:"byProject"`
	}
	convert := func(artifacts workspaceDashboardArtifacts) browserArtifacts {
		assets := make(map[string]string, len(artifacts.Assets))
		for name, data := range artifacts.Assets {
			assets[name] = string(data)
		}
		return browserArtifacts{HTML: artifacts.HTML, Assets: assets, ByProject: artifacts.AssetByProject}
	}
	oldJSON, err := json.Marshal(convert(oldArtifacts))
	if err != nil {
		t.Fatal(err)
	}
	newJSON, err := json.Marshal(convert(newArtifacts))
	if err != nil {
		t.Fatal(err)
	}
	tempDirJSON, err := json.Marshal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceDir := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR")
	if evidenceDir != "" {
		evidenceDir = filepath.Clean(evidenceDir)
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	evidenceDirJSON, err := json.Marshal(evidenceDir)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Join([]string{
		`const {chromium}=require("playwright"),fs=require("fs"),path=require("path"),{pathToFileURL}=require("url"),root=` + string(tempDirJSON) + `,evidenceDir=` + string(evidenceDirJSON) + `,oldArtifacts=` + string(oldJSON) + `,newArtifacts=` + string(newJSON) + `,launchOptions={headless:true};if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)launchOptions.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;`,
		`function publish(artifacts){fs.writeFileSync(path.join(root,"workspace-map.html"),artifacts.html);for(const [name,content] of Object.entries(artifacts.assets)){const target=path.join(root,name);fs.mkdirSync(path.dirname(target),{recursive:true});fs.writeFileSync(target,content);}}`,
		`async function openCodeExplorer(page){await page.locator('[data-view-mode="code-explorer"]').focus();await page.keyboard.press("Enter");await page.locator('.code-project-row[data-code-project="services/orders"]').focus();await page.keyboard.press("Enter");}`,
		`(async()=>{publish(oldArtifacts);const browser=await chromium.launch(launchOptions);try{const dashboardURL=pathToFileURL(path.join(root,"workspace-map.html")).href,oldErrors=[],oldPage=await browser.newPage({viewport:{width:1440,height:900}});oldPage.on("console",message=>{if(message.type()==="error")oldErrors.push(message.text());});await oldPage.route(/workspace-map-assets.*\.js$/,async route=>{await new Promise(resolve=>setTimeout(resolve,180));await route.continue();});await oldPage.goto(dashboardURL,{waitUntil:"load"});publish(newArtifacts);await openCodeExplorer(oldPage);const loadingObserved=(await oldPage.locator("#workspace-workbench").innerText()).includes("Loading symbol usage evidence");await oldPage.waitForFunction(()=>document.querySelector("#workspace-workbench").innerText.includes("OldOrderConsumer.java"));const oldTabText=await oldPage.locator("#workspace-workbench").innerText();if(evidenceDir)await oldPage.screenshot({path:path.join(evidenceDir,"dashboard-old-tab-retained-shard.png"),fullPage:true});const oldAssetRetained=fs.existsSync(path.join(root,oldArtifacts.byProject["services/orders"]));const newPage=await browser.newPage({viewport:{width:1100,height:800}});await newPage.goto(dashboardURL,{waitUntil:"load"});fs.rmSync(path.join(root,newArtifacts.byProject["services/orders"]));await openCodeExplorer(newPage);await newPage.waitForFunction(()=>document.querySelector("#workspace-workbench").innerText.includes("Usage evidence unavailable"));const missingText=await newPage.locator("#workspace-workbench").innerText();if(evidenceDir)await newPage.screenshot({path:path.join(evidenceDir,"dashboard-missing-shard.png"),fullPage:true});process.stdout.write(JSON.stringify({offline:oldPage.url().startsWith("file:"),loadingObserved,oldAssetRetained,oldTabText,missingText,oldErrors}));}finally{await browser.close();}})().catch(error=>{console.error(error);process.exit(1);});`,
	}, "\n")
	output, err := nodeScriptCommand(node, source).CombinedOutput()
	if err != nil {
		t.Fatalf("rendered dashboard asset lifecycle failed: %v\n%s", err, output)
	}
	var result struct {
		Offline, LoadingObserved, OldAssetRetained bool
		OldTabText, MissingText                    string
		OldErrors                                  []string
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode rendered asset lifecycle result: %v\n%s", err, output)
	}
	if !result.Offline || !result.LoadingObserved || !result.OldAssetRetained || len(result.OldErrors) != 0 || !strings.Contains(result.OldTabText, "OldOrderConsumer.java") || !strings.Contains(result.MissingText, "Return and regenerate the workspace dashboard") || strings.Contains(result.MissingText, "No verified projection health") {
		t.Fatalf("offline asset lifecycle = %#v", result)
	}
}
