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
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
    await page.goto(pathToFileURL(path.join(root,'empty.html')).href);
    for(const area of ['Service-Code','Schnittstellen','Datenqualität']){await page.getByRole('button',{name:area,exact:true}).click();assert.equal(await page.locator('h1').innerText(),area);}
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
    console.log('PASS: offline shards, grouped references, cross-project outgoing references, shared selection, empty projections, narrow layout, missing-file recovery');
  }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
