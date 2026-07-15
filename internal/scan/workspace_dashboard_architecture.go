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
function architectureDomains(nodes){
  const groups=new Map();
  (nodes||[]).forEach(function(node){const id=architectureDomainKey(node);if(!groups.has(id))groups.set(id,[]);groups.get(id).push(node);});
  return Array.from(groups.entries()).map(function(entry){const id=entry[0],domainNodes=entry[1].slice().sort(function(a,b){return String(a.label||a.project||a.id).localeCompare(String(b.label||b.project||b.id))||String(a.id).localeCompare(String(b.id));});return {id:id,label:architectureDomainLabel(id),color:architectureDomainColor(id),nodes:domainNodes};}).sort(function(a,b){return a.label.localeCompare(b.label)||a.id.localeCompare(b.id);});
}
function architectureLayout(nodes,width){
  const domains=architectureDomains(nodes),layoutWidth=Math.max(width||0,Math.max(1040,domains.length*300+84)),margin=42,cardWidth=224,cardHeight=74;
  const step=domains.length>1?(layoutWidth-margin*2-cardWidth)/(domains.length-1):0,positions=new Map();let maxLength=0;
  domains.forEach(function(domain,lane){maxLength=Math.max(maxLength,domain.nodes.length);domain.nodes.forEach(function(node,index){positions.set(node.id,{x:margin+lane*step,y:190+index*90,lane:lane,w:cardWidth,h:cardHeight,domain:domain.id});});});
  return {positions:positions,width:layoutWidth,height:Math.max(760,290+maxLength*90),domains:domains,step:step,cardWidth:cardWidth,cardHeight:cardHeight,margin:margin};
}
`
