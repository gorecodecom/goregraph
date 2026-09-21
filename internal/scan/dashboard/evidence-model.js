'use strict';
window.EvidenceModel = {
  summarize(edges) {
    const counts = {all:0, production:0, test:0, unknown:0, http:0, java:0};
    for (const edge of edges) for (const record of edge.records) {
      counts.all += record.count;
      counts[record.scope] += record.count;
      if (record.kind === 'http' || record.kind === 'java') counts[record.kind] += record.count;
    }
    return counts;
  },
  project(edges, scope = 'all', kind = 'all') {
    return edges.flatMap(edge => {
      const records = edge.records.filter(record => (scope === 'all' || record.scope === scope) && (kind === 'all' || record.kind === kind));
      if (!records.length) return [];
      const total = records.reduce((sum,record) => sum + record.count,0);
      const countStatus = predicate => records.filter(record => predicate(record.status)).reduce((sum,record) => sum + record.count,0);
      return [{...edge, records, total, originalTotal:edge.total,
        resolved:countStatus(status => status === 'RESOLVED'),
        unresolved:countStatus(status => status === 'UNRESOLVED'),
        mismatched:countStatus(status => /MISMATCH/.test(status)),
        unknownStatus:countStatus(status => !['RESOLVED','UNRESOLVED'].includes(status) && !/MISMATCH/.test(status))
      }];
    });
  }
};
window.EvidenceModel.domainPairs = function(edges, nodes) {
  const byId=new Map(nodes.map(node=>[node.id,node]));
  const pairs=new Map();
  for(const edge of edges) {
    const from=byId.get(edge.from)?.domain, to=byId.get(edge.to)?.domain;
    if(!from||!to) continue;
    const id=JSON.stringify([from,to]);
    if(!pairs.has(id)) pairs.set(id,{id,from,to,edges:[],callers:new Set(),targets:new Set(),total:0});
    const pair=pairs.get(id);
    pair.edges.push(edge);pair.callers.add(edge.from);pair.targets.add(edge.to);pair.total+=edge.total;
  }
  return [...pairs.values()].map(pair=>({...pair,callers:pair.callers.size,targets:pair.targets.size}));
};
