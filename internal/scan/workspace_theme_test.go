package scan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDashboardAppliesThemeBeforeStyles(t *testing.T) {
	html := workspaceQualityHTML()
	theme := strings.Index(html, "goregraph.dashboard.theme")
	styles := strings.Index(html, `<style id="workspace-modern-styles">`)
	if theme < 0 || styles < 0 || theme > styles {
		t.Fatal("saved theme must be applied before the dashboard styles load")
	}
}

func TestWorkspaceDashboardThemeInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for rendered dashboard theme tests")
	}
	if output, err := exec.Command(node, "-e", `require.resolve("playwright")`).CombinedOutput(); err != nil {
		t.Skipf("Playwright is not installed: %s", output)
	}
	previewDir := t.TempDir()
	if evidenceDir := os.Getenv("GOREGRAPH_DASHBOARD_EVIDENCE_DIR"); evidenceDir != "" {
		previewDir = evidenceDir
	}
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(previewDir, "theme-preview.html")
	if err := os.WriteFile(preview, []byte(workspaceQualityHTML()), 0o644); err != nil {
		t.Fatal(err)
	}
	encodedPreview, _ := json.Marshal(preview)
	encodedDir, _ := json.Marshal(previewDir)
	source := `const {chromium}=require('playwright'),{pathToFileURL}=require('url'),path=require('path');
const preview=` + string(encodedPreview) + `,evidenceDir=` + string(encodedDir) + `;
function assert(condition,message){if(!condition)throw new Error(message);}
async function assertTheme(page,theme){await page.waitForFunction(expected=>document.documentElement.dataset.theme===expected,theme);}
async function assertSelection(control,name){
  assert(await control.getByRole('button',{name,exact:true}).getAttribute('aria-pressed')==='true','theme button selection is incorrect: '+name);
  assert(await control.locator('[aria-pressed="true"]').count()===1,'theme selection must be exclusive');
}
async function contrast(page,selector){return page.locator(selector).first().evaluate(element=>{
  const luminance=color=>{const channels=color.match(/[\d.]+/g).slice(0,3).map(Number).map(value=>{value/=255;return value<=.04045?value/12.92:((value+.055)/1.055)**2.4;});return channels[0]*.2126+channels[1]*.7152+channels[2]*.0722;};
  const style=getComputedStyle(element),front=luminance(style.color),back=luminance(style.backgroundColor);return (Math.max(front,back)+.05)/(Math.min(front,back)+.05);
});}
(async()=>{
  const options={headless:true};if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)options.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;
  const browser=await chromium.launch(options);
  try {
    const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
    page.on('pageerror',error=>errors.push(error.message));
    page.on('console',message=>{if(message.type()==='error')errors.push(message.text());});
    await page.emulateMedia({colorScheme:'dark'});
    const url=pathToFileURL(preview).href+'#quality';
    await page.goto(url);
    await page.getByRole('heading',{name:'Datenqualität',exact:true}).waitFor();
    const control=page.getByRole('group',{name:'Darstellung',exact:true});
    const light=control.getByRole('button',{name:'Hell',exact:true}),system=control.getByRole('button',{name:'System',exact:true}),dark=control.getByRole('button',{name:'Dunkel',exact:true});
    assert((await control.getByRole('button').all()).length===3,'theme choices must be three buttons');
    assert((await control.getByRole('button').allTextContents()).every(text=>text.trim()===''),'theme buttons must use icons');
    for(const button of [light,system,dark])assert(await button.getAttribute('title'),'icon button has no tooltip');
    await assertTheme(page,'light');
    await assertSelection(control,'Hell');
    await dark.focus();
    assert(await dark.evaluate(element=>getComputedStyle(element).outlineStyle)!=='none','theme button has no keyboard focus indicator');
    await dark.press('Enter');
    await assertTheme(page,'dark');
    await assertSelection(control,'Dunkel');
    assert(await page.evaluate(()=>localStorage.getItem('goregraph.dashboard.theme'))==='dark','explicit dark preference was not saved');
    for(const selector of ['.quality-card','.sidebar','.quality-banner','.module-table-wrap'])assert(await page.locator(selector).first().evaluate(element=>getComputedStyle(element).backgroundColor)!=='rgb(255, 255, 255)','white surface remained: '+selector);
    for(const selector of ['.quality-card','.pill.warn','.pill.neutral'])assert(await contrast(page,selector)>=4.5,'insufficient dark text contrast: '+selector);
    assert((await page.locator('#content').innerText()).includes('Für Konfigurationsdateien'),'German explanations missing from dark mode');
    await page.screenshot({path:path.join(evidenceDir,'theme-quality-dark-desktop.png'),fullPage:true});
    await page.reload();await assertTheme(page,'dark');await assertSelection(control,'Dunkel');
    for(const area of ['api','code','architecture']){
      await page.locator('[data-area="'+area+'"]').first().click();
      await assertTheme(page,'dark');
      await assertSelection(control,'Dunkel');
    }
    await page.locator('#service-list').getByRole('button',{name:'Storefront',exact:true}).click();
    await page.locator('.diagram').waitFor();
    assert(await page.locator('.node-surface').first().evaluate(element=>getComputedStyle(element).fill)!=='rgb(255, 255, 255)','SVG nodes retained light surfaces');
    assert(await page.locator('.diagram text').first().evaluate(element=>getComputedStyle(element).fill)!=='rgb(23, 44, 60)','SVG labels retained dark foregrounds');
    await page.screenshot({path:path.join(evidenceDir,'theme-architecture-dark.png'),fullPage:true});
    await page.locator('[data-area="quality"]').click();
    await page.setViewportSize({width:390,height:844});
    assert(await control.isVisible(),'theme control is hidden on narrow screens');
    for(const button of [light,system,dark]){
      const bounds=await button.boundingBox();
      assert(bounds&&bounds.width>=44&&bounds.height>=44&&bounds.x>=0&&bounds.x+bounds.width<=390,'theme button is too small or outside the narrow viewport');
    }
    assert(!await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),'dark dashboard overflows narrow viewport');
    await page.screenshot({path:path.join(evidenceDir,'theme-quality-dark-mobile.png'),fullPage:true});
    await system.click();await assertTheme(page,'dark');await assertSelection(control,'System');
    await page.emulateMedia({colorScheme:'light'});await assertTheme(page,'light');
    await page.reload();await assertTheme(page,'light');await assertSelection(control,'System');
    await page.emulateMedia({colorScheme:'dark'});await assertTheme(page,'dark');
    await light.focus();await light.press('Space');await assertTheme(page,'light');await assertSelection(control,'Hell');
    await page.evaluate(()=>window.dispatchEvent(new StorageEvent('storage',{key:'goregraph.dashboard.theme',newValue:'dark'})));
    await assertTheme(page,'dark');await assertSelection(control,'Dunkel');
    await page.evaluate(()=>window.dispatchEvent(new StorageEvent('storage',{key:'goregraph.dashboard.theme',newValue:'invalid'})));
    await assertTheme(page,'light');await assertSelection(control,'Hell');
    await page.setViewportSize({width:1440,height:1000});
    await page.screenshot({path:path.join(evidenceDir,'theme-quality-light-desktop.png'),fullPage:true});
    assert(await page.locator('.quality-card').first().evaluate(element=>getComputedStyle(element).backgroundColor)==='rgb(255, 255, 255)','light mode did not restore existing surfaces');
    const blocked=await browser.newPage();blocked.on('pageerror',error=>errors.push(error.message));
    await blocked.addInitScript(()=>Object.defineProperty(window,'localStorage',{get(){throw new DOMException('Storage blocked','SecurityError');}}));
    await blocked.goto(url);await blocked.getByRole('group',{name:'Darstellung',exact:true}).getByRole('button',{name:'Dunkel',exact:true}).click();await assertTheme(blocked,'dark');
    assert(errors.length===0,'browser errors: '+errors.join('; '));
    console.log('Theme icon buttons, keyboard access, selection, persistence, system changes, storage events, unavailable storage, all areas, SVG colors, text contrast and narrow viewport passed');
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});`
	if output, err := nodeScriptCommand(node, source).CombinedOutput(); err != nil {
		t.Fatalf("rendered theme check failed: %v\n%s", err, output)
	}
}
