const {chromium}=require('playwright');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {pathToFileURL}=require('node:url');
(async()=>{
  const root=process.argv[2],options={headless:true};
  if(process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM)options.executablePath=process.env.GOREGRAPH_PLAYWRIGHT_CHROMIUM;
  const browser=await chromium.launch(options);
  const errors=[];
  try{
    const page=await browser.newPage({viewport:{width:1440,height:1000}});
    page.on('pageerror',error=>errors.push(error.message));
    const url=pathToFileURL(path.join(root,'workspace-map.html')).href;
    await page.goto(url+'#code');await page.reload();
    await page.waitForFunction(()=>document.querySelector('.reference-sites li'));
    assert.equal(await page.locator('.reference-sites li').count(),2);
    assert.equal(await page.locator('.usage-card').count(),1);
    await page.getByRole('button',{name:'OrderConsumer',exact:true}).click();
    await page.getByRole('button',{name:/^Verwendet selbst/}).click();
    await page.waitForFunction(()=>document.querySelector('.reference-sites li'));
    assert.equal(await page.locator('.reference-sites li').count(),2,'cross-project outgoing references load from provider shard');
    await page.getByRole('button',{name:'Architektur',exact:true}).click();
    assert.equal(await page.locator('h1').innerText(),'Storefront');
    for(const area of ['Schnittstellen','Datenqualität','Service-Code']){
      await page.getByRole('button',{name:area,exact:true}).click();
      assert.equal(await page.locator('h1').innerText(),area);
    }
    await page.getByRole('button',{name:'Tests & Tooling',exact:true}).click();
    assert.equal(await page.locator('.tooling-workbench .symbol-row').count(),13);
    assert.equal(await page.getByText('Statischer Snapshot',{exact:true}).isVisible(),true);
    await page.locator('#tooling-group').selectOption('stories');
    await page.locator('#tooling-search').fill('ProductCard.stories');
    assert.equal(await page.locator('.tooling-workbench .symbol-row').count(),1);
    await page.locator('.tooling-workbench .symbol-row').click();
    assert.equal(await page.locator('#tooling-search').inputValue(),'ProductCard.stories');
    assert.equal(await page.locator('#tooling-group').inputValue(),'stories');
    assert.equal(await page.locator('.tooling-detail [data-tooling-file="src/productFixture.ts"]').count(),1);
    await page.locator('.tooling-detail [data-tooling-file="src/ProductCard.tsx"]').click();
    await page.getByRole('button',{name:'ProductCard',exact:true}).click();
    assert.equal(await page.locator('[data-code-view="symbols"]').getAttribute('aria-pressed'),'true');
    await page.getByRole('button',{name:'Tests & Tooling',exact:true}).click();
    await page.locator('.symbol-list [data-tooling-file=".storybook/preview.ts"]').click();
    assert.equal(await page.locator('.tooling-observations').getByText('A11y: test = off deklariert',{exact:true}).isVisible(),true);
    await page.locator('[data-code-view="symbols"]').focus();await page.keyboard.press('Enter');
    assert.equal(await page.locator('[data-code-view="symbols"]').getAttribute('aria-pressed'),'true');
    await page.getByRole('button',{name:'Tests & Tooling',exact:true}).click();
    for(const width of [320,768,1024,1440]){
      await page.setViewportSize({width,height:1000});
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'tooling overflow at '+width);
    }
    if(process.env.GOREGRAPH_DASHBOARD_SCREENSHOT_DIR){fs.mkdirSync(process.env.GOREGRAPH_DASHBOARD_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.GOREGRAPH_DASHBOARD_SCREENSHOT_DIR,'tooling-desktop.png'),fullPage:true});}
    await page.locator('.nav-group').filter({has:page.locator('[data-node="a"]')}).locator('summary').click();
    await page.locator('.sidebar [data-node="a"]').click();
    assert.equal(await page.getByText('Keine passenden Tooling-Quellen im Index',{exact:true}).isVisible(),true);
    await page.locator('.nav-group').filter({has:page.locator('[data-node="b"]')}).locator('summary').click();
    await page.locator('.sidebar [data-node="b"]').click();
    assert.equal(await page.locator('.tooling-workbench .symbol-row').count(),13);
    await page.getByRole('button',{name:'Klassen & Verwendungen',exact:true}).click();
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
    await page.goto(pathToFileURL(path.join(root,'empty.html')).href);
    for(const area of ['Service-Code','Schnittstellen','Datenqualität']){await page.getByRole('button',{name:area,exact:true}).click();assert.equal(await page.locator('h1').innerText(),area);}
    await page.getByRole('button',{name:'Service-Code',exact:true}).click();await page.getByRole('button',{name:'Tests & Tooling',exact:true}).click();assert.equal(await page.getByText('Tooling-Daten nicht verfügbar',{exact:true}).isVisible(),true);
    await page.goto(url+'#advanced');await page.reload();await page.locator('[data-view-mode="architecture"]').waitFor();await Promise.all([page.waitForEvent('load'),page.getByRole('link',{name:'Workspace Explorer',exact:true}).click()]);assert.equal(await page.locator('#area-navigation').isVisible(),true);assert.equal(new URL(page.url()).hash,'#architecture');
    assert.deepEqual(errors,[]);
    const assets=path.join(root,'workspace-map-assets');
    const saved=fs.readdirSync(assets).map(name=>[name,fs.readFileSync(path.join(assets,name))]);
    for(const [name] of saved)fs.unlinkSync(path.join(assets,name));
    await page.goto(url+'#code');await page.reload();
    await page.getByText('Verwendungsnachweise nicht vollständig verfügbar.',{exact:true}).waitFor();
    assert.equal(await page.locator('.usage-card').count(),0);
    for(const [name,body] of saved)fs.writeFileSync(path.join(assets,name),body);
    await page.getByRole('button',{name:'Erneut laden',exact:true}).click();
    await page.waitForFunction(()=>document.querySelectorAll('.reference-sites li').length===2);
    console.log('PASS: offline shards, grouped references, cross-project outgoing references, shared selection, tooling filters and source navigation, literal notes, keyboard, empty projections, four responsive widths, missing-file recovery');
  }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
