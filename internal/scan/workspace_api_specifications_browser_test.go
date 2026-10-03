package scan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkspaceAPISpecificationQualityInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered contract quality tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed: %s", output)
	}
	record, _ := parseAPISpecification("documentation/API/swagger.yaml", []byte(swaggerSpecificationFixture))
	record.Repository = "documentation"
	jsonRecord, _ := parseAPISpecification("documentation/openapi.json", []byte(`{"openapi":"3.1.0","paths":{"/metadata":{"get":{"responses":{}}}}}`))
	jsonRecord.Repository = "documentation"
	prefixRecord, _ := parseAPISpecification("documentation/swagger-vd.yaml", []byte("swagger: '2.0'\nbasePath: /api/1.0\npaths:\n  /cadasters/{cadasterId}:\n    get: {responses: {}}\n"))
	prefixRecord.Repository = "documentation"
	index := APISpecificationIndexRecord{SchemaVersion: SchemaVersion, Enabled: true, InventoryComplete: true, Documents: []APISpecificationRecord{record, {File: "documentation/openapi-broken.yaml", Repository: "documentation", Status: "invalid", Limitations: []string{"invalid_document"}}, jsonRecord, prefixRecord}}
	linkWorkspaceAPISpecifications(&index, []workspaceIndexProject{
		{record: WorkspaceProjectRecord{Path: "services/orders"}, routes: []CodeRouteRecord{{Kind: "backend", HTTPMethod: "GET", Path: "/api/orders/{id}", File: "src/Orders.java", Line: 12}}},
		{record: WorkspaceProjectRecord{Path: "services/cadaster"}, endpoints: []SpringEndpointRecord{{HTTPMethod: "GET", Path: "/cadasters/{id}", File: "src/Cadaster.java", Line: 42}}},
	})
	serviceMap := WorkspaceServiceMapRecord{SchemaVersion: SchemaVersion, APISpecifications: &index, Nodes: []WorkspaceServiceNodeRecord{
		{ID: "service:web", Label: "Storefront", Project: "frontend/web", Indexed: true},
		{ID: "service:orders", Label: "Orders", Project: "services/orders", Indexed: true},
		{ID: "service:cadaster", Label: "Cadaster", Project: "services/cadaster", Indexed: true},
	}}
	html := RenderWorkspaceDashboardHTMLWithCodeExplorer(WorkspaceGraphRecord{SchemaVersion: SchemaVersion, Root: "/synthetic"}, serviceMap, WorkspaceEndpointTraceIndexRecord{SchemaVersion: SchemaVersion}, WorkspaceSymbolIndexRecord{}, WorkspaceSymbolUsageIndexRecord{})
	directory := t.TempDir()
	if evidence := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR"); evidence != "" {
		directory = evidence
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(directory, "api-specifications-preview.html")
	if err := os.WriteFile(preview, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	encodedPreview, _ := json.Marshal(preview)
	encodedDirectory, _ := json.Marshal(directory)
	source := `const {chromium}=require('playwright'),{pathToFileURL}=require('url'),path=require('path');
(async()=>{const options={headless:true};if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)options.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;
const browser=await chromium.launch(options);try{const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
page.on('pageerror',e=>errors.push(e.message));await page.goto(pathToFileURL(` + string(encodedPreview) + `).href+'#quality');
const panel=page.locator('.api-specifications');await panel.getByRole('heading',{name:'Swagger- und OpenAPI-Verträge'}).waitFor();
await panel.getByRole('button',{name:'Verträge im gesamten Workspace anzeigen'}).click();
const coverage=page.locator('.quality-capability-table');
for(const [language,expected] of [['json','Ausgewertet'],['yaml','Teilweise ausgewertet']]){const row=coverage.locator('[data-specification-language="'+language+'"]');if(!(await row.innerText()).includes(expected))throw new Error('Contract coverage missing for '+language);}
await coverage.locator('[data-specification-language="json"] button').click();
if(await panel.locator('[data-specification-file="documentation/openapi.json"]').getAttribute('open')===null)throw new Error('Coverage detail navigation failed');
await panel.locator('.api-specification-document').first().locator('summary').first().click();
await panel.locator('.api-specification-model summary').click();
const text=await panel.innerText();for(const expected of ['Repository: documentation','GET','/orders/{orderId}','Mit Server-Präfix:','/api/orders/{orderId}','Passender Codebeleg','src/Orders.java:12','Order','Pflichtfeld','direkt deklarierte Felder','Analyse fehlgeschlagen'])if(!text.includes(expected))throw new Error('Missing evidence: '+expected);
if(text.includes('NEVER_EXPORT_EXAMPLE'))throw new Error('Schema example leaked');
await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(` + string(encodedDirectory) + `,'api-specifications-desktop.png'),fullPage:true});
await panel.getByRole('button',{name:'Schnittstellen im Service öffnen'}).first().click();
await page.getByRole('heading',{name:'Schnittstellen',exact:true}).waitFor();
if(!(await page.locator('.context-bar').innerText()).includes('services/orders'))throw new Error('Wrong service navigation');
await page.locator('[data-area="quality"]').click();if(await panel.locator('.api-specification-document').count()!==1)throw new Error('Service context did not filter unrelated document');
await panel.getByRole('button',{name:'Verträge im gesamten Workspace anzeigen'}).click();
await panel.locator('.api-specification-document').first().locator('summary').first().click();
await panel.locator('.api-specification-document').nth(1).locator('summary').first().click();
await panel.locator('.api-specification-document').nth(3).locator('summary').first().click();
if(!(await panel.innerText()).includes('Passender Service-Pfad · Server-Präfix ungeprüft'))throw new Error('Prefix candidate claimed a complete route match');
await panel.locator('.api-specification-document').nth(3).getByRole('button',{name:'Schnittstellen im Service öffnen'}).click();
if(!(await page.locator('.context-bar').innerText()).includes('services/cadaster'))throw new Error('Prefix candidate did not navigate to its service');
await page.locator('[data-area="quality"]').click();
if(await panel.locator('.api-specification-document').count()!==1)throw new Error('Prefix candidate was hidden in service context');
if(!(await coverage.locator('[data-specification-language="yaml"]').innerText()).includes('1 Vertragsdatei im Kontext'))throw new Error('Service coverage lost its prefix candidate');
await panel.getByRole('button',{name:'Verträge im gesamten Workspace anzeigen'}).click();
await panel.locator('.api-specification-document').first().locator('summary').first().click();
await panel.locator('.api-specification-document').nth(1).locator('summary').first().click();
await panel.locator('.api-specification-document').nth(3).locator('summary').first().click();
await page.getByRole('button',{name:'Dunkel',exact:true}).click();
await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(` + string(encodedDirectory) + `,'api-specifications-dark.png'),fullPage:true});
await page.setViewportSize({width:390,height:844});await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(` + string(encodedDirectory) + `,'api-specifications-mobile.png'),fullPage:true});
if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw new Error('Contract panel overflows narrow viewport');
await panel.locator('summary').first().focus();await page.keyboard.press('Enter');if(await panel.locator('.api-specification-document').first().getAttribute('open')!==null)throw new Error('Keyboard could not collapse document');
if(errors.length)throw new Error('Browser errors: '+errors.join('; '));console.log('Contract evidence, German explanations, service navigation, dark theme, narrow viewport and keyboard passed');
await page.evaluate(()=>{window.WORKSPACE_DATA.apiSpecifications.inventory_complete=false;window.WORKSPACE_DATA.apiSpecifications.issues=['inventory_unavailable'];window.renderWorkspaceArea();});
if(!(await panel.innerText()).includes('Fehlende Einträge gelten nicht als belegte Löschungen'))throw new Error('Incomplete inventory hid its limitations');
await page.evaluate(()=>{window.WORKSPACE_DATA.apiSpecifications.documents=window.WORKSPACE_DATA.apiSpecifications.documents.filter(document=>!document.file.endsWith('.json'));window.renderWorkspaceArea();});
if(!(await coverage.locator('[data-specification-language="json"]').innerText()).includes('Unterstützt · Erfassung unvollständig'))throw new Error('Incomplete inventory claimed JSON absence');
await page.evaluate(()=>{window.WORKSPACE_DATA.apiSpecifications.enabled=false;window.renderWorkspaceArea();});
if(!(await coverage.locator('[data-specification-language="yaml"]').innerText()).includes('Ausgeschaltet'))throw new Error('Disabled supplement claimed analyzed coverage');
await page.evaluate(()=>{delete window.WORKSPACE_DATA.apiSpecifications;window.renderWorkspaceArea();});
if(!(await coverage.locator('[data-specification-language="json"]').innerText()).includes('Noch nicht im Export'))throw new Error('Older export claimed contract coverage');
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exit(1);});`
	if output, err := nodeScriptCommand(node, source).CombinedOutput(); err != nil {
		t.Fatalf("rendered contract check failed: %v\n%s", err, output)
	}
}
