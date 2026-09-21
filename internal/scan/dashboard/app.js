'use strict';
const data = window.ARCHITECTURE_DATA;
const byId = new Map(data.nodes.map(node => [node.id, node]));
const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
const state = {selected:data.nodes[0]?.id||null, view:'overview', domainPair:null, overviewMode:'relations', panelReturn:null, direction:'both', query:'', scope:'all', kind:'all', edge:null, inPage:0, outPage:0};
const pageSize = 6;
const domainNames = new Map(data.groups.map(group => [group.id, group.label]));
const groupOf = node => node?.domain||'';
const groupLabel = node => domainNames.get(groupOf(node)) || groupOf(node);
const groups = data.groups.slice().sort((a,b) => a.order - b.order).map(group => group.id).filter(group => data.nodes.some(node => groupOf(node) === group));
const totals = edges => edges.reduce((sum, edge) => sum + (edge.total || 0), 0);
const projectedEdges = () => EvidenceModel.project(data.edges,state.scope,state.kind);
const incoming = id => projectedEdges().filter(edge => edge.to === id);
const outgoing = id => projectedEdges().filter(edge => edge.from === id);
const scopeLabels = {all:'Alle Belege', production:'Produktionscode', test:'Testcode', unknown:'Nicht zuordenbar'};
const kindLabels = {http:'HTTP', java:'Java-Client', unknown:'Nicht bestimmt'};
const edgeKind = edge => [...new Set(edge.records.map(record => record.kind))].map(kind => kindLabels[kind]).join(' + ');
const name = id => byId.get(id)?.label || id;
const isRisk = edge => (edge.unresolved || 0) + (edge.mismatched || 0) > 0;
const sourceLabel = node => node.domain_source === 'production_package' ? 'Produktions-Package' : node.domain_source === 'dashboard_config' ? 'Manuelle Zuordnung' : 'Projektpfad';
function groupingButton() { return '<button class="text-button" data-action="grouping">Gruppierung nachvollziehen</button>'; }
function renderNav() {
  const query = state.query.toLocaleLowerCase();
  let count = 0;
  document.getElementById('service-list').innerHTML = groups.map(group => {
    const nodes = data.nodes.filter(node => groupOf(node) === group && (!query || `${node.label} ${node.project}`.toLocaleLowerCase().includes(query))).sort((a,b) => (a.architecture_order??Number.MAX_SAFE_INTEGER)-(b.architecture_order??Number.MAX_SAFE_INTEGER)||a.label.localeCompare(b.label));
    count += nodes.length;
    if (!nodes.length) return '';
    const open = query || group === groupOf(byId.get(state.selected));
    return `<details class="nav-group" ${open ? 'open' : ''}><summary>${esc(domainNames.get(group) || group)}<small>${nodes.length}</small></summary>${nodes.map(node => `<button class="nav-item ${node.id === state.selected && (state.view === 'focus' || (state.area && state.area !== 'architecture')) ? 'active' : ''}" data-node="${esc(node.id)}" ${node.id === state.selected && (state.view === 'focus' || (state.area && state.area !== 'architecture')) ? 'aria-current="true"' : ''} title="${esc(node.project)}">${esc(node.label)}</button>`).join('')}</details>`;
  }).join('') || '<p class="no-results">Kein passender Service. Suche nach Name oder Projektpfad.</p>';
  document.getElementById('node-count').textContent = `${count} Projekte`;
}
function nodeSvg(node, x, y, selected = false) {
  const width=258, height=selected ? 154 : 66;
  const i=incoming(node.id), o=outgoing(node.id);
  return `<g class="node" role="button" tabindex="0" data-node="${esc(node.id)}" aria-label="${esc(node.label)}, ${i.length} Aufrufer, ${o.length} Ziele"><title>${esc(node.project)} — ${esc(groupLabel(node))}</title><rect class="node-surface" x="${x}" y="${y}" width="${width}" height="${height}" rx="7" fill="${selected ? '#eef8f8' : '#fff'}" stroke="${selected ? '#087b80' : '#cfdae3'}" stroke-width="${selected ? 2 : 1}"/>${selected ? `<text x="${x+18}" y="${y+29}" fill="#087b80" font-size="12" font-weight="600">${esc(groupLabel(node))} / ausgewählt</text><text x="${x+18}" y="${y+59}" font-size="16" font-weight="650" fill="#172c3c">${esc(node.label.length>32?node.label.slice(0,29)+'…':node.label)}</text><line x1="${x+18}" x2="${x+240}" y1="${y+80}" y2="${y+80}" stroke="#c9e2e3"/><text x="${x+18}" y="${y+104}" font-size="12" fill="#476875">${i.length} Aufrufer · ${o.length} Ziel-Services</text><text x="${x+18}" y="${y+130}" font-size="12" fill="#476875">${totals([...i,...o])} statische Belege</text>` : `<text x="${x+14}" y="${y+26}" font-size="13" font-weight="600" fill="#172c3c">${esc(node.label)}</text><text x="${x+14}" y="${y+48}" font-size="11" fill="#647a89">${esc(groupLabel(node))} · ${node.role === 'frontend' ? 'Frontend' : 'Backend'}</text>`}</g>`;
}
function edgeSvg(edge, sy, ey, inbound) {
  const x1=inbound ? 282 : 679, x2=inbound ? 421 : 818, mid=(x1+x2)/2;
  const color=inbound ? '#386bb3' : '#087b80';
  const badgeY=inbound ? sy : ey;
  const label=`${edge.total} ${edge.total === 1 ? 'Beleg' : 'Belege'}`;
  const kind=edgeKind(edge);
  return `<path class="connector ${state.edge===edge.id?'active':''}" opacity="${state.edge && state.edge !== edge.id ? 0.22 : 1}" d="M ${x1} ${sy} C ${mid} ${sy}, ${mid} ${ey}, ${x2-3} ${ey}" stroke="${color}" stroke-dasharray="${edge.records.every(record => record.kind === 'java') ? '5 3' : 'none'}" marker-end="url(#${inbound ? 'in' : 'out'}-arrow)"/><circle cx="${x1}" cy="${sy}" r="3" fill="${color}"/><g class="edge-button" role="button" tabindex="0" data-edge="${esc(edge.id)}" aria-label="${esc(name(edge.from))} an ${esc(name(edge.to))}: ${label}, ${esc(kind)}. Schnittstelle anzeigen"><rect x="${mid-48}" y="${badgeY-11}" width="96" height="23" rx="11" fill="${state.edge === edge.id ? '#dbeaf7' : '#fff'}" stroke="${isRisk(edge) ? '#c79535' : '#cad9e5'}"/><text x="${mid}" y="${badgeY+4}" text-anchor="middle" font-size="11" fill="${color}" font-weight="600">${esc(label)}${isRisk(edge) ? ' *' : ''}</text><text x="${mid}" y="${badgeY+28}" text-anchor="middle" font-size="10" fill="#637987">${esc(kind)}</text></g>`;
}
function emptySvg(x,y,text,subtext) {
  return `<rect x="${x}" y="${y}" width="258" height="95" rx="7" fill="#f8fafc" stroke="#dce5eb" stroke-dasharray="4 4"/><text x="${x+17}" y="${y+33}" font-size="13" fill="#617787">${esc(text)}</text><text x="${x+17}" y="${y+57}" font-size="11" fill="#7b8d99">${esc(subtext)}</text><text x="${x+17}" y="${y+76}" font-size="11" fill="#7b8d99">im vorliegenden Datenstand.</text>`;
}
function paginationHtml(edges, page, side) {
  if(edges.length <= pageSize) return '<span></span>';
  const pages=Math.ceil(edges.length/pageSize);
  return `<div class="page-control"><button data-page="${side}" data-delta="-1" ${page === 0 ? 'disabled' : ''} aria-label="Vorherige ${side === 'in' ? 'Aufrufer' : 'Ziele'}">‹</button><span>${side === 'in' ? 'Aufrufer' : 'Ziele'} ${page*pageSize+1}–${Math.min((page+1)*pageSize,edges.length)} von ${edges.length}</span><button data-page="${side}" data-delta="1" ${page+1 >= pages ? 'disabled' : ''} aria-label="Weitere ${side === 'in' ? 'Aufrufer' : 'Ziele'}">›</button></div>`;
}
function graphHtml(node, ins, outs) {
  const visibleIn=state.direction === 'out' ? [] : ins.slice(state.inPage*pageSize,(state.inPage+1)*pageSize);
  const visibleOut=state.direction === 'in' ? [] : outs.slice(state.outPage*pageSize,(state.outPage+1)*pageSize);
  const count=Math.max(visibleIn.length, visibleOut.length,3);
  const height=116+count*92, cy=(height-154)/2+12;
  const top=95;
  let paths='', cards='';
  const addSide=(edges, inbound) => edges.forEach((edge,index) => {
    const y=top+index*92;
    const port=cy+48+(index+1)*86/(edges.length+1);
    paths+=edgeSvg(edge,inbound ? y+33 : port,inbound ? port : y+33,inbound);
    cards+=nodeSvg(byId.get(inbound ? edge.from : edge.to),inbound ? 24 : 818,y);
  });
  addSide(visibleIn,true);addSide(visibleOut,false);
  if(!visibleIn.length) cards+=emptySvg(24,cy+25,state.direction === 'out' ? 'Aufrufer ausgeblendet' : 'Keine Aufrufer erkannt',state.direction === 'out' ? 'Richtungsfilter aktiv' : 'Keine passenden eingehenden Belege');
  if(!visibleOut.length) cards+=emptySvg(818,cy+25,state.direction === 'in' ? 'Ziele ausgeblendet' : 'Keine Ziele erkannt',state.direction === 'in' ? 'Richtungsfilter aktiv' : 'Keine passenden ausgehenden Belege');
  return `<section class="map-panel" aria-label="Direkte Service-Verbindungen"><div class="panel-heading"><div><h2>Direkte Nachbarschaft</h2><p>Jede Verbindung hat einen sichtbaren Anfang und ein eindeutiges Ziel.</p></div><div class="segmented" aria-label="Richtung filtern">${[['both','Alle'],['in','Eingehend'],['out','Ausgehend']].map(([id,label])=>`<button data-direction="${id}" class="${state.direction === id ? 'active' : ''}" aria-pressed="${state.direction === id}">${label}</button>`).join('')}</div></div><div class="diagram-scroll"><svg class="diagram" viewBox="0 0 1100 ${height}" role="group" aria-label="Aufrufer links, ${esc(node.label)} in der Mitte, Ziele rechts"><defs><marker id="in-arrow" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto" markerUnits="userSpaceOnUse"><path d="M0 0 L7 3.5 L0 7 Z" fill="#386bb3"/></marker><marker id="out-arrow" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto" markerUnits="userSpaceOnUse"><path d="M0 0 L7 3.5 L0 7 Z" fill="#087b80"/></marker></defs><text x="24" y="51" fill="#386bb3" font-size="13" font-weight="600">Aufrufer <tspan fill="#708594" font-weight="400">(${ins.length})</tspan></text><text x="421" y="51" fill="#567180" font-size="13" font-weight="600">Ausgewählter Service</text><text x="818" y="51" fill="#087b80" font-size="13" font-weight="600">Ziel-Services <tspan fill="#708594" font-weight="400">(${outs.length})</tspan></text><line x1="24" x2="1076" y1="68" y2="68" stroke="#e6edf1"/>${paths}${cards}${nodeSvg(node,421,cy,true)}</svg></div>${ins.length > pageSize || outs.length > pageSize ? `<div class="pagination">${state.direction === 'out' ? '<span></span>' : paginationHtml(ins,state.inPage,'in')}${state.direction === 'in' ? '<span></span>' : paginationHtml(outs,state.outPage,'out')}</div>` : ''}<div class="map-footer"><span><i class="legend-line"></i>Eingehende Beziehung</span><span><i class="legend-line out"></i>Ausgehende Beziehung</span><span class="explanation">Gestrichelt: Java-Client · Belegzahl anklicken: Schnittstelle${[...ins,...outs].some(isRisk) ? ' · * enthält offene Zuordnungen' : ''}</span></div></section>`;
}
function scopeControls(rawEdges) {
  const counts=EvidenceModel.summarize(EvidenceModel.project(rawEdges,'all',state.kind));
  return `<section class="evidence-controls" aria-label="Belege filtern"><div class="filter-heading"><strong>Belegbasis</strong><span>Produktions- und Testcode werden anhand des Quellpfads unterschieden.</span></div><div class="filter-row"><div class="scope-buttons" role="group" aria-label="Quellcode filtern">${Object.entries(scopeLabels).map(([scope,label])=>`<button data-scope="${scope}" aria-pressed="${state.scope===scope}" class="${state.scope===scope?'active':''}">${label}<span>${counts[scope]}</span></button>`).join('')}</div><label class="kind-select" for="kind-filter">Verbindungsart<select id="kind-filter" aria-label="Verbindungsart">${[['all','Alle Arten'],...Object.entries(kindLabels)].map(([kind,label])=>`<option value="${kind}" ${state.kind===kind?'selected':''}>${label}</option>`).join('')}</select></label></div></section>`;
}
function evidenceStatus(edge) {
  const open=(edge.unresolved||0)+(edge.mismatched||0);
  if(open) return `<span class="pill warn">${open} offen / abweichend</span>`;
  if(edge.unknownStatus) return `<span class="pill neutral">${edge.unknownStatus} nicht einzeln zugeordnet</span>`;
  return '<span class="pill">Zuordnung belegt</span>';
}
function sourceRecord(record) {
  const file=record.file||'';
  const filename=file.split('/').pop();
  return `<li class="source-record"><div><span class="source-scope ${record.scope}">${esc(scopeLabels[record.scope])}</span>${file?`<strong>${esc(filename)}${record.line?':'+record.line:''}</strong>`:'<strong>Einzelnachweis fehlt</strong>'}</div>${file?`<code class="source-path">${esc(file)}${record.line?':'+record.line:''}</code>`:''}${record.evidence?`<details class="raw-evidence"><summary>Originalbeleg</summary><code>${esc(record.evidence)}</code></details>`:''}</li>`;
}
function operationHtml(records) {
  const groups=new Map();
  for(const record of records) {
    const key=[record.kind,record.method||'',record.path||'',record.client||''].join('|');
    if(!groups.has(key)) groups.set(key,[]);
    groups.get(key).push(record);
  }
  return [...groups.values()].map(group=>{
    const first=group[0], count=group.reduce((sum,record)=>sum+record.count,0);
    let title='',description='';
    if(first.kind==='http') {
      title=`<span class="http-method ${esc(first.method.toLowerCase())}">${esc(first.method)}</span><code>${esc(first.path)}</code>`;
      description='HTTP-Operation im Quellcode erkannt. Keine Laufzeitmessung.';
    } else if(first.kind==='java') {
      title=`<span class="connection-type java">Java-Client</span><code>${esc(first.client||'Client-Import')}</code>`;
      description='Import / Injection erkannt. Konkrete HTTP-Methode und Route sind für diese Beziehung nicht belegt.';
    } else {
      title='<span class="connection-type unknown">Nicht bestimmt</span><strong>Keine eindeutige Einzelzuordnung</strong>';
      description='Die Belege sind im ursprünglichen Service-Paar gezählt. Quellcode-Bereich und konkrete Operation bleiben unbekannt.';
    }
    return `<article class="operation"><div class="operation-title">${title}<span class="operation-count">${count} ${count===1?'Beleg':'Belege'}</span></div><p>${description}</p><ul class="source-records">${group.map(sourceRecord).join('')}</ul></article>`;
  }).join('');
}
function tableHtml(edges) {
  return `<section class="evidence-section" id="evidence"><div class="section-title"><h2>Schnittstellen & Belege <span>(${edges.length} Verbindungen)</span></h2><span>Eine Zeile pro gerichtetem Service-Paar.</span></div><div class="table-wrap">${edges.length?`<table><thead><tr><th scope="col">Aufrufer</th><th scope="col">Ziel-Service</th><th scope="col">Verbindungsart</th><th scope="col">Belege</th><th scope="col">Belegbasis</th><th scope="col">Schnittstelle</th></tr></thead><tbody>${edges.map(edge=>{
    const scopes=EvidenceModel.summarize([edge]);
    const basis=[scopes.production?`${scopes.production} Produktion`:'',scopes.test?`${scopes.test} Test`:'',scopes.unknown?`${scopes.unknown} unbekannt`:''].filter(Boolean).join(' / ');
    return `<tr class="relation-row ${state.edge===edge.id?'selected':''}"><td><strong>${esc(name(edge.from))}</strong></td><td>${esc(name(edge.to))}</td><td><span class="connection-type ${edge.records.every(record=>record.kind==='java')?'java':''}">${esc(edgeKind(edge))}</span></td><td><strong>${edge.total}</strong>${edge.total!==edge.originalTotal?`<small class="original-count"> / ${edge.originalTotal}</small>`:''}</td><td>${esc(basis)}</td><td><button class="row-toggle" data-edge="${esc(edge.id)}" aria-expanded="${state.edge===edge.id}" aria-label="Schnittstelle von ${esc(name(edge.from))} zu ${esc(name(edge.to))}">${state.edge===edge.id?'Schließen −':'Ansehen +'}</button></td></tr>`;
  }).join('')}</tbody></table>`:'<div class="empty-block"><strong>Keine passenden Belege</strong><p>Für diese Kombination aus Quellcode, Verbindungsart und Richtung sind keine Belege vorhanden. Die ursprünglichen Beziehungen bleiben erhalten.</p></div>'}</div><p class="data-note">Der Filter gilt für die angezeigten Beziehungen, Zähler und Einzelbelege. „Produktionscode“ bedeutet: Quellpfad außerhalb erkannter Testbereiche. Nicht eindeutig zuordenbare Einträge bleiben unter „Nicht zuordenbar“ sichtbar.</p></section>`;
}
function renderFocus() {
  syncInspectorLayout();
  if(!byId.has(state.selected)){renderOverview();return;}
  const node=byId.get(state.selected),ins=incoming(node.id),outs=outgoing(node.id),edges=[...new Map([...ins,...outs].map(edge=>[edge.id,edge])).values()];
  const rawEdges=data.edges.filter(edge=>edge.from===node.id||edge.to===node.id);
  const visibleEdges=state.direction==='in'?ins:state.direction==='out'?outs:edges;
  const counts=EvidenceModel.summarize(edges);
  document.getElementById('breadcrumb-current').textContent=`${groupLabel(node)} / Service-Fokus`;
  document.getElementById('content').innerHTML=`<div class="page-heading"><div><span class="group-tag">${esc(groupLabel(node))}</span><h1>${esc(node.label)}</h1><p>${esc(node.project)}</p></div><div class="heading-actions">${groupingButton()}<button class="button" data-action="overview">Alle Domänen ansehen</button></div></div>${scopeControls(rawEdges)}<div class="stat-strip"><div class="stat"><strong>${ins.length}</strong><span>Aufrufer</span></div><div class="stat"><strong>${outs.length}</strong><span>Ziel-Services</span></div><div class="stat"><strong data-evidence-total>${totals(edges)}</strong><span>von ${totals(rawEdges)} Belegen</span></div><div class="stat-caption"><span class="connection-type">${counts.http} HTTP</span> <span class="connection-type java">${counts.java} Java-Client</span>${counts.unknown?` <span class="connection-type unknown">${counts.unknown} ungeklärte Herkunft</span>`:''}</div></div><div class="workspace-split"><div class="workspace-primary">${graphHtml(node,ins,outs)}${tableHtml(visibleEdges)}</div>${inspectorHtml()}</div><p class="data-note">Gruppierungsquelle: ${esc(sourceLabel(node))} · ${esc(node.domain)}. Abdeckung: ${esc(window.WORKSPACE_DATA.health.coverage||'unbekannt')}; Aktualität: ${esc(window.WORKSPACE_DATA.health.freshness||'unbekannt')}.</p>`;
}

function renderOverview() {
  syncInspectorLayout();
  const edges=projectedEdges(),pairs=EvidenceModel.domainPairs(edges,data.nodes);
  document.getElementById('breadcrumb-current').textContent='Domänenübersicht';
  document.getElementById('content').innerHTML=`<div class="page-heading"><div><span class="group-tag">Workspace</span><h1>Architektur nach Domänen</h1><p>Von der Domänenbeziehung zum Service-Paar und seinem konkreten Nachweis.</p></div><div class="heading-actions">${groupingButton()}<button class="button" data-node="${esc(state.selected)}">Zum Service-Fokus</button></div></div>${scopeControls(data.edges)}<div class="stat-strip"><div class="stat"><strong>${data.nodes.length}</strong><span>Projekte</span></div><div class="stat"><strong>${groups.length}</strong><span>unveränderte Gruppen</span></div><div class="stat"><strong>${edges.length}</strong><span>Service-Paare im Filter</span></div><div class="stat-caption">${pairs.length} gerichtete Domänenpaare<br>${totals(edges)} statische Belege</div></div><div class="overview-view-tabs segmented" aria-label="Domänenansicht"><button data-overview-mode="relations" aria-pressed="${state.overviewMode==='relations'}" class="${state.overviewMode==='relations'?'active':''}">Verbindungen</button><button data-overview-mode="services" aria-pressed="${state.overviewMode==='services'}" class="${state.overviewMode==='services'?'active':''}">Services nach Domäne</button></div><div class="workspace-split"><div class="workspace-primary">${state.overviewMode==='relations'?domainMatrixHtml(edges):overviewServicesHtml(edges)}</div>${inspectorHtml()}</div><p class="data-note">Alle ursprünglichen Gruppenzuordnungen bleiben erhalten. Die Matrix aggregiert ausschließlich die vorhandenen gerichteten Service-Paare; fehlende Belege beweisen keine Unabhängigkeit.</p>`;
}
function render() {renderNav();if(window.renderWorkspaceArea&&window.renderWorkspaceArea())return;state.view === 'overview' ? renderOverview() : renderFocus();}
function selectNode(id) {
  if(!byId.has(id)) return;
  if(window.resetWorkspaceSelection)window.resetWorkspaceSelection();
  state.selected=id;state.view='focus';state.edge=null;state.domainPair=null;state.inPage=0;state.outPage=0;state.direction='both';render();window.scrollTo({top:0});
}
function handleAction(target) {
  const pair=target.closest('[data-domain-pair]');
  if(pair){state.domainPair=pair.dataset.domainPair;state.edge=null;state.panelReturn={attr:'data-domain-pair',value:state.domainPair};openInspector();return;}
  const mode=target.closest('[data-overview-mode]');
  if(mode){state.overviewMode=mode.dataset.overviewMode;state.edge=null;state.domainPair=null;render();return;}
  const panelNode=target.closest('[data-panel-node]');
  if(panelNode){selectNode(panelNode.dataset.panelNode);return;}
  const node=target.closest('[data-node]');
  if(node){selectNode(node.dataset.node);return;}
  const edge=target.closest('[data-edge]');
  if(edge){state.edge=edge.dataset.edge;state.panelReturn={attr:'data-edge',value:state.edge};openInspector();return;}
  const scope=target.closest('[data-scope]');
  if(scope){state.scope=scope.dataset.scope;state.inPage=0;state.outPage=0;render();document.querySelector('[data-scope="'+state.scope+'"]').focus({preventScroll:true});return;}
  const direction=target.closest('[data-direction]');
  if(direction){state.direction=direction.dataset.direction;state.edge=null;renderFocus();return;}
  const page=target.closest('[data-page]');
  if(page&&!page.disabled){state[page.dataset.page+'Page']+=Number(page.dataset.delta);renderFocus();return;}
  const action=target.closest('[data-action]')?.dataset.action;
  if(action==='grouping'){document.getElementById('grouping-evidence').innerHTML=data.nodes.map(node=>`<div class="mapping-row"><strong>${esc(node.label)}</strong><span>${esc(groupLabel(node))}<br>${esc(sourceLabel(node))} · ${esc(node.domain)}</span></div>`).join('');document.getElementById('group-dialog').showModal();}
  if(action==='overview') showOverview();
  if(action==='pair-back'){state.edge=null;state.panelReturn={attr:'data-domain-pair',value:state.domainPair};render();document.querySelector('.inspector')?.focus({preventScroll:true});}
  if(action==='close-panel') closeInspector();
}
function closeInspector() {
  const destination=state.panelReturn;
  state.edge=null;state.domainPair=null;render();
  if(destination){const candidates=[...document.querySelectorAll('['+destination.attr+']')];candidates.find(element=>element.getAttribute(destination.attr)===destination.value)?.focus({preventScroll:true});}
}

document.addEventListener('click', event=>handleAction(event.target));
document.addEventListener('change', event=>{if(event.target.id==='kind-filter'){state.kind=event.target.value;state.inPage=0;state.outPage=0;render();document.getElementById('kind-filter').focus({preventScroll:true});}});
document.addEventListener('keydown', event=>{if(event.key==='Escape'&&!document.getElementById('group-dialog').open&&(state.edge||state.domainPair)){event.preventDefault();closeInspector();return;}if((event.key === 'Enter'||event.key === ' ') && event.target.matches('g[role="button"]')){event.preventDefault();handleAction(event.target);}});
document.getElementById('search').addEventListener('input',event=>{state.query=event.target.value;renderNav();});
document.getElementById('overview-nav').addEventListener('click',showOverview);
document.getElementById('home').addEventListener('click',event=>{event.preventDefault();showOverview();});
document.getElementById('close-dialog').addEventListener('click',()=>document.getElementById('group-dialog').close());
document.getElementById('snapshot').textContent='Datenstand '+(data.generated?new Date(data.generated).toLocaleString('de-DE',{dateStyle:'medium',timeStyle:'short'}):'unbekannt');
function domainMatrixHtml(edges) {
  const pairs=EvidenceModel.domainPairs(edges,data.nodes);
  const byPair=new Map(pairs.map(pair=>[pair.id,pair]));
  const strongest=Math.max(1,...pairs.map(pair=>pair.edges.length));
  return `<section class="domain-matrix-panel" aria-labelledby="matrix-title"><div class="matrix-heading"><h2 id="matrix-title">Wer greift auf welche Domäne zu?</h2><p>Eine Zahl zählt gerichtete Service-Paare. Anklicken öffnet die beteiligten Services.</p></div><div class="domain-matrix-scroll"><table class="domain-matrix" style="min-width:${Math.max(800,groups.length*86+125)}px"><caption class="sr-only">Aufrufende Domänen in den Zeilen; Ziel-Domänen in den Spalten.</caption><thead><tr><th scope="col" class="matrix-corner">Aufrufer ↓<br>Ziel →</th>${groups.map(group=>`<th scope="col" title="${esc(group)}">${esc(domainNames.get(group)||group)}</th>`).join('')}</tr></thead><tbody>${groups.map(from=>`<tr><th scope="row" title="${esc(from)}">${esc(domainNames.get(from)||from)}</th>${groups.map(to=>{
    const id=JSON.stringify([from,to]),pair=byPair.get(id),selected=state.domainPair===id;
    if(!pair) return `<td class="matrix-empty ${from===to?'diagonal':''}" aria-label="${esc(domainNames.get(from))} an ${esc(domainNames.get(to))}: keine Belege im aktuellen Filter">–</td>`;
    return `<td class="${from===to?'diagonal':''}"><button class="domain-cell ${selected?'selected':''}" data-domain-pair="${esc(id)}" aria-pressed="${selected}" aria-label="${esc(domainNames.get(from))} an ${esc(domainNames.get(to))}: ${pair.edges.length} Service-Paare" title="${pair.callers} Aufrufer, ${pair.targets} Ziele, ${pair.total} Belege" style="--strength:${(.12+pair.edges.length/strongest*.35).toFixed(2)}"><strong>${pair.edges.length}</strong><small>${pair.total} Belege</small></button></td>`;
  }).join('')}</tr>`).join('')}</tbody></table></div><footer><span><i class="diagonal-key"></i>Diagonale: Beziehungen innerhalb derselben Domäne</span><span>– Keine Belege im aktuellen Filter</span></footer></section>`;
}
function overviewServicesHtml(edges) {
  return `<div class="overview-grid">${groups.map(group=>{
    const nodes=data.nodes.filter(node=>node.domain===group).sort((a,b)=>(a.architecture_order??Number.MAX_SAFE_INTEGER)-(b.architecture_order??Number.MAX_SAFE_INTEGER)||a.label.localeCompare(b.label));
    const ids=new Set(nodes.map(node=>node.id));
    const related=edges.filter(edge=>ids.has(edge.from)||ids.has(edge.to));
    return `<section class="domain-card"><header><h2>${esc(domainNames.get(group)||group)}</h2><p>${nodes.length} Projekte · ${related.length} Service-Paare im Filter</p></header><ul>${nodes.map(node=>`<li><button data-node="${esc(node.id)}"><strong>${esc(node.label)}</strong><span>${edges.filter(edge=>edge.to===node.id).length} ein / ${edges.filter(edge=>edge.from===node.id).length} aus</span></button></li>`).join('')}</ul><footer>${esc(group)}</footer></section>`;
  }).join('')}</div>`;
}
function interfaceContent(edge) {
  const scopes=EvidenceModel.summarize([edge]);
  return `<div class="interface-heading"><div><span class="detail-eyebrow">Gerichtete Service-Verbindung</span><h3>${esc(name(edge.from))}<span aria-label="nach"> → </span>${esc(name(edge.to))}</h3></div></div>${evidenceStatus(edge)}<div class="detail-summary"><span>${scopes.production} Produktionscode</span><span>${scopes.test} Testcode</span>${scopes.unknown?`<span>${scopes.unknown} nicht zuordenbar</span>`:''}<span>${edge.total} von ${edge.originalTotal} Belegen sichtbar</span></div><div class="panel-service-actions"><button class="button" data-panel-node="${esc(edge.from)}">Aufrufer öffnen</button><button class="button" data-panel-node="${esc(edge.to)}">Ziel öffnen</button></div>${operationHtml(edge.records)}`;
}
function inspectorHtml() {
  if(!state.edge&&!state.domainPair) return '';
  const edges=projectedEdges();
  const edge=edges.find(candidate=>candidate.id===state.edge);
  const pair=EvidenceModel.domainPairs(edges,data.nodes).find(candidate=>candidate.id===state.domainPair);
  let content='';
  if(state.edge) {
    const back=state.domainPair?'<button class="panel-back" data-action="pair-back">‹ Zurück zu den Service-Paaren</button>':'';
    content=back+(edge?interfaceContent(edge):'<div class="empty-block">Für diese Verbindung gibt es im aktuellen Filter keine Belege.</div>');
  } else if(pair) {
    content=`<div class="domain-pair-title"><span class="detail-eyebrow">Domänenbeziehung</span><h3>${esc(domainNames.get(pair.from))} <span>→</span> ${esc(domainNames.get(pair.to))}</h3><p>${pair.from===pair.to?'Innerhalb dieser Domäne':'Zwischen diesen beiden Domänen'}</p></div><div class="pair-metrics"><div><strong>${pair.callers}</strong><span>Aufrufer</span></div><div><strong>${pair.targets}</strong><span>Ziel-Services</span></div><div><strong>${pair.edges.length}</strong><span>Service-Paare</span></div></div><p class="pair-evidence-total">${pair.total} statische Belege im aktuellen Filter</p><div class="pair-services">${pair.edges.slice().sort((a,b)=>name(a.from).localeCompare(name(b.from))||name(a.to).localeCompare(name(b.to))).map(item=>`<button class="pair-service" data-edge="${esc(item.id)}" aria-label="Schnittstelle von ${esc(name(item.from))} zu ${esc(name(item.to))}"><span class="pair-route"><strong>${esc(name(item.from))}</strong><span>→ ${esc(name(item.to))}</span></span><span class="pair-meta">${esc(edgeKind(item))}<b>${item.total} Belege</b></span></button>`).join('')}</div>`;
  } else {
    content='<div class="empty-block"><strong>Keine Belege im aktuellen Filter</strong><p>Andere Filter können Beziehungen für dieses Domänenpaar enthalten.</p></div>';
  }
  return `<aside class="inspector" aria-labelledby="inspector-title" tabindex="-1"><header class="inspector-header"><h2 id="inspector-title">${state.edge?'Schnittstelle & Nachweise':'Verbindung im Detail'}</h2><button data-action="close-panel" class="close-panel" aria-label="Detailpanel schließen">×</button></header><div class="inspector-body">${content}</div><footer>Statischer Datenstand · keine Laufzeitmessung</footer></aside>`;
}
function syncInspectorLayout() {
  document.querySelector('.shell').classList.toggle('with-inspector',!!(state.edge||state.domainPair));
}
function showOverview() {
  state.area='architecture';
  state.view='overview';state.edge=null;state.domainPair=null;render();window.scrollTo({top:0});
}
function openInspector() {
  const main=document.getElementById('main');
  const previousTop=window.scrollY;
  render();
  const panel=document.querySelector('.inspector');
  if(innerWidth<1100) panel?.scrollIntoView({block:'start'});
  else window.scrollTo({top:Math.min(previousTop,main.offsetTop+main.querySelector('.workspace-split').offsetTop)});
  document.querySelector('[data-action="close-panel"]')?.focus({preventScroll:true});
}

render();
