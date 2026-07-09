package scan

import (
	"bytes"
	"encoding/json"
	"html"
	"strconv"
	"strings"
)

func RenderWorkspaceDashboardHTML(graph WorkspaceGraphRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string {
	registry := workspaceRegistryFromGraph(graph)
	serviceMap := BuildWorkspaceServiceMap(registry, matches, nil, nil)
	endpointTraces := BuildWorkspaceEndpointTraces(matches, nil, dossiers)
	return RenderWorkspaceDashboardHTMLWithModels(graph, serviceMap, endpointTraces, matches, dossiers)
}

func RenderWorkspaceDashboardHTMLWithModels(graph WorkspaceGraphRecord, serviceMap WorkspaceServiceMapRecord, endpointTraces WorkspaceEndpointTraceIndexRecord, matches []WorkspaceContractMatchRecord, dossiers []FeatureDossierRecord) string {
	payloadValue := struct {
		Graph          WorkspaceGraphRecord              `json:"graph"`
		ServiceMap     WorkspaceServiceMapRecord         `json:"service_map"`
		EndpointTraces WorkspaceEndpointTraceIndexRecord `json:"endpoint_traces"`
		SourceIndex    []dashboardSourceRecord           `json:"source_index,omitempty"`
	}{Graph: graph, ServiceMap: serviceMap, EndpointTraces: endpointTraces, SourceIndex: buildDashboardSourceIndex(endpointTraces)}
	payload := marshalDashboardPayload(payloadValue)
	title := "GoreGraph Workspace Map"
	if graph.Root != "" {
		title += " - " + graph.Root
	}
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</title>
<style>
:root{color-scheme:light;--bg:#f3f6f7;--panel:#fff;--canvas:#eef4f6;--line:#d3dde4;--line-dark:#98a8b4;--text:#17212b;--muted:#5f6f7e;--accent:#0b6b79;--accent-soft:#dcecef;--ok:#287a4b;--warn:#a56a00;--risk:#a33131;font-family:Avenir Next,Segoe UI,Helvetica,Arial,sans-serif}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text)}body,.shell,main,svg{user-select:none}input,.details,.node-row,.relation-row{user-select:text}button,input{font:inherit}.shell{display:grid;grid-template-columns:420px minmax(760px,1fr) 480px;min-height:100vh}.side,.details{background:var(--panel);padding:20px;overflow:auto}.side{border-right:1px solid var(--line)}.details{border-left:1px solid var(--line);font-size:15px}
h1{font-size:20px;line-height:1.2;margin:0 0 14px}.details h1{font-size:24px;line-height:1.14;overflow-wrap:anywhere}h2{font-size:12px;line-height:1.2;margin:18px 0 8px;color:var(--muted);text-transform:uppercase}.summary{display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px;margin:12px 0 16px}.metric{border:1px solid var(--line);border-radius:6px;padding:9px}.metric strong{display:block;font-size:20px}.metric span{font-size:12px;color:var(--muted)}
input{width:100%;height:40px;border:1px solid var(--line);border-radius:6px;padding:0 11px;background:#fff;color:var(--text)}.filters,.modes,.canvas-tools{display:flex;flex-wrap:wrap;gap:7px}.filters button,.modes button,.canvas-tools button{height:32px;border:1px solid var(--line);border-radius:6px;background:#fff;color:var(--text);padding:0 10px;cursor:pointer}.filters button.active,.modes button.active,.canvas-tools button.active{background:var(--accent);border-color:var(--accent);color:#fff}
.item-list{display:grid;gap:7px;margin-top:12px}.node-row{width:100%;border:1px solid var(--line);border-radius:6px;background:#fff;text-align:left;padding:10px;cursor:pointer}.node-row strong{display:block;font-size:13px;line-height:1.25;overflow-wrap:anywhere}.node-row span{display:block;font-size:12px;color:var(--muted);margin-top:4px;overflow-wrap:anywhere}.node-row.selected{border-color:var(--accent);box-shadow:inset 3px 0 0 var(--accent)}
.result-note,.help{font-size:13px;color:var(--muted);line-height:1.45;margin:10px 0 0}.glossary{border-top:1px solid var(--line);margin-top:16px;padding-top:12px}.glossary p{font-size:12px;line-height:1.35;color:var(--muted);margin:5px 0}.glossary strong{color:var(--text)}main{position:relative;overflow:hidden;background:var(--canvas)}.canvas-tools{position:absolute;z-index:2;top:12px;left:12px}.canvas-tools .readout{height:32px;border:1px solid var(--line);border-radius:6px;background:#fff;padding:6px 10px;font-size:13px;color:var(--muted)}
svg{width:100%;height:100vh;display:block;background:var(--canvas);cursor:grab}.lane-title{font-size:12px;fill:var(--muted);font-weight:700;text-transform:uppercase}.domain-title{font-size:14px;fill:var(--text);font-weight:700}.edge{stroke:#8fa2ae;stroke-width:1.35;fill:none;opacity:.36}.edge.focused{stroke:var(--accent);stroke-width:2.8;opacity:.95}.edge.warn{stroke:var(--warn)}.edge.risk{stroke:var(--risk)}.edge-label{font-size:11px;fill:var(--muted);paint-order:stroke;stroke:var(--canvas);stroke-width:4;stroke-linejoin:round}.service-node rect,.trace-step rect,.raw-node rect{fill:#fff;stroke:var(--line);stroke-width:1.2;rx:6}.service-node.selected rect,.trace-step.selected rect,.raw-node.selected rect{stroke:var(--accent);stroke-width:2.2}.service-node.dim,.trace-step.dim,.raw-node.dim{opacity:.35}.raw-node circle{fill:#fff;stroke:var(--line);stroke-width:1.5}.raw-node.focused circle,.raw-node.selected circle{stroke:var(--accent);stroke-width:3}.service-title,.trace-title,.raw-title{font-size:13px;font-weight:700;fill:var(--text)}.service-meta,.trace-meta,.raw-meta{font-size:11px;fill:var(--muted)}.trace-step.frontend_step rect,.trace-step.api_contract rect{fill:#fefcf7}.trace-step.backend_route rect,.trace-step.backend_handler rect,.trace-step.backend_step rect{fill:#f7fbf8}.trace-step.test rect{fill:#f7f8fc}
.trace-step.focused rect,.raw-node.focused rect{stroke:var(--accent);stroke-width:3}.issue-card rect{fill:#fff;stroke:var(--line);stroke-width:1.2;rx:6}.issue-card.selected rect{stroke:var(--accent);stroke-width:2.2}.issue-title{font-size:14px;font-weight:700;fill:var(--text)}.issue-meta{font-size:12px;fill:var(--muted)}.source-link{color:var(--accent);text-decoration:none}.source-link:hover{text-decoration:underline}.empty{color:var(--muted);font-size:14px;line-height:1.55}.detail-list{display:grid;gap:11px}.detail-item{border-top:1px solid var(--line);padding-top:11px;font-size:14px;line-height:1.42;overflow-wrap:anywhere}.detail-item strong{display:block;font-size:15px;color:var(--text);margin-bottom:2px}.detail-item small{display:block;color:var(--muted);font-size:13px;line-height:1.35}.relation-section{border-top:1px solid var(--line);padding-top:12px;margin-top:12px}.relation-section h3{font-size:16px;margin:0 0 8px}.relation-row{display:block;width:100%;text-align:left;border:1px solid var(--line);border-radius:6px;background:#fff;padding:9px;margin:6px 0;cursor:pointer}.relation-row strong{font-size:14px}.relation-row span{display:block;color:var(--muted);font-size:13px;margin-top:2px;overflow-wrap:anywhere}.relation-row.selected{border-color:var(--accent);box-shadow:inset 3px 0 0 var(--accent)}.badge{display:inline-block;border:1px solid var(--line);border-radius:5px;padding:2px 6px;font-size:12px;color:var(--muted);margin:3px 4px 0 0}.badge.ok{color:var(--ok);border-color:#9ac9ad}.badge.warn{color:var(--warn);border-color:#d6b779}.badge.risk{color:var(--risk);border-color:#d99b9b}
@media (max-width:1180px){.shell{grid-template-columns:1fr}.side,.details{border:0;border-bottom:1px solid var(--line)}svg{height:72vh}}
</style>
</head>
<body>
<div class="shell">
<aside class="side">
<h1>GoreGraph Workspace</h1>
<div class="summary"><div class="metric"><strong id="service-count">0</strong><span>services</span></div><div class="metric"><strong id="edge-count">0</strong><span>relations</span></div><div class="metric"><strong id="trace-count">0</strong><span>traces</span></div></div>
<h2>View</h2>
<div class="modes">
<button data-view-mode="issues" class="active">Open Issues</button>
<button data-view-mode="architecture">Architecture Map</button>
<button data-view-mode="services">Focused Service</button>
<button data-view-mode="endpoint">Endpoint Trace</button>
<button data-view-mode="raw">Endpoint Paths</button>
</div>
<p class="help" id="mode-help">Open Issues groups unresolved, mismatched, and out-of-scope contracts by cause so duplicates can be triaged as one problem family.</p>
<input id="workspace-search" aria-label="Search workspace map" placeholder="Search service, endpoint, route, file, symbol">
<h2>Filter</h2>
<div class="filters">
<button data-kind-filter="all" class="active">All</button>
<button data-kind-filter="risk">Risk</button>
<button data-kind-filter="resolved">Resolved</button>
<button data-kind-filter="unresolved">Unresolved</button>
</div>
<div class="filters" style="margin-top:8px"><button id="clear-selection" type="button">Clear selection</button></div>
<h2 id="list-title">Services</h2>
<div id="node-list" class="item-list"></div>
<p id="result-note" class="result-note"></p>
<div class="glossary">
<h2>Status glossary</h2>
<p><strong>RESOLVED</strong>: frontend contract is mapped to a backend route.</p>
<p><strong>MISMATCH</strong>: a related backend route exists, but method or path does not match exactly.</p>
<p><strong>UNRESOLVED</strong>: no reliable backend route was found in indexed projects.</p>
<p><strong>OUT_OF_SCOPE</strong>: intentionally not a backend service, for example a frontend-internal API.</p>
<p><strong>EXTRACTED</strong>: read directly from source structure. <strong>MATCHED</strong>: linked to a concrete route, symbol, or test.</p>
</div>
</aside>
<main>
<div class="canvas-tools">
<button id="zoom-out" title="Zoom out">-</button>
<button id="zoom-in" title="Zoom in">+</button>
<button id="reset-view" title="Reset zoom and pan">100%</button>
<button id="fit-button" title="Fit visible graph">Fit</button>
<button id="toggle-labels" title="Toggle labels">Labels</button>
<span id="zoom-readout" class="readout">100%</span>
</div>
<svg id="workspace-graph" role="img" aria-label="Directed workspace relationship map"><defs><marker id="arrow" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#8fa2ae"></path></marker><marker id="arrow-focus" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#0b6b79"></path></marker></defs><g id="graph-layer"></g></svg>
</main>
<section class="details" id="details"><p class="empty">Select a service or endpoint to inspect directed relationships.</p></section>
</div>
<script>
const workspacePayload = `)
	b.Write(payload)
	b.WriteString(`;
const workspaceGraph = workspacePayload.graph||{nodes:[],edges:[]};
const serviceMap = workspacePayload.service_map||{nodes:[],edges:[]};
const endpointTraces = workspacePayload.endpoint_traces||{traces:[]};
const graphNodes=workspaceGraph.nodes||[];
const graphEdges=workspaceGraph.edges||[];
const serviceNodes=serviceMap.nodes||[];
const serviceEdges=serviceMap.edges||[];
const traces=endpointTraces.traces||[];
const serviceById=new Map(serviceNodes.map(function(n){return [n.id,n];}));
const traceById=new Map(traces.map(function(t){return [t.id,t];}));
const rawById=new Map(graphNodes.map(function(n){return [n.id,n];}));
const state={mode:"issues",query:"",filter:"all",selected:null,zoom:1,panX:0,panY:0,labels:false,drag:null,dragMoved:false,positions:new Map()};
document.getElementById("service-count").textContent=serviceNodes.length;
document.getElementById("edge-count").textContent=serviceEdges.length;
document.getElementById("trace-count").textContent=traces.length;
function escapeHtml(v){return String(v||"").replace(/[&<>"']/g,function(c){return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c];});}
function escapeAttr(v){return escapeHtml(v);}
function shortLabel(v,max){v=String(v||"");max=max||58;return v.length>max?v.slice(0,max-3)+"...":v;}
function includesText(values){const q=state.query.toLowerCase();return !q||values.join(" ").toLowerCase().includes(q);}
function routeStatusClass(value){value=String(value||"").toLowerCase();if(value.includes("mismatch")||value.includes("risk"))return "risk";if(value.includes("unresolved")||value.includes("missing"))return "warn";return "ok";}
function serviceRole(n){const role=String(n.role||"").toLowerCase();if(role)return role;const p=String(n.project||"").toLowerCase();const k=String(n.kind||"").toLowerCase();if(k==="frontend"||p.startsWith("frontend/")||p.includes("frontend")||p.includes("playwright"))return "frontend";if(k==="backend"||p.startsWith("microservices/")||p.startsWith("services/"))return "backend";return "internal";}
function serviceDomain(n){const domain=String(n.domain||"").toLowerCase();if(domain)return domain;const text=String((n.project||"")+" "+(n.label||"")+" "+(n.service||"")).toLowerCase();if(serviceRole(n)==="frontend")return "frontend";if(text.includes("document")||text.includes("container")||text.includes("topic"))return "document";if(text.includes("cadaster")||text.includes("regulation"))return "cadaster";if(text.includes("user")||text.includes("license")||text.includes("product")||text.includes("invoice")||text.includes("shop"))return "identity";return "platform";}
function filteredServiceEdges(){return serviceEdges.filter(function(e){if(state.filter==="risk"&&!(e.mismatched||e.unresolved))return false;if(state.filter==="resolved"&&!e.resolved)return false;if(state.filter==="unresolved"&&!e.unresolved)return false;return includesText([e.from_project,e.to_project,e.direction,(e.endpoints||[]).join(" "),e.risk]);});}
function visibleServices(){const edges=filteredServiceEdges();const edgeSet=new Set();edges.forEach(function(e){edgeSet.add(e.from);edgeSet.add(e.to);});return serviceNodes.filter(function(n){const keepByMode=state.filter==="all"?(edgeSet.has(n.id)||serviceRole(n)==="frontend"||state.mode==="architecture"||state.query):edgeSet.has(n.id);return keepByMode&&includesText([n.label,n.project,n.service,n.kind,n.status,n.role,n.domain]);}).slice(0,260);}
function visibleTraces(){return traces.filter(function(t){if(state.filter==="risk"&&!(t.risk&&t.risk!=="resolved"))return false;if(state.filter==="resolved"&&t.status!=="RESOLVED")return false;if(state.filter==="unresolved"&&t.status==="RESOLVED")return false;return includesText([t.route,t.from_project,t.to_project,t.status,t.risk,(t.steps||[]).map(function(s){return s.label+" "+s.file;}).join(" ")]);}).slice(0,180);}
function isOpenIssueTrace(t){const status=String(t.status||"").toUpperCase();const risk=String(t.risk||"").toLowerCase();if(status&&status!=="RESOLVED")return true;return risk==="frontend_internal_api"||risk==="dynamic_endpoint_unresolved"||risk==="indexed_backend_route_missing"||risk==="method_mismatch";}
function issueGroupKey(t){const path=String(t.path||t.route||"").toLowerCase();if(path.startsWith("/tree/")||path.includes(" /tree/"))return "tree-prefix|"+(t.to_project||"unresolved");return [t.risk||t.status||"issue",t.to_project||"unresolved",t.method||"",t.path||t.route||""].join("|");}
function issueGroupTitle(t){const path=String(t.path||t.route||"");if(path.startsWith("/tree/")||path.includes(" /tree/"))return "RDBV /tree/* -> "+(t.to_project||"unresolved");return (t.risk||t.status||"issue")+" - "+(t.method||"")+" "+(t.path||t.route||"");}
function buildIssueGroups(){const map=new Map();traces.filter(isOpenIssueTrace).filter(function(t){return includesText([t.route,t.from_project,t.to_project,t.status,t.risk]);}).forEach(function(t){const key=issueGroupKey(t);let group=map.get(key);if(!group){group={id:"issue:"+key,title:issueGroupTitle(t),risk:t.risk||t.status||"issue",to:t.to_project||"unresolved",routes:[],projects:new Set(),traces:[]};map.set(key,group);}group.traces.push(t);group.projects.add(t.from_project||"unknown");if(!group.routes.includes(t.route))group.routes.push(t.route);});return Array.from(map.values()).map(function(g){g.projectList=Array.from(g.projects).sort();return g;}).sort(function(a,b){if(b.traces.length!==a.traces.length)return b.traces.length-a.traces.length;return a.title.localeCompare(b.title);});}
function visibleRawNodes(){return graphNodes.filter(function(n){return includesText([n.label,n.project,n.file,n.symbol,n.method,n.path,n.kind,n.risk]);}).slice(0,180);}
function itemButton(id,title,meta){const selected=state.selected===id?" selected":"";return '<button class="node-row'+selected+'" data-select-id="'+escapeAttr(id)+'"><strong>'+escapeHtml(title)+'</strong><span>'+escapeHtml(meta)+'</span></button>';}
function serviceListMeta(n){const status=(n.outgoing||n.incoming) ? (n.outgoing||0)+" outgoing / "+(n.incoming||0)+" incoming" : "Scanned, no outgoing API calls detected";return status+" / "+n.project;}
function serviceBoxMeta(n){return (n.outgoing||0)+" outgoing | "+(n.incoming||0)+" incoming";}
function renderList(){let html="",count=0,title="Services";if(state.mode==="issues"){const groups=buildIssueGroups();count=groups.length;html=groups.map(function(g){return itemButton(g.id,g.title,g.traces.length+" endpoints / "+g.risk+" / "+g.projectList.join(", "));}).join("");title="Open Issues";}else if(state.mode==="services"||state.mode==="architecture"||state.mode==="raw"){const nodes=visibleServices();count=nodes.length;html=nodes.map(function(n){return itemButton(n.id,n.label||n.project,serviceListMeta(n));}).join("");title=state.mode==="architecture"?"Architecture Services":state.mode==="raw"?"Endpoint Path Services":"Services";}else if(state.mode==="endpoint"){const rows=visibleTraces();count=rows.length;html=rows.map(function(t){return itemButton(t.id,t.route,(t.from_project||"unknown")+" -> "+(t.to_project||"unresolved")+" / "+(t.status||"UNKNOWN"));}).join("");title="Endpoints";}document.getElementById("list-title").textContent=title;document.getElementById("node-list").innerHTML=html||"<p class='empty'>No matching items.</p>";document.getElementById("result-note").textContent="Showing "+count+" "+title.toLowerCase()+".";document.querySelectorAll("[data-select-id]").forEach(function(el){el.addEventListener("click",function(){selectOrToggleItem(el.dataset.selectId);});});}
function setViewBox(width,height){document.getElementById("workspace-graph").setAttribute("viewBox","0 0 "+Math.max(width,900)+" "+Math.max(height,620));}
function serviceLayout(nodes,width){const left=[],right=[],middle=[];nodes.forEach(function(n){const role=serviceRole(n);if(role==="frontend")left.push(n);else if(role==="backend")right.push(n);else middle.push(n);});const lanes=[left,middle,right];const xs=[90,Math.max(540,width/2-170),Math.max(940,width-430)];const positions=new Map();let maxLen=0;lanes.forEach(function(lane,i){maxLen=Math.max(maxLen,lane.length);lane.forEach(function(n,index){positions.set(n.id,{x:xs[i],y:104+index*118,lane:i,w:360,h:78});});});return {positions:positions,height:Math.max(660,190+maxLen*118),xs:xs};}
function architectureLayout(nodes,width){const domains=["frontend","document","cadaster","identity","platform"];const labels={frontend:"Frontend clients",document:"Documents / WPO",cadaster:"Cadaster / Regulation",identity:"Identity / Commerce",platform:"Platform / Internal"};const columns=domains.map(function(domain){return nodes.filter(function(n){return serviceDomain(n)===domain;});});const usable=Math.max(width,1700)-180;const step=usable/Math.max(1,domains.length-1);const positions=new Map();let maxLen=0;columns.forEach(function(column,i){maxLen=Math.max(maxLen,column.length);column.forEach(function(n,index){positions.set(n.id,{x:90+i*step,y:126+index*118,lane:i,w:360,h:78});});});return {positions:positions,height:Math.max(780,220+maxLen*118),domains:domains,labels:labels,xs:domains.map(function(_,i){return 90+i*step;})};}
function truncateWord(word,maxChars){word=String(word||"");return word.length>maxChars?shortLabel(word,maxChars):word;}
function wrapSvgText(value,maxChars,maxLines){const words=String(value||"").split(/\s+/).filter(Boolean).map(function(word){return truncateWord(word,maxChars);});const lines=[];let line="";words.forEach(function(word){const next=line?line+" "+word:word;if(next.length>maxChars&&line){lines.push(line);line=word;}else{line=next;}});if(line)lines.push(line);if(!lines.length)lines.push("");if(lines.length>maxLines){lines.length=maxLines;lines[maxLines-1]=shortLabel(lines[maxLines-1],maxChars);}return lines;}
function svgTextBlock(cls,x,y,value,maxChars,maxLines,lineHeight){return wrapSvgText(value,maxChars,maxLines).map(function(line,i){return '<text class="'+cls+'" x="'+x+'" y="'+(y+i*lineHeight)+'">'+escapeHtml(line)+'</text>';}).join("");}
function boxNode(cls,id,x,y,w,h,title,meta,selected,dim){const titleCls=cls.includes("trace")?"trace-title":cls.includes("raw")?"raw-title":"service-title";const metaCls=cls.includes("trace")?"trace-meta":cls.includes("raw")?"raw-meta":"service-meta";let html='<g class="'+cls+(selected?' selected':'')+(dim?' dim':'')+'" data-select-id="'+escapeAttr(id)+'" data-focus-id="'+escapeAttr(id)+'"><title>'+escapeHtml(title+" / "+meta)+'</title><rect x="'+x+'" y="'+y+'" width="'+w+'" height="'+h+'"></rect>';html+=svgTextBlock(titleCls,x+14,y+24,title,Math.max(18,Math.floor((w-28)/8)),2,16);html+=svgTextBlock(metaCls,x+14,y+h-26,meta,Math.max(18,Math.floor((w-28)/7)),1,14);html+='</g>';return html;}
function curvedPath(a,b){const ax=a.x+a.w,ay=a.y+a.h/2,bx=b.x,by=b.y+b.h/2,mid=(ax+bx)/2;return "M"+ax+" "+ay+" C"+mid+" "+ay+" "+mid+" "+by+" "+bx+" "+by;}
function renderServiceMap(){const svg=document.getElementById("workspace-graph");const layer=document.getElementById("graph-layer");const width=svg.clientWidth||1200;const allNodes=visibleServices();const focused=state.selected?serviceFocus(state.selected):null;const nodes=focused?allNodes.filter(function(n){return focused.has(n.id);}):allNodes;const nodeIds=new Set(nodes.map(function(n){return n.id;}));const layout=serviceLayout(nodes,width);state.positions=layout.positions;setViewBox(width,layout.height);let body='<rect x="0" y="0" width="'+Math.max(width,1100)+'" height="'+layout.height+'" fill="transparent"></rect><text class="lane-title" x="90" y="42">Focused Service View</text><text class="raw-meta" x="90" y="66">Select a service to isolate incoming and outgoing relationships. Select it again to clear.</text>';["Frontend clients","Internal services","Backend services"].forEach(function(label,i){body+='<text class="domain-title" x="'+layout.xs[i]+'" y="94">'+label+'</text>';});filteredServiceEdges().forEach(function(e){if(!nodeIds.has(e.from)||!nodeIds.has(e.to))return;const a=layout.positions.get(e.from),c=layout.positions.get(e.to);if(!a||!c)return;const isFocused=state.selected&&(e.from===state.selected||e.to===state.selected);body+='<path class="edge '+routeStatusClass(e.risk)+(isFocused?' focused':'')+'" marker-end="url(#'+(isFocused?'arrow-focus':'arrow')+')" d="'+curvedPath(a,c)+'"></path>';if(isFocused&&state.labels){body+='<text class="edge-label" x="'+((a.x+c.x)/2+80)+'" y="'+((a.y+c.y)/2+34)+'">'+escapeHtml(e.total+" relation"+(e.total===1?"":"s"))+'</text>';}});nodes.forEach(function(n){const p=layout.positions.get(n.id);body+=boxNode("service-node",n.id,p.x,p.y,p.w,p.h,n.label||n.project,serviceBoxMeta(n),state.selected===n.id,false);});layer.innerHTML=body;wireGraphSelection();applyTransform();if(state.selected)showServiceDetails(state.selected,false);}
function renderArchitectureMap(){const svg=document.getElementById("workspace-graph");const layer=document.getElementById("graph-layer");const width=svg.clientWidth||1400;const nodes=visibleServices();const nodeIds=new Set(nodes.map(function(n){return n.id;}));const layout=architectureLayout(nodes,width);state.positions=layout.positions;setViewBox(width,layout.height);let body='<rect x="0" y="0" width="'+Math.max(width,1700)+'" height="'+layout.height+'" fill="transparent"></rect><text class="lane-title" x="90" y="42">Architecture Map</text>';layout.domains.forEach(function(domain,i){body+='<text class="domain-title" x="'+layout.xs[i]+'" y="84">'+escapeHtml(layout.labels[domain])+'</text>';});const focused=state.selected?serviceFocus(state.selected):null;filteredServiceEdges().forEach(function(e){if(!nodeIds.has(e.from)||!nodeIds.has(e.to))return;const a=layout.positions.get(e.from),c=layout.positions.get(e.to);if(!a||!c)return;const isFocused=state.selected&&(e.from===state.selected||e.to===state.selected);body+='<path class="edge '+routeStatusClass(e.risk)+(isFocused?' focused':'')+'" marker-end="url(#'+(isFocused?'arrow-focus':'arrow')+')" d="'+curvedPath(a,c)+'"></path>';if(isFocused&&state.labels){body+='<text class="edge-label" x="'+((a.x+c.x)/2+100)+'" y="'+((a.y+c.y)/2+32)+'">'+escapeHtml(e.total+" relation"+(e.total===1?"":"s"))+'</text>';}});nodes.forEach(function(n){const p=layout.positions.get(n.id);if(!p)return;const dim=focused&&!focused.has(n.id);body+=boxNode("service-node",n.id,p.x,p.y,p.w,p.h,n.label||n.project,serviceBoxMeta(n),state.selected===n.id,dim);});layer.innerHTML=body;wireGraphSelection();applyTransform();if(state.selected)showServiceDetails(state.selected,false);}
function serviceFocus(id){const ids=new Set([id]);serviceEdges.forEach(function(e){if(e.from===id)ids.add(e.to);if(e.to===id)ids.add(e.from);});return ids;}
function serviceIdForProject(project){const node=serviceNodes.find(function(n){return n.project===project;});return node?node.id:null;}
function sourceHref(project,file,line){if(!project||!file||!workspaceGraph.root)return "";let path=workspaceGraph.root.replace(/\/$/,"")+"/"+project.replace(/^\/+/,"")+"/"+file.replace(/^\/+/,"");return "file://"+path.split("/").map(function(part,i){return i===0?"":encodeURIComponent(part);}).join("/")+(line?"#L"+line:"");}
function fileLink(project,file,line){if(!file)return "";const label=file+(line?":"+line:"");const href=sourceHref(project,file,line);return href?'<a class="source-link" href="'+escapeAttr(href)+'">'+escapeHtml(label)+'</a>':escapeHtml(label);}
function issueGroupByID(id){return buildIssueGroups().find(function(g){return g.id===id;});}
function renderIssueWorkbench(){const svg=document.getElementById("workspace-graph");const layer=document.getElementById("graph-layer");const width=Math.max(svg.clientWidth||1200,1180);const groups=buildIssueGroups();state.positions=new Map();const rowH=118,top=124,x=70,w=Math.min(980,width-140),h=84;const height=Math.max(720,top+Math.max(groups.length,1)*rowH+90);setViewBox(width,height);let body='<rect x="0" y="0" width="'+Math.max(width,1180)+'" height="'+height+'" fill="transparent"></rect><text class="lane-title" x="70" y="42">Issue Workbench</text><text class="raw-meta" x="70" y="66">Open Issues groups unresolved, mismatched, dynamic, and out-of-scope contracts. Repeated /tree routes are shown as one prefix/gateway problem family.</text><text class="domain-title" x="70" y="104">Problem family</text>';if(!groups.length){body+='<text class="empty" x="70" y="150">No open issues for the current search/filter.</text>';}groups.forEach(function(g,i){const y=top+i*rowH;state.positions.set(g.id,{x:x,y:y,w:w,h:h});const selected=state.selected===g.id;body+='<g class="issue-card'+(selected?' selected':'')+'" data-select-id="'+escapeAttr(g.id)+'" data-focus-id="'+escapeAttr(g.id)+'"><title>'+escapeHtml(g.title)+'</title><rect x="'+x+'" y="'+y+'" width="'+w+'" height="'+h+'"></rect>';body+=svgTextBlock("issue-title",x+16,y+26,g.title,Math.floor((w-32)/8),2,16);body+=svgTextBlock("issue-meta",x+16,y+h-26,g.traces.length+" endpoints | "+g.risk+" | "+g.projectList.join(", "),Math.floor((w-32)/7),1,14);body+='</g>';});layer.innerHTML=body;wireGraphSelection();applyTransform();if(state.selected&&String(state.selected).startsWith("issue:"))showIssueDetails(state.selected);}
function renderEndpointTrace(){const svg=document.getElementById("workspace-graph");const layer=document.getElementById("graph-layer");const width=svg.clientWidth||1100;const rows=visibleTraces();const trace=traceById.get(state.selected)||rows[0];state.positions=new Map();setViewBox(width,760);let body='<rect x="0" y="0" width="'+Math.max(width,900)+'" height="760" fill="transparent"></rect>';if(!trace){layer.innerHTML=body+'<text class="lane-title" x="80" y="90">No endpoint trace found</text>';applyTransform();return;}const steps=trace.steps||[];const gap=330;steps.forEach(function(step,i){const x=70+i*gap,y=170;const p={x:x,y:y,w:286,h:98};state.positions.set(step.id,p);body+=boxNode("trace-step trace-card "+escapeAttr(step.kind)+(state.focusStep===step.id?' focused':''),step.id,x,y,p.w,p.h,step.label,step.kind+(step.file?" / "+step.file:""),false,false);if(i+1<steps.length){const x1=x+p.w,y1=y+p.h/2,x2=70+(i+1)*gap,y2=y+p.h/2;body+='<path class="edge focused" marker-end="url(#arrow-focus)" d="M'+x1+' '+y1+' L'+x2+' '+y2+'"></path>';}});body+='<text class="lane-title" x="70" y="95">Endpoint Trace</text><text class="trace-title" x="70" y="122">'+escapeHtml(trace.route)+'</text><text class="trace-meta" x="70" y="144">'+escapeHtml((trace.from_project||"unknown")+" -> "+(trace.to_project||"unresolved"))+'</text>';layer.innerHTML=body;wireGraphSelection();applyTransform();showTraceDetails(trace.id,false);}
function endpointPathRowsForService(serviceId){const node=serviceById.get(serviceId)||visibleServices()[0];if(!node)return {service:null,rows:[]};const project=node.project;const rows=[];traces.forEach(function(t){if(t.from_project===project||t.to_project===project){rows.push({id:t.id,from:t.from_project||"unknown",route:t.route||[t.method,t.path].filter(Boolean).join(" "),to:t.to_project||"unresolved",status:t.status||"UNKNOWN",risk:t.risk||"",kind:"endpoint_trace"});}});serviceEdges.forEach(function(e){if(e.from===node.id||e.to===node.id){(e.endpoints&&e.endpoints.length?e.endpoints:["service dependency"]).forEach(function(endpoint,index){rows.push({id:e.id+":path:"+index,from:e.from_project||"unknown",route:endpoint,to:e.to_project||"unknown",status:e.risk||"relationship",risk:e.risk||"",kind:"service_relation"});});}});const seen=new Set();return {service:node,rows:rows.filter(function(row){const key=row.from+"|"+row.route+"|"+row.to+"|"+row.kind;if(seen.has(key))return false;seen.add(key);return true;}).slice(0,120)};}
function renderEndpointPaths(){const svg=document.getElementById("workspace-graph");const layer=document.getElementById("graph-layer");const width=Math.max(svg.clientWidth||1200,1220);const selectedService=state.selected&&serviceById.has(state.selected)?state.selected:null;const model=endpointPathRowsForService(selectedService);state.positions=new Map();const rowH=112,top=132,leftX=70,midX=430,rightX=860,cardW=300,cardH=72;const height=Math.max(720,top+Math.max(model.rows.length,1)*rowH+90);setViewBox(width,height);let body='<rect x="0" y="0" width="'+Math.max(width,1280)+'" height="'+height+'" fill="transparent"></rect><text class="lane-title" x="70" y="42">Endpoint Paths</text><text class="raw-meta" x="70" y="66">Select a service to list its endpoint relations as caller -> endpoint -> provider. This replaces the low-level raw node cloud.</text><text class="domain-title" x="'+leftX+'" y="106">Caller</text><text class="domain-title" x="'+midX+'" y="106">Endpoint / relation</text><text class="domain-title" x="'+rightX+'" y="106">Provider / next hop</text>';
if(!model.service){layer.innerHTML=body+'<text class="empty" x="70" y="160">No service selected or available.</text>';applyTransform();return;}
if(!model.rows.length){body+='<text class="empty" x="70" y="160">No endpoint paths found for '+escapeHtml(model.service.label||model.service.project)+'.</text>';}
model.rows.forEach(function(row,i){const y=top+i*rowH;const fromId=serviceIdForProject(row.from)||row.id+":from",routeId=row.id,toId=serviceIdForProject(row.to)||row.id+":to";const selected=state.selected===row.id;const pFrom={x:leftX,y:y,w:cardW,h:cardH},pRoute={x:midX,y:y,w:cardW+80,h:cardH},pTo={x:rightX,y:y,w:cardW,h:cardH};state.positions.set(row.id,pRoute);state.positions.set(fromId,pFrom);state.positions.set(toId,pTo);body+=boxNode("raw-node path-card",fromId,pFrom.x,pFrom.y,pFrom.w,pFrom.h,row.from,row.kind,false,false);body+=boxNode("raw-node path-card",routeId,pRoute.x,pRoute.y,pRoute.w,pRoute.h,row.route,row.status+(row.risk?" / "+row.risk:""),selected,false);body+=boxNode("raw-node path-card",toId,pTo.x,pTo.y,pTo.w,pTo.h,row.to,row.kind,false,false);body+='<path class="edge focused" marker-end="url(#arrow-focus)" d="M'+(pFrom.x+pFrom.w)+' '+(y+cardH/2)+' L'+pRoute.x+' '+(y+cardH/2)+'"></path>';body+='<path class="edge focused" marker-end="url(#arrow-focus)" d="M'+(pRoute.x+pRoute.w)+' '+(y+cardH/2)+' L'+pTo.x+' '+(y+cardH/2)+'"></path>';});
layer.innerHTML=body;wireGraphSelection();applyTransform();showEndpointPathDetails(model.service,model.rows);}
function renderCanvas(){if(state.mode==="issues")renderIssueWorkbench();else if(state.mode==="architecture")renderArchitectureMap();else if(state.mode==="services")renderServiceMap();else if(state.mode==="endpoint")renderEndpointTrace();else renderEndpointPaths();}
function applyTransform(){document.getElementById("graph-layer").setAttribute("transform","translate("+state.panX+" "+state.panY+") scale("+state.zoom+")");document.getElementById("zoom-readout").textContent=Math.round(state.zoom*100)+"%";}
function zoomBy(delta){state.zoom=Math.max(.35,Math.min(3,state.zoom*delta));applyTransform();}
function panBy(dx,dy){state.panX+=dx;state.panY+=dy;applyTransform();}
function resetView(){state.zoom=1;state.panX=0;state.panY=0;applyTransform();}
function centerOnPosition(id){const p=state.positions.get(id);if(!p)return;const svg=document.getElementById("workspace-graph");const width=svg.clientWidth||1100,height=svg.clientHeight||720;state.panX=(width/2-(p.x+p.w/2))*state.zoom;state.panY=(height/2-(p.y+p.h/2))*state.zoom;applyTransform();}
function focusGraphItem(id){if(!id)return;centerOnPosition(id);}
function focusTraceStep(id){state.focusStep=id;renderCanvas();centerOnPosition(id);}
function wireGraphSelection(){document.querySelectorAll("#graph-layer [data-select-id]").forEach(function(el){el.addEventListener("pointerdown",function(e){e.stopPropagation();});el.addEventListener("click",function(e){e.preventDefault();e.stopPropagation();if(state.dragMoved)return;selectOrToggleItem(el.dataset.selectId);});});}
function detailField(label,value){return value?'<div class="detail-item"><strong>'+escapeHtml(label)+'</strong><small>'+escapeHtml(value)+'</small></div>':"";}
function badges(edge){return '<span class="badge ok">'+(edge.resolved||0)+' resolved</span><span class="badge risk">'+(edge.mismatched||0)+' mismatch</span><span class="badge warn">'+(edge.unresolved||0)+' unresolved</span>';}
function edgeRows(edges,title){let html='<div class="relation-section"><h3>'+escapeHtml(title)+'</h3>';if(!edges.length)return html+'<p class="empty">None.</p></div>';edges.forEach(function(e){const other=title==="Outgoing"?serviceById.get(e.to):serviceById.get(e.from);const examples=(e.problems&&e.problems.length?e.problems:e.endpoints||[]).slice(0,3).join(" | ");html+='<button class="relation-row" data-select-id="'+escapeAttr(other?other.id:"")+'"><strong>'+escapeHtml(e.direction)+'</strong><span>'+escapeHtml(examples)+'</span>'+badges(e)+'</button>';});return html+'</div>';}
function showServiceDetails(id,select){if(select)state.selected=id;const node=serviceById.get(id);if(!node)return;const incoming=serviceEdges.filter(function(e){return e.to===id;});const outgoing=serviceEdges.filter(function(e){return e.from===id;});let html='<h1>'+escapeHtml(node.label||node.project)+'</h1><div class="detail-list">'+detailField("Project",node.project)+detailField("Kind",node.kind)+detailField("Service",node.service)+detailField("Status",node.status)+'</div>';html+=edgeRows(outgoing,"Outgoing")+edgeRows(incoming,"Incoming");document.getElementById("details").innerHTML=html;document.querySelectorAll(".relation-row[data-select-id]").forEach(function(el){el.addEventListener("click",function(){if(el.dataset.selectId)selectItem(el.dataset.selectId);});});}
function showTraceDetails(id,select){if(select)state.selected=id;const trace=traceById.get(id);if(!trace)return;let html='<h1>'+escapeHtml(trace.route)+'</h1><div class="detail-list">'+detailField("Direction",(trace.from_project||"unknown")+" -> "+(trace.to_project||"unresolved"))+detailField("Status",trace.status)+detailField("Risk",trace.risk)+'</div><div class="relation-section"><h3>Endpoint Trace</h3>';(trace.steps||[]).forEach(function(step,i){const source=step.file?fileLink(step.project,step.file,step.line):"";html+='<button class="relation-row'+(state.focusStep===step.id?' selected':'')+'" data-step-id="'+escapeAttr(step.id)+'"><strong>'+escapeHtml((i+1)+". "+step.label)+'</strong><span>'+escapeHtml(step.kind)+(source?" / "+source:"")+'</span></button>';});html+='</div>';document.getElementById("details").innerHTML=html;document.querySelectorAll("[data-step-id]").forEach(function(el){el.addEventListener("click",function(){focusTraceStep(el.dataset.stepId);});});}
function showRawDetails(id){const node=rawById.get(id);if(!node)return;let html='<h1>'+escapeHtml(node.label||node.id)+'</h1><div class="detail-list">'+detailField("Kind",node.kind)+detailField("Project",node.project)+detailField("File",node.file)+detailField("Symbol",node.symbol)+detailField("Route",[node.method,node.path].filter(Boolean).join(" "))+detailField("Confidence",node.confidence)+detailField("Risk",node.risk)+'</div>';document.getElementById("details").innerHTML=html;}
function showEndpointPathDetails(service,rows){let html='<h1>'+escapeHtml(service.label||service.project)+'</h1><div class="detail-list">'+detailField("Project",service.project)+detailField("View","Endpoint Paths list caller -> endpoint/relation -> provider/next hop for the selected service.")+'</div><div class="relation-section"><h3>Endpoint Paths</h3>';if(!rows.length)html+='<p class="empty">No endpoint paths found.</p>';rows.forEach(function(row){html+='<button class="relation-row" data-path-id="'+escapeAttr(row.id)+'"><strong>'+escapeHtml(row.route)+'</strong><span>'+escapeHtml(row.from+" -> "+row.to+" / "+row.status)+'</span></button>';});html+='</div>';document.getElementById("details").innerHTML=html;document.querySelectorAll("[data-path-id]").forEach(function(el){el.addEventListener("click",function(){selectItem(el.dataset.pathId);});});}
function showIssueDetails(id){const group=issueGroupByID(id);if(!group)return;let html='<h1>'+escapeHtml(group.title)+'</h1><div class="detail-list">'+detailField("Issue",group.risk)+detailField("Likely owner",group.to)+detailField("Affected projects",group.projectList.join(", "))+'</div><div class="relation-section"><h3>Open endpoints</h3>';group.traces.forEach(function(trace){const firstSource=(trace.steps||[]).find(function(step){return step.file;});const source=firstSource?fileLink(firstSource.project,firstSource.file,firstSource.line):"";html+='<button class="relation-row" data-trace-id="'+escapeAttr(trace.id)+'"><strong>'+escapeHtml(trace.route)+'</strong><span>'+escapeHtml((trace.from_project||"unknown")+" -> "+(trace.to_project||"unresolved")+" / "+(trace.status||"UNKNOWN")+" / "+(trace.risk||""))+(source?" / "+source:"")+'</span></button>';});html+='</div>';document.getElementById("details").innerHTML=html;document.querySelectorAll("[data-trace-id]").forEach(function(el){el.addEventListener("click",function(){state.mode="endpoint";document.querySelectorAll("[data-view-mode]").forEach(function(btn){btn.classList.toggle("active",btn.dataset.viewMode==="endpoint");});selectItem(el.dataset.traceId);});});}
function clearSelection(){state.selected=null;state.focusStep=null;document.getElementById("details").innerHTML='<p class="empty">Select a service or endpoint to inspect directed relationships.</p>';renderList();renderCanvas();}
function selectOrToggleItem(id){if(state.selected===id){clearSelection();return;}selectItem(id);}
function selectItem(id){state.selected=id;state.focusStep=null;renderList();renderCanvas();if(state.mode==="issues")showIssueDetails(id);else if(state.mode==="architecture"||state.mode==="services")showServiceDetails(id,false);else if(state.mode==="endpoint")showTraceDetails(id,false);else if(serviceById.has(id)){const model=endpointPathRowsForService(id);showEndpointPathDetails(model.service,model.rows);}else if(traceById.has(id))showTraceDetails(id,false);else showRawDetails(id);}
function setMode(mode){state.mode=mode;state.selected=null;state.filter="all";document.querySelectorAll("[data-view-mode]").forEach(function(btn){btn.classList.toggle("active",btn.dataset.viewMode===mode);});document.querySelectorAll("[data-kind-filter]").forEach(function(btn){btn.classList.toggle("active",btn.dataset.kindFilter==="all");});document.getElementById("mode-help").textContent=mode==="issues"?"Open Issues groups unresolved, mismatched, and out-of-scope contracts by cause so duplicate routes can be triaged together.":mode==="architecture"?"Architecture Map groups scanned projects into frontend, domain, and backend lanes. Select a service or relation to inspect direction and evidence.":mode==="services"?"Service Map shows directed project/service relationships. Select a service to see incoming and outgoing API calls.":mode==="endpoint"?"Endpoint Trace shows a selected call from frontend consumer through backend handler, steps, tests, and risks.":"Endpoint Paths lists one selected service as caller -> endpoint/relation -> provider/next hop, without the old raw node cloud.";resetView();renderList();renderCanvas();}
document.getElementById("workspace-search").addEventListener("input",function(e){state.query=e.target.value;state.selected=null;renderList();renderCanvas();});
document.querySelectorAll("[data-view-mode]").forEach(function(btn){btn.addEventListener("click",function(){setMode(btn.dataset.viewMode);});});
document.querySelectorAll("[data-kind-filter]").forEach(function(btn){btn.addEventListener("click",function(){state.filter=btn.dataset.kindFilter;state.selected=null;document.querySelectorAll("[data-kind-filter]").forEach(function(b){b.classList.toggle("active",b===btn);});renderList();renderCanvas();});});
document.getElementById("zoom-in").addEventListener("click",function(){zoomBy(1.2);});
document.getElementById("zoom-out").addEventListener("click",function(){zoomBy(.84);});
document.getElementById("reset-view").addEventListener("click",resetView);
document.getElementById("clear-selection").addEventListener("click",clearSelection);
document.addEventListener("keydown",function(e){if(e.key==="Escape")clearSelection();});
document.getElementById("fit-button").addEventListener("click",function(){state.query="";state.selected=null;document.getElementById("workspace-search").value="";resetView();renderList();renderCanvas();});
document.getElementById("toggle-labels").addEventListener("click",function(){state.labels=!state.labels;this.classList.toggle("active",state.labels);renderCanvas();});
document.getElementById("workspace-graph").addEventListener("wheel",function(e){e.preventDefault();zoomBy(e.deltaY<0?1.08:.92);},{passive:false});
document.getElementById("workspace-graph").addEventListener("pointerdown",function(e){e.preventDefault();state.drag={x:e.clientX,y:e.clientY};state.dragMoved=false;this.setPointerCapture(e.pointerId);});
document.getElementById("workspace-graph").addEventListener("pointermove",function(e){if(!state.drag)return;const dx=e.clientX-state.drag.x,dy=e.clientY-state.drag.y;if(Math.abs(dx)+Math.abs(dy)>2)state.dragMoved=true;panBy(dx,dy);state.drag={x:e.clientX,y:e.clientY};});
document.getElementById("workspace-graph").addEventListener("pointerup",function(){state.drag=null;setTimeout(function(){state.dragMoved=false;},0);});
window.addEventListener("resize",renderCanvas);
renderList();renderCanvas();
</script>
</body>
</html>`)
	return b.String()
}

func marshalDashboardPayload(value any) []byte {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return bytes.TrimSpace(b.Bytes())
}

type dashboardSourceRecord struct {
	Label   string `json:"label"`
	Project string `json:"project,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

func buildDashboardSourceIndex(endpointTraces WorkspaceEndpointTraceIndexRecord) []dashboardSourceRecord {
	seen := map[string]bool{}
	var records []dashboardSourceRecord
	for _, trace := range endpointTraces.Traces {
		for _, step := range trace.Steps {
			if step.File == "" {
				continue
			}
			label := step.File
			if step.Line > 0 {
				label += ":" + strconv.Itoa(step.Line)
			}
			key := step.Project + "\x00" + label
			if seen[key] {
				continue
			}
			seen[key] = true
			records = append(records, dashboardSourceRecord{
				Label:   label,
				Project: step.Project,
				File:    step.File,
				Line:    step.Line,
			})
		}
	}
	return records
}

func workspaceRegistryFromGraph(graph WorkspaceGraphRecord) WorkspaceRegistryRecord {
	registry := WorkspaceRegistryRecord{Root: graph.Root}
	seen := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind != "project" || node.Project == "" || seen[node.Project] {
			continue
		}
		seen[node.Project] = true
		registry.Projects = append(registry.Projects, WorkspaceProjectRecord{
			Name:    firstNonEmpty(node.Label, node.Project),
			Path:    node.Project,
			Kind:    node.Meta["kind"],
			Service: node.Meta["service"],
			Indexed: true,
			Status:  node.Risk,
		})
	}
	return registry
}
