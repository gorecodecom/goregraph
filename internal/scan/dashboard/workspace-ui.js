'use strict';
const workspace = window.WORKSPACE_DATA;
Object.assign(state, {area:'architecture', moduleScope:'service', codeQuery:'', codeKind:'all', codeScope:'all', symbol:null, usageDirection:'incoming', usageScope:'all', usageCategory:'all', codeLimit:60, usageLimit:40, codeHistory:[], apiMode:'offered', apiQuery:'', apiStatus:'all', apiLimit:60, endpoint:null, trace:null});
const symbolById = new Map(workspace.symbols.map(symbol => [symbol.id, symbol]));
const symbolsByProject = new Map();
const incomingUsages = new Map();
const outgoingUsages = new Map();
function appendIndex(index, key, value) {
  if (!key) return;
  if (!index.has(key)) index.set(key, []);
  index.get(key).push(value);
}
workspace.symbols.forEach(symbol => appendIndex(symbolsByProject, symbol.project, symbol));
function indexWorkspaceUsage(usage){
  appendIndex(incomingUsages,usage.provider_symbol_id,usage);
  for(const candidate of usage.candidate_symbol_ids||[])if(candidate!==usage.provider_symbol_id)appendIndex(incomingUsages,candidate,usage);
  appendIndex(outgoingUsages,usage.consumer_symbol_id,usage);
}
workspace.usages.forEach(indexWorkspaceUsage);
window.onWorkspaceUsages=records=>records.forEach(indexWorkspaceUsage);
window.onWorkspaceUsageLoad=()=>{if(state.area==='code')renderCode();};
const kindNames = {class:'Klasse',interface:'Interface',record:'Record',enum:'Enum',function:'Funktion',component:'Komponente',type:'Typ'};
const relationNames = {imports_value:'Import',calls:'Aufruf',calls_local:'Lokaler Aufruf',renders_component:'Komponente eingebunden',calls_export:'Export aufgerufen',reexports_value:'Re-Export',instantiates:'Instanziierung',http_reachability:'Über API erreichbar',type_reference:'Typreferenz',imports_type:'Typ-Import',reexports_type:'Typ-Re-Export',return_type:'Rückgabetyp',parameter_type:'Parametertyp',calls_method_owner:'Methodenaufruf auf diesem Typ',field_type:'Feldtyp',extends_type:'Vererbung',static_import:'Statischer Import',implements_type:'Implementierung'};
const statusNames = {OUT_OF_SCOPE:'Außerhalb des Analyseumfangs',RESOLVED:'Zugeordnet',UNRESOLVED:'Ungeklärt',AMBIGUOUS:'Mehrdeutig',MISMATCH:'Abweichung',EXACT:'Eindeutig',MATCHED:'Zugeordnet',COMPLETE:'Vollständig im Analyseumfang',PARTIAL:'Teilweise',UNAVAILABLE:'Nicht verfügbar',UNKNOWN:'Unbekannt'};
const areaNames = {architecture:'Architektur',api:'Schnittstellen',code:'Service-Code',quality:'Datenqualität'};
const selectedProject = () => byId.get(state.selected)?.project||'';
const projectLabel = project => data.nodes.find(node => node.project === project)?.label || project || 'Ziel nicht zugeordnet';
const sourceScope = file => /(^|\/)(test|tests|__tests__|e2e|fixtures|__mocks__)(\/|$)|\.(test|spec)\.[^/]+$/i.test((file||'').replace(/\\/g,'/')) ? 'test' : /(^|\/)src\//.test((file||'').replace(/\\/g,'/')) ? 'production' : 'unknown';
const scopeMatches = (file, filter) => filter === 'all' || sourceScope(file) === filter;
function statusBadge(status) {
  return `<span class="pill ${['UNRESOLVED','AMBIGUOUS','MISMATCH','PARTIAL'].includes(status)?'warn':status==='UNAVAILABLE'||status==='UNKNOWN'?'neutral':''}">${esc(statusNames[status]||status||'Nicht angegeben')}</span>`;
}
function fileReference(project, file, line) {
  return `<div class="file-reference"><span>${esc(projectLabel(project))}</span><code>${esc(file||'Datei nicht angegeben')}${line?':'+line:''}</code></div>`;
}
function emptyState(title, detail) { return `<div class="empty-block"><strong>${esc(title)}</strong><p>${esc(detail)}</p></div>`; }
function selector(id, label, options, value) {
  return `<label class="module-select" for="${id}">${esc(label)}<select id="${id}">${options.map(([key,text])=>`<option value="${esc(key)}" ${key===value?'selected':''}>${esc(text)}</option>`).join('')}</select></label>`;
}
function sourceSelector(id, label, value) {
  return selector(id,label,[['all','Alle Quellbereiche'],['production','Produktionscode'],['test','Testcode'],['unknown','Nicht zuordenbar']],value);
}
function moduleHeading(title, description, workspaceAllowed=true) {
  const node=byId.get(state.selected)||{label:'Kein Service vorhanden',project:'',domain:''};
  return `<div class="page-heading"><div><span class="group-tag">${esc(workspaceAllowed&&state.moduleScope==='all'?'Workspace':groupLabel(node))}</span><h1>${esc(title)}</h1><p>${esc(description)}</p></div></div><div class="context-bar"><div><span>Service-Kontext</span><strong>${esc(node.label)}</strong><small>${esc(node.project)}</small></div>${workspaceAllowed?`<div class="segmented" aria-label="Datenumfang">${[['service','Dieser Service'],['all','Gesamter Workspace']].map(([key,label])=>`<button data-module-scope="${key}" class="${state.moduleScope===key?'active':''}" aria-pressed="${state.moduleScope===key}">${label}</button>`).join('')}</div>`:'<span class="context-hint">Service links wechseln · Auswahl bleibt in allen Bereichen erhalten</span>'}</div>`;
}
function moduleStats(items) {
  return `<div class="module-stats">${items.map(([value,label])=>`<div><strong>${esc(value)}</strong><span>${esc(label)}</span></div>`).join('')}</div>`;
}
function renderAreaNavigation() {
  try{history.replaceState(null,'','#'+state.area);}catch(error){/* Sandboxed previews can deny history access. */}
  document.getElementById('area-navigation').innerHTML=Object.entries(areaNames).map(([area,label])=>`<button data-area="${area}" class="${state.area===area?'active':''}" ${state.area===area?'aria-current="page"':''}>${label}</button>`).join('');
  document.getElementById('area-crumb').textContent=areaNames[state.area];
}
window.resetWorkspaceSelection=()=>{
  state.symbol=null;state.endpoint=null;state.trace=null;state.codeQuery='';state.codeLimit=60;state.usageLimit=40;state.codeHistory=[];state.moduleScope='service';state.qualityTraceIds=null;
};
window.renderWorkspaceArea=()=>{
  renderAreaNavigation();
  if(state.area==='architecture') return false;
  document.querySelector('.shell').classList.remove('with-inspector');
  document.getElementById('breadcrumb-current').textContent=state.moduleScope==='all'&&state.area!=='code'?'Workspace':name(state.selected);
  if(state.area==='code') renderCode();
  if(state.area==='api') renderApi();
  if(state.area==='quality') renderQuality();
  return true;
};
function symbolLink(symbol, label) {
  return symbol?`<button class="inline-link" data-code-symbol="${esc(symbol.id)}">${esc(label||symbol.name)}</button>`:esc(label||'Symbol nicht zugeordnet');
}
function usageRows(symbol) {
  const incoming=state.usageDirection==='incoming';
  const records=(incoming?incomingUsages:outgoingUsages).get(symbol.id)||[];
  return records.filter(usage=>scopeMatches(usage.source_file,state.usageScope)&&(state.usageCategory==='all'||usage.category===state.usageCategory)).sort((a,b)=>(a.category==='direct_reference'?0:1)-(b.category==='direct_reference'?0:1)||a.consumer_project.localeCompare(b.consumer_project)||String(a.source_file||'').localeCompare(String(b.source_file||''))||(a.source_line||0)-(b.source_line||0));
}
function usageCard(usage, incoming) {
  const other=symbolById.get(incoming?usage.consumer_symbol_id:usage.provider_symbol_id);
  const title=other?.name||(incoming?projectLabel(usage.consumer_project):usage.target_qualified_name)||'Nicht zugeordnet';
  return `<article class="usage-card"><div class="usage-card-title">${symbolLink(other,title)}<span class="source-scope ${sourceScope(usage.source_file)}">${esc(scopeLabels[sourceScope(usage.source_file)])}</span></div><p>${esc(relationNames[usage.relation_kind]||usage.relation_kind)} ${usage.category==='reached_through_api'?'<span class="pill neutral">Indirekt über API</span>':''} ${statusBadge(usage.resolution)}</p>${fileReference(usage.consumer_project,usage.source_file,usage.source_line)}${other&&other.project!==selectedProject()?`<small>Zugehöriger Service: ${esc(projectLabel(other.project))}</small>`:''}${usage.api_path?.length?`<details class="api-route-evidence"><summary>API-Pfad: ${esc(usage.api_path.find(step=>step.kind==='http_contract')?.label||'Nachweis öffnen')}</summary><ol>${usage.api_path.filter(step=>step.kind!=='workspace_contract').map(step=>`<li><strong>${esc(step.label)}</strong>${fileReference(step.project,step.file,step.line)}</li>`).join('')}</ol></details>`:''}<details class="raw-evidence"><summary>Einordnung des Nachweises</summary><p>${esc(usage.reason||'Keine weitere Begründung im Export.')}</p><code>${esc(usage.target_qualified_name||'')}</code><small>Konfidenz: ${esc(usage.confidence)}</small>${(usage.limitations||[]).map(limit=>`<p>${esc(limit==='frontend_route_context_partial'?'Frontend-Routenkontext nur teilweise belegt.':limit)}</p>`).join('')}</details></article>`;
}
function groupedUsageRows(rows) {
  const groups=new Map();
  for(const usage of rows){
    const other=state.usageDirection==='incoming'?usage.consumer_symbol_id:usage.provider_symbol_id;
    const key=usage.category==='direct_reference'?JSON.stringify([other||usage.target_qualified_name,usage.consumer_project,usage.source_file]):usage.id;
    if(!groups.has(key))groups.set(key,[]);
    groups.get(key).push(usage);
  }
  return [...groups.values()];
}
function usageGroupCard(records) {
  if(records.length===1)return usageCard(records[0],state.usageDirection==='incoming');
  const usage=records[0],other=symbolById.get(state.usageDirection==='incoming'?usage.consumer_symbol_id:usage.provider_symbol_id);
  const title=other?.name||usage.target_qualified_name||projectLabel(usage.consumer_project);
  return `<article class="usage-card"><div class="usage-card-title">${symbolLink(other,title)}<span class="source-scope ${sourceScope(usage.source_file)}">${esc(scopeLabels[sourceScope(usage.source_file)])}</span></div><p>${records.length} direkte Nachweise · ${[...new Set(records.map(record=>relationNames[record.relation_kind]||record.relation_kind))].map(esc).join(' / ')}</p>${fileReference(usage.consumer_project,usage.source_file)}<details class="reference-sites"><summary>${records.length} Fundstellen mit Zeilennummern</summary><ul>${records.map(record=>`<li><strong>Zeile ${record.source_line||'unbekannt'}</strong><span>${esc(relationNames[record.relation_kind]||record.relation_kind)}</span>${statusBadge(record.resolution)}<small>${esc(record.reason)} · Konfidenz: ${esc(record.confidence)}</small></li>`).join('')}</ul></details></article>`;
}
function symbolDetails(symbol) {
  if(!symbol) return emptyState('Symbol auswählen','Links findest du die Klassen, Interfaces und weiteren Symbole dieses Services.');
  const loading=state.usageLoad||{};
  if(loading.loading||loading.failed?.length)return `<section class="symbol-detail"><h2>${esc(symbol.name)}</h2><p role="status">${loading.loading?'Verwendungsnachweise werden geladen …':'Verwendungsnachweise nicht vollständig verfügbar.'}</p>${loading.failed?.length?`<p>${esc(loading.failed.join(', '))}</p><button class="button" data-module-action="retry-usages">Erneut laden</button>`:''}<p class="data-note">Fehlende Dateien sind keine leere Trefferliste. Das Dashboard und sein Verzeichnis workspace-map-assets müssen zusammenbleiben.</p></section>`;
  const incoming=incomingUsages.get(symbol.id)||[],outgoing=outgoingUsages.get(symbol.id)||[],rows=usageRows(symbol),usageGroups=groupedUsageRows(rows);
  const direct=incoming.filter(u=>u.category==='direct_reference').length;
  const indirect=incoming.filter(u=>u.category==='reached_through_api').length;
  return `<section class="symbol-detail" aria-label="Symbol und Verwendungen"><div class="detail-title"><div><span class="detail-eyebrow">${esc(kindNames[symbol.kind]||symbol.kind)} · ${esc(symbol.language)}</span><h2>${esc(symbol.name)}</h2></div>${state.codeHistory.length?'<button class="button" data-module-action="code-back">‹ Zurück</button>':''}</div><code class="qualified-name">${esc(symbol.qualified_name)}</code>${fileReference(symbol.project,symbol.declaration_file,symbol.declaration_line)}<div class="symbol-summary"><span>${direct} direkte Verwendungsnachweise</span><span>${indirect} über API</span><span>Verwendende Projekte: ${new Set(incoming.map(u=>u.consumer_project)).size}</span></div><div class="module-tabs segmented" aria-label="Verwendungsrichtung"><button data-usage-direction="incoming" class="${state.usageDirection==='incoming'?'active':''}" aria-pressed="${state.usageDirection==='incoming'}">Wird verwendet von (${incoming.length})</button><button data-usage-direction="outgoing" class="${state.usageDirection==='outgoing'?'active':''}" aria-pressed="${state.usageDirection==='outgoing'}">Verwendet selbst ${state.usageDirection==='outgoing'?'('+outgoing.length+')':''}</button></div><div class="module-filter usage-filters">${sourceSelector('usage-scope','Verwendungsstellen',state.usageScope)}${selector('usage-category','Nachweisart',[['all','Alle Nachweise'],['direct_reference','Direkte Referenzen'],['reached_through_api','Über API erreichbar'],['unresolved','Ungeklärt'],['ambiguous','Mehrdeutig']],state.usageCategory)}</div><p class="list-count">${rows.length} Nachweise in ${usageGroups.length} Einträgen · statische Beziehungen, keine Laufzeitmessung</p><div class="usage-list">${rows.length?usageGroups.slice(0,state.usageLimit).map(usageGroupCard).join(''):emptyState('Keine Verwendungsnachweise im Filter','Das belegt nicht, dass das Symbol ungenutzt ist. Dynamische Aufrufe, generierter Code und Dependency Injection können fehlen.')}</div>${usageGroups.length>state.usageLimit?`<button class="button load-more" data-module-action="more-usages">Weitere Einträge (${usageGroups.length-state.usageLimit})</button>`:''}</section>`;
}
function renderCode() {
  state.usageLoad=ensureWorkspaceUsages(selectedProject(),state.usageDirection==='outgoing');
  const all=(symbolsByProject.get(selectedProject())||[]).slice().sort((a,b)=>a.name.localeCompare(b.name)||String(a.qualified_name||a.name).localeCompare(String(b.qualified_name||b.name)));
  const query=state.codeQuery.toLocaleLowerCase();
  const symbols=all.filter(symbol=>(state.codeKind==='all'||symbol.kind===state.codeKind)&&scopeMatches(symbol.declaration_file,state.codeScope)&&(!query||`${symbol.name} ${symbol.qualified_name} ${symbol.declaration_file}`.toLocaleLowerCase().includes(query)));
  if(!state.symbol||!symbols.some(symbol=>symbol.id===state.symbol)) state.symbol=symbols.find(s=>s.kind==='class'&&s.name.endsWith('Service')&&sourceScope(s.declaration_file)==='production'&&(incomingUsages.get(s.id)||[]).some(u=>u.category==='direct_reference'))?.id||symbols[0]?.id||null;
  const symbol=symbolById.get(state.symbol);
  document.getElementById('content').innerHTML=moduleHeading('Service-Code','Klassen, Interfaces und Funktionen entdecken – und ihre Verwendungen nachvollziehen.',false)+moduleStats([[all.length,'Symbole im Export'],[all.filter(s=>['class','interface','record','enum'].includes(s.kind)).length,'Klassen & Typen'],[new Set(all.map(s=>s.declaration_file)).size,'Quelldateien']])+`<div class="module-filter"><label class="module-search" for="code-search">Symbole suchen<input id="code-search" type="search" placeholder="Klasse, Package oder Datei …" value="${esc(state.codeQuery)}"></label>${selector('code-kind','Symbolart',[['all','Alle Symbolarten'],...Object.entries(kindNames).filter(([kind])=>all.some(s=>s.kind===kind))],state.codeKind)}${sourceSelector('code-scope','Deklarationen',state.codeScope)}</div><div class="code-workbench"><section class="symbol-browser" aria-label="Symbole des Services"><header><h2>Symbole</h2><span>${symbols.length} Treffer</span></header><div class="symbol-list">${symbols.length?symbols.slice(0,state.codeLimit).map(s=>`<button class="symbol-row ${s.id===state.symbol?'active':''}" data-symbol="${esc(s.id)}" aria-pressed="${s.id===state.symbol}"><span class="symbol-kind">${esc(kindNames[s.kind]||s.kind)}</span><strong>${esc(s.name)}</strong><small>${esc(s.package||s.declaration_file)}</small><span class="symbol-row-meta">${state.usageLoad.loading||state.usageLoad.failed.length?'—':(incomingUsages.get(s.id)||[]).filter(u=>u.category==='direct_reference').length} direkte Nachweise · ${esc(scopeLabels[sourceScope(s.declaration_file)])}</span></button>`).join(''):emptyState('Keine Symbole gefunden',all.length?'Suche oder Filter anpassen.':'Für diesen Service sind keine Symboldeklarationen im vorliegenden Export enthalten.')}</div>${symbols.length>state.codeLimit?`<button class="button load-more" data-module-action="more-symbols">Weitere Symbole (${symbols.length-state.codeLimit})</button>`:''}</section>${symbolDetails(symbol)}</div><p class="data-note">Direkte Referenzen und Erreichbarkeit über APIs sind getrennt gekennzeichnet. Aufrufe auf einem Typ sind keine vollständige Methodenliste. Verwendungsanalyse: teilweise; fehlende Nachweise bedeuten nicht „ungenutzt“.</p>`;
}
function endpointController(endpoint) {
  const matches=(symbolsByProject.get(endpoint.provider_project)||[]).filter(s=>s.name===endpoint.controller&&s.declaration_file===endpoint.file);
  return matches.length===1?matches[0]:null;
}
function apiDetail() {
  const endpoint=workspace.endpoints.find(e=>e.id===state.endpoint);
  const trace=workspace.traces.find(t=>t.id===state.trace);
  if(!endpoint&&!trace) return '';
  let body='';
  if(endpoint){
    body=`<span class="detail-eyebrow">Angebotene Schnittstelle</span><h2><span class="http-method">${esc(endpoint.http_method)}</span> ${esc(endpoint.path)}</h2><p>${esc(endpoint.provider_service)} · ${esc(endpoint.handler)}</p>${fileReference(endpoint.provider_project,endpoint.file,endpoint.line)}<p>${symbolLink(endpointController(endpoint),'Controller im Service-Code öffnen')}</p><dl class="api-types"><dt>Request</dt><dd>${esc(endpoint.request_type||'Nicht angegeben')}</dd><dt>Response</dt><dd>${esc(endpoint.response_type||'Nicht angegeben')}</dd></dl><h3>Parameter</h3>${(endpoint.parameters||[]).length?`<ul class="parameter-list">${endpoint.parameters.map(p=>`<li><strong>${esc(p.name)}</strong><code>${esc(p.type)}</code><small>${esc(p.location)} · ${p.required===true?'Pflicht':p.required===false?'Optional':'Pflichtstatus unbekannt'}</small></li>`).join('')}</ul>`:'<p class="muted">Keine Parameter im Export angegeben.</p>'}<h3>Verwendungsstellen (${(endpoint.consumers||[]).length})</h3>${(endpoint.consumers||[]).map(c=>`<article class="usage-card"><strong>${esc(projectLabel(c.project))}</strong> ${statusBadge(c.resolution)}${fileReference(c.project,c.file,c.line)}</article>`).join('')||'<p class="muted">Keine Aufrufer im Export zugeordnet. Das belegt keine ungenutzte API.</p>'}${(endpoint.mismatches||[]).length?`<h3>Prüfhinweise</h3>${endpoint.mismatches.map(m=>`<p class="notice">${esc(m.kind==='missing_call_auth_evidence'?'Authentifizierungsnachweis am Aufrufer fehlt. Das ist eine Lücke der statischen Belege, kein nachgewiesener Authentifizierungsfehler.':m.reason)}</p>`).join('')}`:''}<details class="raw-evidence"><summary>Abdeckung und Security-Belege</summary><p>${statusBadge(endpoint.coverage)} · Konfidenz ${esc(endpoint.confidence)}</p>${(endpoint.security||[]).map(s=>`<p>${esc(s.summary)}</p>${fileReference(endpoint.provider_project,s.file,s.line)}${(s.limitations||[]).map(l=>`<p class="muted">${esc(l)}</p>`).join('')}`).join('')||'<p>Keine Security-Belege im Export.</p>'}</details>`;
  } else {
    const linked=workspace.endpoints.filter(e=>(e.consumers||[]).some(consumer=>(consumer.evidence_ids||[]).includes(trace.id))||(e.provider_project===trace.to_project&&e.http_method===trace.method&&e.path===trace.path));
    body=`<span class="detail-eyebrow">Verwendete Schnittstelle</span><h2>${esc(trace.method)} ${esc(trace.path)}</h2><p>${esc(projectLabel(trace.from_project))} → ${esc(projectLabel(trace.to_project))}</p>${statusBadge(trace.status)}<p class="data-note">Originalstatus: ${esc(trace.risk)}. Ein ungeklärter Treffer ist kein belegter Laufzeitfehler.</p>${trace.steps.map(step=>`<article class="usage-card"><strong>${esc(step.symbol||step.label)}</strong><p>${esc(step.kind)}</p>${fileReference(step.project,step.file,step.line)}</article>`).join('')||emptyState('Kein Einzelbeleg im Export','Die Anfrage ist gezählt, enthält aber keine passende Quellenangabe.')}${linked.length===1?`<button class="button" data-endpoint="${esc(linked[0].id)}">Angebotene Schnittstelle öffnen</button>`:''}`;
  }
  return `<aside class="module-detail" aria-label="Schnittstellendetails"><header><strong>Schnittstelle & Nachweise</strong><button class="close-panel" data-module-action="close-api" aria-label="Schnittstellendetails schließen">×</button></header><div class="module-detail-body">${body}</div></aside>`;
}
function scopedTraces() {return workspace.traces.filter(t=>state.moduleScope==='all'||t.from_project===selectedProject()||t.to_project===selectedProject());}
function renderApi() {
  const query=state.apiQuery.toLocaleLowerCase(),offered=state.apiMode==='offered';
  const base=offered?workspace.endpoints.filter(e=>state.moduleScope==='all'||e.provider_project===selectedProject()):workspace.traces.filter(t=>state.qualityTraceIds?state.qualityTraceIds.includes(t.id):state.moduleScope==='all'||t.from_project===selectedProject());
  const rows=base.filter(row=>(!query||`${row.http_method||row.method} ${row.path} ${row.provider_service||projectLabel(row.to_project)} ${row.handler||''}`.toLocaleLowerCase().includes(query))&&(offered||state.apiStatus==='all'||row.status===state.apiStatus));
  const selected=!!(state.endpoint||state.trace);
  document.getElementById('content').innerHTML=moduleHeading('Schnittstellen','Angebotene APIs und ihre statisch erkannten Verwendungen in einer Ansicht.')+`${state.qualityTraceIds?'<p class="notice">Prüffälle aus Datenqualität: eingehende und ausgehende Aufrufe im gewählten Kontext. Mit „Verwendete APIs“ zurück zu den ausgehenden Aufrufen.</p>':''}<div class="module-tabs segmented"><button data-api-mode="offered" class="${offered?'active':''}" aria-pressed="${offered}">Angebotene APIs</button><button data-api-mode="used" class="${!offered?'active':''}" aria-pressed="${!offered}">Verwendete APIs</button></div><div class="module-filter"><label class="module-search" for="api-search">Schnittstellen suchen<input id="api-search" type="search" placeholder="Pfad, Methode, Service oder Handler …" value="${esc(state.apiQuery)}"></label>${!offered?selector('api-status','Zuordnung',[['all','Alle Status'],['RESOLVED','Zugeordnet'],['UNRESOLVED','Ungeklärt'],['AMBIGUOUS','Mehrdeutig'],['MISMATCH','Abweichung'],['OUT_OF_SCOPE','Außerhalb des Analyseumfangs']],state.apiStatus):''}<span class="list-count">${rows.length} von ${base.length} ${offered?'APIs':'Aufrufnachweisen'}</span></div><div class="api-workbench ${selected?'has-detail':''}"><div class="module-table-wrap"><table class="module-table"><thead><tr><th>Methode / Pfad</th><th>${offered?'Handler / Anbieter':'Aufrufer → Ziel'}</th><th>${offered?'Aufrufstellen':'Zuordnung'}</th><th>Details</th></tr></thead><tbody>${rows.slice(0,state.apiLimit).map(row=>offered?`<tr><td><span class="http-method ${esc(row.http_method.toLowerCase())}">${esc(row.http_method)}</span><code>${esc(row.path)}</code></td><td><strong>${esc(row.handler)}</strong><small>${esc(row.provider_service)}</small></td><td>${(row.consumers||[]).length}${row.mismatches?.length?'<small>Prüfhinweise vorhanden</small>':''}</td><td><button class="row-toggle" data-endpoint="${esc(row.id)}" aria-label="Details ${esc(row.http_method)} ${esc(row.path)}">Ansehen</button></td></tr>`:`<tr><td><span class="http-method">${esc(row.method)}</span><code>${esc(row.path)}</code></td><td><strong>${esc(projectLabel(row.from_project))}</strong><small>→ ${esc(projectLabel(row.to_project))}</small></td><td>${statusBadge(row.status)}</td><td><button class="row-toggle" data-trace="${esc(row.id)}" aria-label="Nachweis ${esc(row.method)} ${esc(row.path)}">Ansehen</button></td></tr>`).join('')}</tbody></table>${!rows.length?emptyState('Keine passenden Schnittstellen',base.length?'Suche oder Filter anpassen.':'Für diesen Service enthält der Export in dieser Ansicht keine Einträge. Andere Verbindungsarten findest du unter Architektur.'):''}${rows.length>state.apiLimit?`<button class="button load-more" data-module-action="more-api">Weitere Einträge (${rows.length-state.apiLimit})</button>`:''}</div>${apiDetail()}</div><p class="data-note">Angebotene APIs stammen aus dem API-Katalog. Verwendete APIs sind einzelne exportierte Aufrufnachweise und können dieselbe Route mehrfach enthalten. Java-Imports sind keine belegten HTTP-Aufrufe.</p>`;
}
function renderQuality() {
  const traces=scopedTraces(),all=state.moduleScope==='all';
  const capabilities=workspace.capabilities.filter(c=>all||c.project===selectedProject());
  const codeCoverage=workspace.usageCoverage.filter(c=>all||c.project===selectedProject());
  const families=workspace.diagnosticFamilies.filter(f=>all||f.service===name(state.selected)||(f.affected_projects||[]).some(p=>p===selectedProject()||p===name(state.selected)));
  const counts=status=>traces.filter(t=>t.status===status).length;
  const coverageGroups=new Map();
  for(const capability of capabilities){const key=[capability.id,capability.language,capability.coverage].join('|');if(!coverageGroups.has(key))coverageGroups.set(key,{...capability,projects:new Set()});coverageGroups.get(key).projects.add(capability.project);}
  document.getElementById('content').innerHTML=moduleHeading('Datenqualität','Abdeckung, offene Zuordnungen und die Grenzen des aktuellen Datenstands.')+`<div class="quality-banner"><div><span>Datenstand</span><strong>${esc((workspace.generated?new Date(workspace.generated).toLocaleString('de-DE'):'Unbekannt'))}</strong></div><div><span>Integrität</span><strong>${esc(({valid:'Gültig',invalid:'Ungültig',unknown:'Unbekannt'})[workspace.health.integrity]||workspace.health.integrity||'Unbekannt')}</strong></div><div><span>Aktualität</span><strong>${(!workspace.health.freshness||workspace.health.freshness==='unknown')?'Ungeprüft':esc(workspace.health.freshness)}</strong></div><div><span>Abdeckung</span><strong>${workspace.health.coverage==='partial'?'Teilweise':esc(workspace.health.coverage||'Unbekannt')}</strong></div><div><span>Indexierte Projekte · Workspace</span><strong>${workspace.workspaceCoverage.indexed_projects??'—'} / ${workspace.workspaceCoverage.known_projects??'—'}</strong></div></div><p class="data-note">Ein indexiertes Projekt ist nicht automatisch vollständig analysiert. Aktualität wurde nicht gegen die aktuellen Quelldateien geprüft.</p>`+moduleStats([[traces.length,'HTTP-Aufrufnachweise im Kontext'],[counts('RESOLVED'),'Zugeordnet'],[counts('UNRESOLVED'),'Ungeklärt'],[counts('AMBIGUOUS')+counts('MISMATCH'),'Mehrdeutig / abweichend'],...(counts('OUT_OF_SCOPE')?[[counts('OUT_OF_SCOPE'),'Außerhalb des Analyseumfangs']]:[]),...(traces.some(t=>!['RESOLVED','UNRESOLVED','AMBIGUOUS','MISMATCH','OUT_OF_SCOPE'].includes(t.status))?[[traces.filter(t=>!['RESOLVED','UNRESOLVED','AMBIGUOUS','MISMATCH','OUT_OF_SCOPE'].includes(t.status)).length,'Andere / unbekannte Status']]:[])])+`<div class="quality-grid"><section class="quality-card"><h2>Offene Zuordnungen untersuchen</h2><p>Von einem Status direkt zu den betroffenen Aufrufen wechseln.</p><div class="quality-actions">${[['UNRESOLVED','Ungeklärte Aufrufe'],['AMBIGUOUS','Mehrdeutige Aufrufe'],['MISMATCH','Abweichende Aufrufe']].map(([status,label])=>`<button data-quality-status="${status}" ${counts(status)?'':'disabled'}><span>${label}</span><strong>${counts(status)}</strong><span>→</span></button>`).join('')}</div><p class="data-note">Im Service-Kontext zählen hier eingehende und ausgehende Aufrufe. Der Sprung zeigt die exakten betroffenen Nachweise.</p></section><section class="quality-card"><h2>Verwendungsanalyse einordnen</h2><p>Klassen und Verwendungen sind statische Nachweise. Insbesondere dynamische Bindungen können fehlen.</p><div class="coverage-tags">${[...new Set(codeCoverage.filter(c=>c.capability==='direct_usages').map(c=>`${c.language}: ${statusNames[c.coverage]||c.coverage}`))].map(label=>`<span class="pill neutral">${esc(label)}</span>`).join('')}</div><button class="button" data-area="code">Service-Code öffnen</button><p class="data-note">Vollständig bedeutet vollständig im jeweiligen Analyseumfang – keine Garantie für sämtliche Laufzeitbeziehungen.</p></section></div><section class="quality-card"><h2>Diagnosegruppen (${families.length})</h2><p>Zusammengefasste Hinweise aus dem Export. Die Zuordnung folgt den dort angegebenen Services.</p>${families.length?`<div class="diagnostic-list">${families.map(f=>`<details><summary><span>${esc(f.code)}</span><strong>${esc(f.service)} · ${esc(f.route_pattern||'Ohne Route')}</strong><small>${f.affected_count} betroffen</small></summary><p>${esc(f.root_cause)}</p><p>${esc(f.suggested_check)}</p></details>`).join('')}</div>`:emptyState('Keine Diagnosegruppe zugeordnet','Fehlende Hinweise beweisen keine vollständige Analyse.')}</section><section class="quality-card"><h2>Analyse-Abdeckung nach Fähigkeit</h2><div class="module-table-wrap"><table class="module-table"><thead><tr><th>Fähigkeit</th><th>Sprache</th><th>Abdeckung</th><th>${all?'Projekte':'Einordnung'}</th></tr></thead><tbody>${[...coverageGroups.values()].sort((a,b)=>a.id.localeCompare(b.id)||a.language.localeCompare(b.language)).map(c=>`<tr><td>${esc(c.id)}</td><td>${esc(c.language)}</td><td>${statusBadge(c.coverage)}</td><td>${all?c.projects.size:`<span>${esc(c.reason)}</span>`}</td></tr>`).join('')}</tbody></table></div></section>`;
}
function jumpToSymbol(id, remember=true) {
  const symbol=symbolById.get(id),node=symbol&&data.nodes.find(n=>n.project===symbol.project);
  if(!node) return;
  if(remember&&state.area==='code'&&state.symbol)state.codeHistory.push({symbol:state.symbol,selected:state.selected});
  state.area='code';state.selected=node.id;state.symbol=id;state.codeQuery='';state.codeKind='all';state.codeScope='all';state.usageScope='all';state.usageCategory='all';state.usageLimit=40;state.usageDirection='incoming';state.codeLimit=Math.max(60,(symbolsByProject.get(symbol.project)||[]).length);state.edge=null;state.domainPair=null;
  render();document.querySelector('.symbol-detail')?.scrollIntoView({block:'start'});
}
function moduleClick(target) {
  const area=target.closest('[data-area]');
  if(area){if(area.dataset.area==='architecture'&&state.area!=='architecture')state.view='focus';state.area=area.dataset.area;state.edge=null;state.domainPair=null;state.endpoint=null;state.trace=null;state.qualityTraceIds=null;render();window.scrollTo({top:0});return;}
  const scope=target.closest('[data-module-scope]');
  if(scope){state.moduleScope=scope.dataset.moduleScope;state.endpoint=null;state.trace=null;state.qualityTraceIds=null;render();return;}
  const symbol=target.closest('[data-symbol]');
  if(symbol){state.symbol=symbol.dataset.symbol;state.usageLimit=40;renderCode();if(innerWidth<1000)document.querySelector('.symbol-detail')?.scrollIntoView({block:'start'});return;}
  const jump=target.closest('[data-code-symbol]');
  if(jump){jumpToSymbol(jump.dataset.codeSymbol);return;}
  const direction=target.closest('[data-usage-direction]');
  if(direction){state.usageDirection=direction.dataset.usageDirection;state.usageLimit=40;renderCode();return;}
  const apiMode=target.closest('[data-api-mode]');
  if(apiMode){state.apiMode=apiMode.dataset.apiMode;state.endpoint=null;state.trace=null;state.apiLimit=60;state.qualityTraceIds=null;renderApi();return;}
  const endpoint=target.closest('[data-endpoint]'),trace=target.closest('[data-trace]');
  if(endpoint||trace){state.endpoint=endpoint?.dataset.endpoint||null;state.trace=trace?.dataset.trace||null;renderApi();if(innerWidth<1100)document.querySelector('.module-detail')?.scrollIntoView({block:'start'});return;}
  const quality=target.closest('[data-quality-status]');
  if(quality){state.qualityTraceIds=scopedTraces().map(t=>t.id);state.area='api';state.apiMode='used';state.apiStatus=quality.dataset.qualityStatus;state.apiQuery='';state.endpoint=null;state.trace=null;render();window.scrollTo({top:0});return;}
  const action=target.closest('[data-module-action]')?.dataset.moduleAction;
  if(action==='code-back'){const previous=state.codeHistory.pop();if(previous)jumpToSymbol(previous.symbol,false);}
  if(action==='retry-usages'){retryWorkspaceUsages();renderCode();}
  if(action==='more-symbols'){state.codeLimit+=60;renderCode();}
  if(action==='more-usages'){state.usageLimit+=40;renderCode();}
  if(action==='more-api'){state.apiLimit+=60;renderApi();}
  if(action==='close-api'){state.endpoint=null;state.trace=null;renderApi();}
}
document.addEventListener('click',event=>moduleClick(event.target));
document.addEventListener('change',event=>{
  const fields={'code-kind':'codeKind','code-scope':'codeScope','usage-scope':'usageScope','usage-category':'usageCategory','api-status':'apiStatus'};
  const field=fields[event.target.id];if(!field)return;
  state[field]=event.target.value;if(field==='apiStatus'){state.endpoint=null;state.trace=null;}state.codeLimit=60;state.usageLimit=40;state.apiLimit=60;render();document.getElementById(event.target.id)?.focus({preventScroll:true});
});
document.addEventListener('input',event=>{
  const field=event.target.id==='code-search'?'codeQuery':event.target.id==='api-search'?'apiQuery':null;
  if(!field)return;const id=event.target.id,position=event.target.selectionStart;state[field]=event.target.value;if(field==='apiQuery'){state.endpoint=null;state.trace=null;}state.codeLimit=60;state.apiLimit=60;render();const input=document.getElementById(id);input.focus({preventScroll:true});input.setSelectionRange(position,position);
});
document.getElementById('workspace-name').textContent=workspace.root.replace(/\\/g,'/').split('/').filter(Boolean).pop()||'Workspace';
document.getElementById('snapshot-health').textContent='Abdeckung: '+(workspace.health.coverage||'unbekannt')+' · Aktualität: '+(workspace.health.freshness||'unbekannt');
document.getElementById('advanced-tools').addEventListener('click',event=>{event.preventDefault();location.hash='advanced';location.reload();});
const initialArea=location.hash.slice(1);
if(areaNames[initialArea])state.area=initialArea;
render();

document.addEventListener('keydown',event=>{if(event.key==='Escape'&&state.area==='api'&&(state.endpoint||state.trace)){const attr=state.endpoint?'data-endpoint':'data-trace',id=state.endpoint||state.trace;state.endpoint=null;state.trace=null;renderApi();[...document.querySelectorAll('['+attr+']')].find(e=>e.getAttribute(attr)===id)?.focus({preventScroll:true});}});
