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
function architectureDomains(nodes){
  const groups=new Map();
  (nodes||[]).forEach(function(node){const id=architectureDomainKey(node);if(!groups.has(id))groups.set(id,[]);groups.get(id).push(node);});
  return Array.from(groups.entries()).map(function(entry){const id=entry[0],domainNodes=entry[1].slice().sort(function(a,b){return architectureStringCompare(a.label||a.project||a.id,b.label||b.project||b.id)||architectureStringCompare(a.id,b.id);});return {id:id,label:architectureDomainLabel(id),color:architectureDomainColor(id),nodes:domainNodes};}).sort(function(a,b){return architectureStringCompare(a.label,b.label)||architectureStringCompare(a.id,b.id);});
}
function architectureCanvasGeometry(width,focusHeight){
  const compact=(width||0)<=1000,presentationTop=12,legendTop=compact?56:12,toolsTop=compact?100:12,focusTop=compact?144:96,resolvedFocusHeight=Math.max(44,focusHeight||0),focusBottom=focusTop+resolvedFocusHeight;
  return {compact:compact,presentationTop:presentationTop,legendTop:legendTop,toolsTop:toolsTop,focusTop:focusTop,focusBottom:focusBottom,contentInset:compact?focusBottom+24:0};
}
function architectureLayout(nodes,width){
  const domains=architectureDomains(nodes),layoutWidth=Math.max(width||0,Math.max(1040,domains.length*300+84)),margin=42,cardWidth=224,cardHeight=74,laneTop=118;
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
