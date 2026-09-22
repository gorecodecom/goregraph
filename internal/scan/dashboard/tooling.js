'use strict';
// Tooling is an offline source inventory. It never upgrades declarations to run results.
const toolingGroups = {configuration:'Konfiguration',stories:'Stories & Daten',runner:'Test-Runner',visual:'Visuelle Tests',ci:'CI',documentation:'Dokumentation'};
function toolingGroup(source) {
  const file=source.file.toLowerCase();
  if(source.kind==='configuration'||file.includes('/.storybook/main.'))return 'configuration';
  if(source.kind==='ci')return 'ci';
  if(source.kind==='documentation')return 'documentation';
  if(file.includes('playwright')||file.includes('/visual'))return 'visual';
  if(source.kind==='package'||file.includes('vitest')||file.includes('setup')||file.startsWith('scripts/'))return 'runner';
  return 'stories';
}
function toolingSources(record) {
  const order=Object.keys(toolingGroups);
  return (record?.sources||[]).slice().sort((a,b)=>order.indexOf(toolingGroup(a))-order.indexOf(toolingGroup(b))||a.file.localeCompare(b.file));
}
function codeViewTabs() {
  return `<div class="module-tabs segmented code-view-tabs" aria-label="Service-Code-Ansicht">${[['symbols','Klassen & Verwendungen'],['tooling','Tests & Tooling']].map(([key,label])=>`<button data-code-view="${key}" aria-pressed="${state.codeView===key}" class="${state.codeView===key?'active':''}">${label}</button>`).join('')}</div>`;
}
function toolingObservation(note) {
  if(note.kind==='a11y_test_literal')return `A11y: test = ${note.value} deklariert`;
  if(note.kind==='allow_failure_literal')return `CI: allow_failure = ${note.value} deklariert`;
  return 'Konfigurationshinweis';
}
function toolingSourceButton(file,label,line,lineLabel='Fundstelle') {
  return `<button class="inline-link" data-tooling-file="${esc(file)}">${esc(label||file)}</button>${line?`<small>${esc(lineLabel)}: Zeile ${line}</small>`:''}`;
}
function toolingDetails(source,sources) {
  if(!source)return emptyState('Quelle auswählen','Wähle links eine Story, Konfiguration oder weitere Tooling-Datei.');
  const byFile=new Map(sources.map(item=>[item.file,item]));
  const outgoing=(source.references||[]).filter(ref=>byFile.has(ref.file));
  const incoming=sources.flatMap(item=>(item.references||[]).filter(ref=>ref.file===source.file).map(ref=>({...ref,file:item.file})));
  const codeSymbols=(symbolsByProject.get(selectedProject())||[]).filter(symbol=>symbol.declaration_file===source.file);
  const relationList=(links,empty,lineLabel)=>links.length?`<ul class="tooling-links">${links.map(ref=>`<li>${toolingSourceButton(ref.file,ref.file,ref.line,lineLabel)}<span>${ref.kind==='include'?'Lokaler CI-Include':ref.kind==='script'?'Skriptverweis':'Statischer Dateiverweis'}</span></li>`).join('')}</ul>`:`<p class="muted">${empty}</p>`;
  return `<section class="symbol-detail tooling-detail" aria-label="Tooling-Quelle und Verbindungen"><span class="detail-eyebrow">${esc(toolingGroups[toolingGroup(source)])}</span><h2>${esc(source.file.split('/').pop())}</h2>${fileReference(selectedProject(),source.file,1)}<p class="tooling-source-meta">${source.lines||0} Quellzeilen im Snapshot${source.topics?.length?' · '+source.topics.map(esc).join(' / '):''}</p>${codeSymbols.length?`<div class="tooling-symbol-links"><h3>Im Service-Code</h3>${codeSymbols.slice(0,12).map(symbol=>symbolLink(symbol)).join(' · ')}</div>`:''}${source.observations?.length?`<div class="tooling-observations"><h3>Deklarationen im Quelltext</h3>${source.observations.map(note=>`<div><strong>${esc(toolingObservation(note))}</strong><small>Zeile ${note.line}</small></div>`).join('')}<p>Gilt für die Fundstelle. Überschreibungen, wirksame Einstellungen und Pipeline-Ergebnisse sind damit nicht geprüft.</p></div>`:''}<h3>Verweist auf <span>${outgoing.length}</span></h3>${relationList(outgoing,'Keine aufgelösten lokalen Dateiverweise im Export.','Verweis in dieser Quelle')}<h3>Referenziert von <span>${incoming.length}</span></h3>${relationList(incoming,'Keine eingehenden Dateiverweise im Export.','Verweis in der genannten Datei')}<p class="data-note">Verbindungen zeigen statische Dateiverweise. Sie belegen weder die Testauswahl noch eine erfolgreiche Ausführung.</p>${source.unknown?.length?`<details class="tooling-unknown"><summary>${source.unknown.length} offene Zuordnung${source.unknown.length===1?'':'en'}</summary><ul>${source.unknown.map(issue=>`<li>${esc(toolingUnknown(issue))}</li>`).join('')}</ul></details>`:''}</section>`;
}
function toolingUnknown(issue) {
  if(issue.includes('external or conditional'))return 'Externer oder bedingter CI-Include: Wirkung nicht aufgelöst.';
  if(issue.includes('dynamic CI')||issue.includes('inline or dynamic'))return 'Dynamischer oder komplexer CI-Include: Quelle manuell prüfen.';
  if(issue.includes('missing or excluded'))return 'Lokale Referenz fehlt im Index oder ist ausgeschlossen.';
  if(issue.includes('ambiguous'))return 'Lokale Referenz ist mehrdeutig.';
  if(issue.includes('limit reached'))return 'Analysegrenze erreicht; weitere Referenzen können fehlen.';
  return issue;
}
function renderTooling() {
  const record=workspace.tooling[selectedProject()];
  const heading=moduleHeading('Service-Code','Tooling-Quellen und ihre Zusammenhänge im ausgewählten Projekt.',false)+codeViewTabs();
  if(record?.version!==1){document.getElementById('content').innerHTML=heading+emptyState('Tooling-Daten nicht verfügbar','Dieser Export enthält für das Projekt noch keine Tooling-Projektion. Das Dashboard mit der aktuellen GoreGraph-Version neu erzeugen.');return;}
  const sources=toolingSources(record);
  if(!sources.length){document.getElementById('content').innerHTML=heading+emptyState('Keine passenden Tooling-Quellen im Index','Im Snapshot wurden keine passenden Storybook-, Playwright-, Vitest- oder CI-Quellen erfasst. Das belegt nicht, dass das Projekt keine Tests hat.');return;}
  const query=(state.toolingQuery||'').toLocaleLowerCase();
  const filtered=sources.filter(source=>(state.toolingGroup==='all'||toolingGroup(source)===state.toolingGroup)&&(!query||source.file.toLocaleLowerCase().includes(query)));
  if(!filtered.some(source=>source.file===state.toolingFile))state.toolingFile=filtered[0]?.file||null;
  const selected=filtered.find(source=>source.file===state.toolingFile);
  const issues=sources.reduce((n,source)=>n+(source.unknown||[]).length,0);
  const configs=sources.filter(source=>toolingGroup(source)==='configuration').length;
  const notes=sources.flatMap(source=>(source.observations||[]).map(note=>({source,note})));
  document.getElementById('content').innerHTML=heading+moduleStats([[sources.filter(source=>source.kind==='story').length,'Story-Dateien'],[record.total??sources.length,'Tooling-Quelldateien'],[configs,'Konfigurationsdateien']])+`<div class="tooling-summary"><span class="pill neutral">Statischer Snapshot</span><p>Quellen und lokale Verweise sind erfasst. Wirksame Aktivierung, erfolgreiche Testläufe und Referenzfreigaben sind nicht nachgewiesen.</p></div>${record.truncated?'<p class="notice">Die Quellenliste ist auf 2048 Dateien begrenzt. Weitere Quellen können im Index vorhanden sein.</p>':''}${notes.length?`<details class="tooling-highlights"><summary>${notes.length} Konfigurationshinweis${notes.length===1?'':'e'} · A11y und CI prüfen${notes.length>20?' (erste 20)':''}</summary><ul>${notes.slice(0,20).map(({source,note})=>`<li><strong>${esc(toolingObservation(note))}</strong>${toolingSourceButton(source.file,source.file,note.line)}</li>`).join('')}</ul></details>`:''}<div class="module-filter"><label class="module-search" for="tooling-search">Tooling-Quellen suchen<input id="tooling-search" type="search" placeholder="Story, Komponente oder Konfiguration …" value="${esc(state.toolingQuery)}"></label>${selector('tooling-group','Bereich',[['all','Alle Bereiche'],...Object.entries(toolingGroups)],state.toolingGroup)}</div><div class="code-workbench tooling-workbench"><section class="symbol-browser" aria-label="Tooling-Quellen"><header><h2>Quellen</h2><span>${filtered.length} Dateien</span></header><div class="symbol-list">${filtered.slice(0,state.toolingLimit).map(source=>`<button class="symbol-row ${source.file===state.toolingFile?'active':''}" data-tooling-file="${esc(source.file)}" aria-pressed="${source.file===state.toolingFile}"><span class="symbol-kind">${esc(toolingGroups[toolingGroup(source)])}</span><strong>${esc(source.file.split('/').pop())}</strong><small>${esc(source.file)}</small>${source.observations?.length?`<span class="tooling-note-label">${esc(toolingObservation(source.observations[0]))}</span>`:''}</button>`).join('')||emptyState('Keine Quellen im Filter','Suche oder Bereichsfilter anpassen.')}</div>${filtered.length>state.toolingLimit?`<button class="button load-more" data-module-action="more-tooling">Weitere Quellen (${filtered.length-state.toolingLimit})</button>`:''}</section>${toolingDetails(selected,sources)}</div><p class="data-note">Dateianzahlen sind keine Testfall- oder Coverage-Zahlen.${issues?' '+issues+' offene Referenzzuordnungen; Details stehen an der jeweiligen Quelle.':''} Erfasst sind ausgewählte indexierte Quellen, keine vollständige Laufzeit- oder CI-Analyse.</p>`;
}
function toolingClick(target) {
  const view=target.closest('[data-code-view]');
  if(view){state.codeView=view.dataset.codeView;renderCode();document.querySelector(`[data-code-view="${state.codeView}"]`)?.focus({preventScroll:true});return true;}
  const file=target.closest('[data-tooling-file]');
  if(file){state.toolingFile=file.dataset.toolingFile;if(!file.classList.contains('symbol-row')){state.toolingQuery='';state.toolingGroup='all';const sources=toolingSources(workspace.tooling[selectedProject()]);state.toolingLimit=Math.max(40,sources.findIndex(source=>source.file===state.toolingFile)+1);}renderTooling();document.querySelector('.tooling-detail h2')?.setAttribute('tabindex','-1');document.querySelector('.tooling-detail h2')?.focus({preventScroll:innerWidth>=1000});return true;}
  return false;
}
