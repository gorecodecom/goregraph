package scan

const workspaceDashboardArchitectureModelScript = `
const architectureLanePalette=[
  {fill:"#e8eef1",stroke:"#bdcbd2"},{fill:"#edf0eb",stroke:"#c4cec0"},
  {fill:"#f1eee8",stroke:"#d2c8b9"},{fill:"#ecebf1",stroke:"#c8c4d1"},
  {fill:"#eaf0ef",stroke:"#bfd0cc"},{fill:"#f0ebea",stroke:"#d2c2bf"}
];
function architectureDomainKey(node){const value=String(node&&node.domain||"").trim().toLowerCase();return value||"unassigned";}
function architectureDomainLabel(domain){return String(domain||"unassigned").split(/[._\-/]+/).filter(Boolean).map(function(word){return word.charAt(0).toUpperCase()+word.slice(1);}).join(" ")||"Unassigned";}
function architectureDomainColor(domain){let hash=2166136261;String(domain||"unassigned").split("").forEach(function(character){hash^=character.charCodeAt(0);hash=Math.imul(hash,16777619);});return architectureLanePalette[(hash>>>0)%architectureLanePalette.length];}
function architectureStringCompare(a,b){a=String(a||"");b=String(b||"");return a<b?-1:a>b?1:0;}
function architectureMoveItem(items,fromIndex,toIndex){
  const result=(items||[]).slice();if(fromIndex<0||fromIndex>=result.length||toIndex<0||toIndex>=result.length||fromIndex===toIndex)return result;
  const moved=result.splice(fromIndex,1)[0];result.splice(toIndex,0,moved);return result;
}
function architectureMoveServiceDraft(draft,project,fromGroupID,toGroupID,toIndex){
  const groups=((draft&&draft.groups)||[]).map(function(group){return Object.assign({},group,{services:(group.services||[]).slice()});}),source=groups.find(function(group){return group.id===fromGroupID;}),target=groups.find(function(group){return group.id===toGroupID;});
  if(!source||!target)return {groups:groups};const sourceIndex=source.services.findIndex(function(service){return service.project===project;});if(sourceIndex<0)return {groups:groups};
  const moved=Object.assign({},source.services.splice(sourceIndex,1)[0],{group:toGroupID,manual:true});const bounded=Math.max(0,Math.min(Number.isInteger(toIndex)?toIndex:target.services.length,target.services.length));target.services.splice(bounded,0,moved);source.services=source.services.map(function(service){return Object.assign({},service,{manual:true});});target.services=target.services.map(function(service){return Object.assign({},service,{manual:true});});return {groups:groups};
}
function architectureEditorLifecycle(editor,event){
  const next=Object.assign({},editor||{}),type=event&&event.type;if(type==="begin"){if(next.architectureBusy)return next;next.architectureBusy=true;next.architectureRequestVersion=(next.architectureRequestVersion||0)+1;return next;}
  if(event&&event.requestVersion!==undefined&&event.requestVersion!==next.architectureRequestVersion)return next;
  if(type==="dirty"){next.architectureDirty=true;return next;}if(type==="discard"){next.architectureDirty=false;return next;}if(type==="failure"){next.architectureBusy=false;return next;}if(type==="saved"){next.architectureBusy=false;next.architectureDirty=false;return next;}if(type==="reset"){next.architectureBusy=false;next.architectureDirty=false;next.architectureEditing=false;next.architectureBaseDraft=null;next.architectureDraft=null;next.architectureLoadedConfig=architectureEmptyConfig();next.architectureResetRequiresRebuild=true;return next;}return next;
}
function architectureEditorCanMutate(editor){return !!(editor&&editor.architectureEditing&&!editor.architectureBusy&&!editor.architectureResetRequiresRebuild&&editor.architectureDraft);}
function architectureEditorNeedsDiscardConfirmation(editor){return !!(editor&&editor.architectureDirty);}
function architectureCloneDraft(draft){return {groups:((draft&&draft.groups)||[]).map(function(group){return Object.assign({},group,{services:(group.services||[]).map(function(service){return Object.assign({},service);})});})};}
function architectureDraftServicePositions(draft){const positions=new Map();((draft&&draft.groups)||[]).forEach(function(group){(group.services||[]).forEach(function(service,index){positions.set(service.project,{group:group.id,index:index,service:service});});});return positions;}
function architectureLocallyChangedServices(baseDraft,localDraft){
  const base=architectureDraftServicePositions(baseDraft),local=architectureDraftServicePositions(localDraft),changed=new Set();local.forEach(function(position,project){const previous=base.get(project);if(!previous||previous.group!==position.group){changed.add(project);return;}base.forEach(function(otherPrevious,otherProject){const otherLocal=local.get(otherProject);if(otherProject===project||otherPrevious.group!==previous.group||!otherLocal||otherLocal.group!==position.group)return;const before=previous.index<otherPrevious.index,after=position.index<otherLocal.index;if(before!==after){changed.add(project);changed.add(otherProject);}});});return changed;
}
function architectureThreeWayGroupOrder(baseDraft,localDraft,latestDraft){
  const base=((baseDraft&&baseDraft.groups)||[]).map(function(group){return group.id;}),local=((localDraft&&localDraft.groups)||[]).map(function(group){return group.id;}),latest=((latestDraft&&latestDraft.groups)||[]).map(function(group){return group.id;}),localIndex=new Map(local.map(function(id,index){return [id,index];})),latestIndex=new Map(latest.map(function(id,index){return [id,index];})),outgoing=new Map(latest.map(function(id){return [id,new Set()];})),indegree=new Map(latest.map(function(id){return [id,0];}));
  for(let left=0;left<base.length;left++)for(let right=left+1;right<base.length;right++){const a=base[left],b=base[right];if(!localIndex.has(a)||!localIndex.has(b)||!latestIndex.has(a)||!latestIndex.has(b)||localIndex.get(a)<localIndex.get(b))continue;if(!outgoing.get(b).has(a)){outgoing.get(b).add(a);indegree.set(a,indegree.get(a)+1);}}
  const available=latest.filter(function(id){return indegree.get(id)===0;}),result=[];while(available.length){available.sort(function(a,b){return latestIndex.get(a)-latestIndex.get(b);});const id=available.shift();result.push(id);outgoing.get(id).forEach(function(next){indegree.set(next,indegree.get(next)-1);if(indegree.get(next)===0)available.push(next);});}return result.length===latest.length?result:latest;
}
function architectureDraftThreeWayMerge(baseDraft,localDraft,latestDraft){
  const base=architectureCloneDraft(baseDraft),local=architectureCloneDraft(localDraft),latest=architectureCloneDraft(latestDraft),baseGroups=new Map(base.groups.map(function(group){return [group.id,group];})),localGroups=new Map(local.groups.map(function(group){return [group.id,group];})),latestGroups=new Map(latest.groups.map(function(group){return [group.id,group];}));
  local.groups.forEach(function(group){const previous=baseGroups.get(group.id);if(latestGroups.has(group.id)){if(!previous||group.label!==previous.label){const target=latestGroups.get(group.id);target.label=group.label;target.manual=true;}}else if(!previous||group.label!==previous.label){const added=Object.assign({},group,{services:[]});latest.groups.push(added);latestGroups.set(added.id,added);}});
  const order=architectureThreeWayGroupOrder(base,local,latest),orderIndex=new Map(order.map(function(id,index){return [id,index];}));latest.groups.sort(function(a,b){const aIndex=orderIndex.has(a.id)?orderIndex.get(a.id):Number.MAX_SAFE_INTEGER,bIndex=orderIndex.has(b.id)?orderIndex.get(b.id):Number.MAX_SAFE_INTEGER;return aIndex-bIndex||architectureStringCompare(a.id,b.id);});
  const localPositions=architectureDraftServicePositions(local),changed=[];architectureLocallyChangedServices(base,local).forEach(function(project){changed.push({project:project,position:localPositions.get(project)});});changed.sort(function(a,b){const aGroup=local.groups.findIndex(function(group){return group.id===a.position.group;}),bGroup=local.groups.findIndex(function(group){return group.id===b.position.group;});return aGroup-bGroup||a.position.index-b.position.index||architectureStringCompare(a.project,b.project);});
  changed.forEach(function(change){let moved=null;latest.groups.forEach(function(group){const index=group.services.findIndex(function(service){return service.project===change.project;});if(index>=0)moved=group.services.splice(index,1)[0];});moved=Object.assign({},moved||change.position.service,{manual:true});let target=latest.groups.find(function(group){return group.id===change.position.group;});if(!target){const localGroup=localGroups.get(change.position.group);target=Object.assign({},localGroup||{id:change.position.group,label:architectureDomainLabel(change.position.group)},{services:[]});latest.groups.push(target);}const localGroup=localGroups.get(change.position.group),localServices=localGroup&&localGroup.services||[],previousProjects=localServices.slice(0,change.position.index).map(function(service){return service.project;}).reverse(),nextProjects=localServices.slice(change.position.index+1).map(function(service){return service.project;}),before=nextProjects.map(function(project){return target.services.findIndex(function(service){return service.project===project;});}).find(function(index){return index>=0;}),after=previousProjects.map(function(project){return target.services.findIndex(function(service){return service.project===project;});}).find(function(index){return index>=0;}),index=before!==undefined?before:after!==undefined?after+1:Math.min(change.position.index,target.services.length);target.services.splice(index,0,moved);});return latest;
}
function architectureEditorRebase(payload,baseDraft,localDraft,latestDraft){localDraft=localDraft||baseDraft;latestDraft=latestDraft||baseDraft;const merged=architectureDraftThreeWayMerge(baseDraft,localDraft,latestDraft);return {architectureRevision:payload.revision,architectureLoadedConfig:payload.config,architectureBaseDraft:architectureCloneDraft(latestDraft),architectureDraft:merged,architectureDirty:true};}
function architectureServiceKeyIsStable(project){project=String(project||"");return !!project&&!project.startsWith("/")&&!/^[A-Za-z]:[\\/]/.test(project)&&!project.split(/[\\/]+/).includes("..");}
function architectureDraftFromConfigData(nodes,groupRecords,config){
  config=config||architectureEmptyConfig();const configured=config.architecture||{},configuredGroups=configured.groups||{},configuredServices=configured.services||{},domains=architectureDomains(nodes,groupRecords),groupByID=new Map();
  domains.forEach(function(domain){const loadedGroup=configuredGroups[domain.id],detectedLabel=domain.manual?architectureDomainLabel(domain.id):domain.label;groupByID.set(domain.id,{id:domain.id,label:loadedGroup&&loadedGroup.label||detectedLabel,manual:!!loadedGroup,services:[]});});
  Object.keys(configuredGroups).sort(architectureStringCompare).forEach(function(groupID){if(!groupByID.has(groupID))groupByID.set(groupID,{id:groupID,label:configuredGroups[groupID].label,manual:true,services:[]});});
  (nodes||[]).forEach(function(node){const configuredService=configuredServices[node.project],groupID=configuredService?configuredService.group:architectureDomainKey(node);if(!groupByID.has(groupID))groupByID.set(groupID,{id:groupID,label:(configuredGroups[groupID]&&configuredGroups[groupID].label)||architectureDomainLabel(groupID),manual:!!configuredGroups[groupID],services:[]});groupByID.get(groupID).services.push({id:node.id,project:node.project,label:node.label||node.project,order:configuredService?Number(configuredService.order)||0:Number(node.architecture_order)||0,manual:!!configuredService});});
  const configuredOrder=new Map((configured.groupOrder||[]).map(function(groupID,index){return [groupID,index];})),baseOrder=new Map(domains.map(function(domain,index){return [domain.id,index];})),groups=Array.from(groupByID.values()).sort(function(a,b){const aConfigured=configuredOrder.has(a.id),bConfigured=configuredOrder.has(b.id);if(aConfigured!==bConfigured)return aConfigured?-1:1;if(aConfigured&&configuredOrder.get(a.id)!==configuredOrder.get(b.id))return configuredOrder.get(a.id)-configuredOrder.get(b.id);const aBase=baseOrder.has(a.id)?baseOrder.get(a.id):Number.MAX_SAFE_INTEGER,bBase=baseOrder.has(b.id)?baseOrder.get(b.id):Number.MAX_SAFE_INTEGER;if(aBase!==bBase)return aBase-bBase;return architectureStringCompare(a.label,b.label)||architectureStringCompare(a.id,b.id);});
  groups.forEach(function(group){group.services.sort(function(a,b){if(a.order!==b.order)return a.order-b.order;return architectureStringCompare(a.label,b.label)||architectureStringCompare(a.project,b.project)||architectureStringCompare(a.id,b.id);});});return {groups:groups};
}
function architectureDraftConfigValue(draft){
  const groups={},services={},order=[],requiredGroups=new Set();((draft&&draft.groups)||[]).forEach(function(group){order.push(group.id);(group.services||[]).forEach(function(service,index){if(service.manual&&architectureServiceKeyIsStable(service.project)){services[service.project]={group:group.id,order:index};requiredGroups.add(group.id);}});});((draft&&draft.groups)||[]).forEach(function(group){if(group.manual||requiredGroups.has(group.id))groups[group.id]={label:String(group.label||"").trim()};});return {schema:1,architecture:{groupOrder:order,groups:groups,services:services}};
}
function architectureEmptyConfig(){return {schema:1,architecture:{groupOrder:[],groups:{},services:{}}};}
function architectureDomains(nodes,groupRecords){
  if(groupRecords===undefined&&typeof architectureGroups!=="undefined")groupRecords=architectureGroups;
  const records=new Map(),groups=new Map();
  (groupRecords||[]).forEach(function(record){const id=String(record&&record.id||"").trim();if(!id)return;records.set(id,record);groups.set(id,[]);});
  (nodes||[]).forEach(function(node){const id=architectureDomainKey(node);if(!groups.has(id))groups.set(id,[]);groups.get(id).push(node);});
  return Array.from(groups.entries()).map(function(entry){
    const id=entry[0],record=records.get(id)||{},hasOrder=Object.prototype.hasOwnProperty.call(record,"order"),domainNodes=entry[1].slice().sort(function(a,b){
      const aOrder=Number(a.architecture_order)||0,bOrder=Number(b.architecture_order)||0;if(aOrder!==bOrder)return aOrder-bOrder;
      return architectureStringCompare(a.label,b.label)||architectureStringCompare(a.project,b.project)||architectureStringCompare(a.id,b.id);
    });
    return {id:id,label:record.label||architectureDomainLabel(id),order:hasOrder?Number(record.order)||0:0,hasOrder:hasOrder,manual:!!record.manual,color:architectureDomainColor(id),nodes:domainNodes};
  }).sort(function(a,b){if(a.hasOrder!==b.hasOrder)return a.hasOrder?-1:1;if(a.hasOrder&&a.order!==b.order)return a.order-b.order;return architectureStringCompare(a.label,b.label)||architectureStringCompare(a.id,b.id);});
}
function architectureCanvasGeometry(width,focusHeight){
  const compact=(width||0)<=1000,presentationTop=12,legendTop=compact?56:12,toolsTop=compact?100:12,focusTop=compact?144:96,resolvedFocusHeight=Math.max(44,focusHeight||0),focusBottom=focusTop+resolvedFocusHeight,wideFocusHeight=46,wideTitleClearance=16,wideContentInset=resolvedFocusHeight>wideFocusHeight?resolvedFocusHeight-wideFocusHeight+wideTitleClearance:0;
  return {compact:compact,presentationTop:presentationTop,legendTop:legendTop,toolsTop:toolsTop,focusTop:focusTop,focusBottom:focusBottom,contentInset:compact?focusBottom+24:wideContentInset};
}
function architectureLayout(nodes,width,groupRecords){
  const domains=architectureDomains(nodes,groupRecords),layoutWidth=Math.max(width||0,Math.max(1040,domains.length*300+84)),margin=42,cardWidth=224,cardHeight=74,laneTop=118;
  const step=domains.length>1?(layoutWidth-margin*2-cardWidth)/(domains.length-1):0,positions=new Map();let maxLength=0;
  domains.forEach(function(domain,lane){maxLength=Math.max(maxLength,domain.nodes.length);domain.nodes.forEach(function(node,index){positions.set(node.id,{x:margin+lane*step,y:190+index*90,lane:lane,w:cardWidth,h:cardHeight,domain:domain.id});});});
  return {positions:positions,width:layoutWidth,height:Math.max(760,290+maxLength*90),domains:domains,step:step,cardWidth:cardWidth,cardHeight:cardHeight,margin:margin,laneTop:laneTop};
}
function architectureEdgeRisk(edge){return !!(edge&&(edge.mismatched||edge.unresolved||String(edge.risk||"").match(/risk|mismatch|unresolved|missing/i)));}
function architectureDirectionMatches(edge,selected,domain,direction,nodeByID){
  if(selected){if(direction==="incoming")return edge.to===selected;if(direction==="outgoing")return edge.from===selected;return edge.from===selected||edge.to===selected;}
  if(!domain)return true;
  const from=nodeByID.get(edge.from),to=nodeByID.get(edge.to),fromDomain=from&&architectureDomainKey(from),toDomain=to&&architectureDomainKey(to);
  if(direction==="incoming")return toDomain===domain&&fromDomain!==domain;
  if(direction==="both")return (fromDomain===domain)!==(toDomain===domain);
  return fromDomain===domain&&toDomain!==domain;
}
function architectureFocusModel(nodes,edges,options){
  options=options||{};const nodeByID=new Map((nodes||[]).map(function(node){return [node.id,node];})),nodeIDs=new Set(),edgeIDs=new Set();
  if(options.selected)nodeIDs.add(options.selected);
  if(options.domain)(nodes||[]).forEach(function(node){if(architectureDomainKey(node)===options.domain)nodeIDs.add(node.id);});
  (edges||[]).forEach(function(edge){if(!architectureDirectionMatches(edge,options.selected,options.domain,options.direction||"both",nodeByID))return;if(options.riskOnly&&!architectureEdgeRisk(edge))return;edgeIDs.add(edge.id);nodeIDs.add(edge.from);nodeIDs.add(edge.to);});
  return {nodeIDs:nodeIDs,edgeIDs:edgeIDs};
}
function architectureDirectNeighborhood(edges,selected){
  const nodeIDs=new Set();if(!selected)return nodeIDs;nodeIDs.add(selected);
  (edges||[]).forEach(function(edge){if(edge.from===selected||edge.to===selected){nodeIDs.add(edge.from);nodeIDs.add(edge.to);}});
  return nodeIDs;
}
function architectureRelationshipSummary(selected,edges){
  const incoming=(edges||[]).filter(function(edge){return edge.to===selected;}),outgoing=(edges||[]).filter(function(edge){return edge.from===selected;}),all=incoming.concat(outgoing),sum=function(records,key){return records.reduce(function(total,edge){return total+(edge[key]||0);},0);};
  return {incomingRelationships:sum(incoming,"total"),incomingServices:new Set(incoming.map(function(edge){return edge.from;})).size,outgoingRelationships:sum(outgoing,"total"),outgoingServices:new Set(outgoing.map(function(edge){return edge.to;})).size,resolved:sum(all,"resolved"),unresolved:sum(all,"unresolved"),mismatched:sum(all,"mismatched")};
}
function architectureTooltipPosition(anchor,tooltipSize,viewportSize){
  const padding=8,gap=8,viewportWidth=Math.max(0,viewportSize.width||0),viewportHeight=Math.max(0,viewportSize.height||0),tooltipWidth=Math.min(tooltipSize.width||0,Math.max(0,viewportWidth-padding*2)),tooltipHeight=tooltipSize.height||0,halfWidth=tooltipWidth/2,center=(anchor.left||0)+(anchor.width||0)/2,minLeft=padding+halfWidth,maxLeft=Math.max(minLeft,viewportWidth-padding-halfWidth),left=Math.max(minLeft,Math.min(maxLeft,center)),below=(anchor.bottom||0)+gap,above=(anchor.top||0)-gap-tooltipHeight,fitsBelow=below+tooltipHeight<=viewportHeight-padding;
  return {left:Math.round(left),top:Math.round(fitsBelow?below:Math.max(padding,above)),placement:fitsBelow?"below":"above"};
}
function architectureBundleRisk(edge){return edge.mismatched?"mismatch":edge.unresolved?"unresolved":"resolved";}
function architectureBundles(edges,nodeByID){
  const bundles=new Map();
  (edges||[]).slice().sort(function(a,b){return architectureStringCompare(a.id,b.id);}).forEach(function(edge){
    const from=nodeByID.get(edge.from),to=nodeByID.get(edge.to);if(!from||!to)return;
    const fromDomain=architectureDomainKey(from),toDomain=architectureDomainKey(to),risk=architectureBundleRisk(edge),key=[fromDomain,toDomain,risk].join("|");
    if(!bundles.has(key))bundles.set(key,{id:"bundle:"+key,fromDomain:fromDomain,toDomain:toDomain,risk:risk,total:0,edges:[]});
    const bundle=bundles.get(key);bundle.total+=edge.total||1;bundle.edges.push(edge);
  });
  return Array.from(bundles.values()).sort(function(a,b){return architectureStringCompare(a.id,b.id);});
}
function architectureBundleGeometry(bundle,layout,index){
  const records=bundle.edges.map(function(edge){return {edge:edge,from:layout.positions.get(edge.from),to:layout.positions.get(edge.to)};}).filter(function(record){return record.from&&record.to;});
  if(!records.length)return null;
  const forward=records.reduce(function(total,record){return total+(record.to.x-record.from.x);},0)>=0,sourceLane=records[0].from.lane,targetLane=records[0].to.lane,gutter=Math.max(72,layout.step-layout.cardWidth);
  if(sourceLane===targetLane){
    const laneX=layout.margin+sourceLane*layout.step,cardRight=laneX+layout.cardWidth,count=bundle.total||1,badgeLabel=count+" call"+(count===1?"":"s"),badgeHalfWidth=Math.max(58,badgeLabel.length*7+18)/2,trunkHalfStroke=1.2,branchHalfStroke=.675,railInset=Math.max(badgeHalfWidth,trunkHalfStroke,branchHalfStroke),rightRailX=cardRight+gutter*.34,leftRailX=laneX-gutter*.34,rightFits=rightRailX+railInset<=layout.width,leftFits=leftRailX-railInset>=0,attachRight=rightFits||(!leftFits&&layout.width-cardRight>=laneX),railX=rightFits?rightRailX:leftFits?leftRailX:Math.max(railInset,Math.min(layout.width-railInset,attachRight?rightRailX:leftRailX)),branches=records.map(function(record){const source={x:attachRight?record.from.x+record.from.w:record.from.x,y:record.from.y+record.from.h/2},target={x:attachRight?record.to.x+record.to.w:record.to.x,y:record.to.y+record.to.h/2};return {edge:record.edge,sourcePath:"M"+source.x+" "+source.y+" C"+railX+" "+source.y+" "+railX+" "+source.y+" "+railX+" "+source.y,targetPath:"M"+railX+" "+target.y+" C"+railX+" "+target.y+" "+target.x+" "+target.y+" "+target.x+" "+target.y,target:target};}),branchYs=branches.flatMap(function(branch){return [layout.positions.get(branch.edge.from).y+layout.cardHeight/2,layout.positions.get(branch.edge.to).y+layout.cardHeight/2];}).sort(function(a,b){return a-b;});
    return {trunkPath:"M"+railX+" "+branchYs[0]+" L"+railX+" "+branchYs[branchYs.length-1],branches:branches,badge:{x:railX,y:branchYs[Math.floor(branchYs.length/2)]}};
  }
  const sourceTrunkX=forward?layout.margin+sourceLane*layout.step+layout.cardWidth+gutter*.34:layout.margin+sourceLane*layout.step-gutter*.34;
  const targetTrunkX=forward?layout.margin+targetLane*layout.step-gutter*.34:layout.margin+targetLane*layout.step+layout.cardWidth+gutter*.34;
  const ys=records.flatMap(function(record){return [record.from.y+record.from.h/2,record.to.y+record.to.h/2];}).sort(function(a,b){return a-b;}),trunkY=ys[Math.floor(ys.length/2)]+index%3*10;
  const branches=records.map(function(record){const source={x:forward?record.from.x+record.from.w:record.from.x,y:record.from.y+record.from.h/2},target={x:forward?record.to.x:record.to.x+record.to.w,y:record.to.y+record.to.h/2};return {edge:record.edge,sourcePath:"M"+source.x+" "+source.y+" C"+sourceTrunkX+" "+source.y+" "+sourceTrunkX+" "+trunkY+" "+sourceTrunkX+" "+trunkY,targetPath:"M"+targetTrunkX+" "+trunkY+" C"+targetTrunkX+" "+target.y+" "+target.x+" "+target.y+" "+target.x+" "+target.y,target:target};});
  return {trunkPath:"M"+sourceTrunkX+" "+trunkY+" L"+targetTrunkX+" "+trunkY,branches:branches,badge:{x:(sourceTrunkX+targetTrunkX)/2,y:trunkY}};
}
`
