'use strict';
// Adapt existing projection records for presentation without changing their identities.
function dashboardSourceScope(file) {
  const normalized=String(file||'').replace(/\\/g,'/');
  if (/(^|\/)(test|tests|__tests__|e2e|fixtures|__mocks__)(\/|$)|\.(test|spec)\.[^/]+$/i.test(normalized)) return 'test';
  return /(^|\/)src\//.test(normalized)?'production':'unknown';
}
function dashboardProjection(payload) {
  const serviceMap=payload.service_map||{},graph=payload.graph||{};
  const nodes=(serviceMap.nodes||[]).map(node=>({...node,label:node.label||node.project||node.id,domain:node.domain||'ungrouped'}));
  const nodeById=new Map(nodes.map(node=>[node.id,node]));
  const groups=(serviceMap.architecture_groups||[]).map(group=>({...group}));
  for(const node of nodes) if(!groups.some(group=>group.id===node.domain)) groups.push({id:node.domain,label:node.domain==='ungrouped'?'Ohne Gruppenzuordnung':node.domain,order:groups.length});
  const traces=(payload.endpoint_traces?.traces||[]).map(trace=>({...trace,method:trace.method||String(trace.route||'').split(' ')[0]||'',path:trace.path||String(trace.route||'').split(' ').slice(1).join(' '),steps:trace.steps||[]}));
  const byPair=new Map();
  for(const trace of traces){const key=JSON.stringify([trace.from_project,trace.to_project]);if(!byPair.has(key))byPair.set(key,[]);byPair.get(key).push(trace);}
  const edges=(serviceMap.edges||[]).map(edge=>{
    const projected={...edge,endpoints:edge.endpoints||[],evidence:edge.evidence||[],from_project:edge.from_project||nodeById.get(edge.from)?.project,to_project:edge.to_project||nodeById.get(edge.to)?.project};
    let records=[];
    if(projected.endpoints.includes('java_client_import')&&projected.evidence.length===edge.total){
      records=projected.evidence.map((evidence,index)=>{
        const match=String(evidence).match(/^(.+?)(?::(\d+))?\s+imports\s+([^;\s]+)/);
        const location=match?match[1]:String(evidence).split(/\s/)[0].replace(/:\d+$/,'');
        return {id:edge.id+':import:'+index,count:1,kind:'java',scope:dashboardSourceScope(location),file:location,line:Number(match?.[2]||0),client:match?.[3]||'',evidence,status:edge.resolved===edge.total?'RESOLVED':'UNKNOWN'};
      });
    }else{
      records=(byPair.get(JSON.stringify([projected.from_project,projected.to_project]))||[]).map(trace=>{
        const origins=trace.steps.filter(step=>['api_contract','api_call'].includes(step.kind)&&step.project===projected.from_project);
        const locations=new Map(origins.map(step=>[JSON.stringify([step.file,step.line]),step]));
        const origin=locations.size===1?[...locations.values()][0]:{};
        return {id:trace.id,count:1,kind:trace.method&&trace.path?'http':'unknown',scope:dashboardSourceScope(origin.file),file:origin.file||'',line:origin.line||0,method:trace.method,path:trace.path,evidence:origin.symbol||'',status:trace.status||'UNKNOWN'};
      });
    }
    // Conflicting aggregates cannot be apportioned reliably to individual traces.
    if(records.length>edge.total)records=[];
    const remaining=Math.max(0,(edge.total||0)-records.length);
    if(remaining)records.push({id:edge.id+':unmapped',count:remaining,kind:'unknown',scope:'unknown',file:'',status:'UNKNOWN',evidence:'Im Service-Paar gezählt; keinem Einzelnachweis eindeutig zugeordnet.'});
    return {...projected,records};
  });
  return {architecture:{generated:serviceMap.generated||graph.generated,nodes,edges,groups},workspace:{tooling:payload.tooling||{},generated:serviceMap.generated||graph.generated,root:graph.root||serviceMap.root||'',symbols:payload.symbol_index?.symbols||[],usages:payload.symbol_usages?.usages||[],usageCoverage:payload.symbol_usages?.coverage||[],endpoints:payload.api_catalog?.endpoints||[],traces,health:serviceMap.health||{},workspaceCoverage:serviceMap.workspace_coverage||{},contractSummary:serviceMap.contract_summary||{},capabilities:serviceMap.capabilities||[],diagnosticFamilies:serviceMap.diagnostic_families||[]}};
}
const projectedDashboard=dashboardProjection(workspacePayload);
window.ARCHITECTURE_DATA=projectedDashboard.architecture;
window.WORKSPACE_DATA=projectedDashboard.workspace;
const usageAssets=workspacePayload.code_usage_assets||{};
const usageLoads=new Map();
const allUsageRecords=new Map(window.WORKSPACE_DATA.usages.map(usage=>[usage.id,usage]));
function registerUsageRecords(records){
  const added=[];
  for(const usage of records||[])if(!allUsageRecords.has(usage.id)){allUsageRecords.set(usage.id,usage);added.push(usage);}
  window.WORKSPACE_DATA.usages=[...allUsageRecords.values()];
  window.onWorkspaceUsages?.(added);
}
globalThis.__goregraphRegisterCodeUsageShard=(project,payload)=>{
  const request=usageLoads.get(project);
  if(!request||request.status!=='loading')return;
  request.registered=true;
  registerUsageRecords(payload?.usages);
};
function loadWorkspaceUsageAsset(project){
  if(usageLoads.has(project))return;
  const asset=usageAssets[project];
  if(!asset)return;
  const request={status:'loading',registered:false};usageLoads.set(project,request);
  if(!/^workspace-map-assets\/code-usages-[a-f0-9]{32}\.js$/.test(asset)){request.status='error';return;}
  const script=document.createElement('script');script.src=asset;script.async=true;
  let timer;
  const finish=success=>{if(request.status!=='loading')return;clearTimeout(timer);request.status=success?'ready':'error';script.remove();window.onWorkspaceUsageLoad?.();};
  script.onload=()=>finish(request.registered);script.onerror=()=>finish(false);
  timer=setTimeout(()=>finish(false),15000);document.head.appendChild(script);
}
function ensureWorkspaceUsages(project,outgoing){
  // Provider-oriented shards contain all incoming evidence. Outgoing references may
  // reside in any provider shard, so they require the complete existing projection.
  const projects=outgoing?Object.keys(usageAssets):[project].filter(p=>usageAssets[p]);
  const active=[...usageLoads.values()].filter(request=>request.status==='loading').length;
  projects.filter(p=>!usageLoads.has(p)).slice(0,Math.max(0,4-active)).forEach(loadWorkspaceUsageAsset);
  return {loading:projects.some(p=>!usageLoads.has(p)||usageLoads.get(p).status==='loading'),failed:projects.filter(p=>usageLoads.get(p)?.status==='error')};
}
function retryWorkspaceUsages(){for(const [project,request] of usageLoads)if(request.status==='error')usageLoads.delete(project);}
