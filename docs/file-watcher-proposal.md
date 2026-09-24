# Vorschlag: GoreGraph-Ausgaben nach Dateiänderungen automatisch aktualisieren

Status: Idee, noch nicht implementiert
Datum: 2026-09-24

## Anlass und Ziel

Nach Änderungen an Frontend-Code, Tests oder Konfiguration können der Agent-Index
und das Dashboard einen älteren Stand zeigen. Heute aktualisiert
`goregraph update <root> --target all` beide Projektionen in einem Lauf. Ein
optionaler lokaler Dateiwächter könnte diesen Befehl nach Änderungen ausführen,
ohne dass ein Codex-Agent den Aktualisierungsschritt übernehmen muss.

Der Wächter soll die Analyse nicht als inkrementelle Verarbeitung einzelner
Dateien ausgeben: Ein geändertes Projekt wird weiterhin vollständig analysiert.
Der Vorteil ist, dass die Aktualisierung bereits vor der nächsten Agent-Anfrage
oder Dashboard-Nutzung abgeschlossen sein kann.

## Beobachteter Umfang

- Beobachte Projektverzeichnisse auf angelegte, geänderte, umbenannte und
  gelöschte Dateien. Quellcode, Tests, Konfiguration und indexierte Dokumentation
  gehören dazu, soweit sie nach der jeweiligen GoreGraph-Konfiguration
  Scan-Eingaben sind.
- Nutze für die Entscheidung dieselben Projekt- und Ausschlussregeln wie der
  Scanner. Änderungen an `.git/`, generierten GoreGraph-Ausgaben,
  Abhängigkeitsverzeichnissen und Build-Ausgaben dürfen keine erneute
  Aktualisierung auslösen.
- Berücksichtige neue und entfernte Projektverzeichnisse in einem Workspace.
  Änderungen an der GoreGraph-Konfiguration oder den Ausschlussregeln müssen
  eine erneute Prüfung auslösen.

## Ablauf

1. Beim Start prüft der Wächter den vorhandenen Stand. Fehlt ein Index, baut er
   Agent-Index und Dashboard; andernfalls prüft er die aktuellen Eingaben.
2. Nach einem relevanten Datei-Ereignis wartet er eine kurze Ruhezeit, damit
   mehrere Speicheraktionen einen einzigen Aktualisierungslauf auslösen.
3. Für ein einzelnes Projekt führt er `goregraph update <root> --target all`
   aus. Für einen Workspace nutzt er
   `goregraph workspace update <root> --target all`: unveränderte Projekte
   werden anhand ihrer Inhalte übersprungen, geänderte Projekte vollständig
   neu aufgebaut.
4. Es läuft höchstens eine Aktualisierung gleichzeitig. Treffen währenddessen
   weitere Änderungen ein, merkt der Wächter sie vor und prüft nach dem Lauf
   erneut. Er ignoriert seine eigenen generierten Ausgaben.
5. Bei einem Fehler meldet er den Zustand sichtbar und behält den letzten
   gültigen Ausgabestand. Ein späteres Ereignis oder ein expliziter Neustart
   muss eine erneute Prüfung ermöglichen.

Der Wächter führt keinen Anwendungscode und keine Tests aus. Er braucht keinen
Netzwerkdienst. Ein bereits geöffnetes, statisch geladenes Dashboard muss nach
einer Aktualisierung gegebenenfalls neu geladen werden; automatisches Neuladen
wäre eine eigene Erweiterung.

## Leistung und Betrieb

Ein Dateiwächter verschiebt den Zeitpunkt der Arbeit, verkürzt aber nicht die
Analyse eines geänderten Projekts. `workspace update` prüft die Inhalte aller
erkannten Projekte und vermeidet die aufwendige Analyse unveränderter Projekte.
Echte inkrementelle Datei- und Abhängigkeitsaktualisierung wäre eine separate,
größere Änderung am Indexer.

Der Betrieb sollte ausdrücklich aktiviert und wieder beendet werden können,
entweder als laufender lokaler Prozess oder über einen Startmechanismus des
Betriebssystems. Ein Git-Hook allein reicht nicht aus, da er Änderungen vor
einem Commit nicht erfasst. Ohne aktivierten Wächter bleiben die bestehenden
manuellen Update-Befehle gültig.

## Vor der Umsetzung entscheiden

- Betriebssystemübergreifende Datei-Ereignisse oder regelmäßige Inhaltsprüfung;
  Verhalten bei verpassten Ereignissen und nach Ruhezustand.
- Standard für Ruhezeit, CPU- und I/O-Aufwand sowie die maximale Häufigkeit von
  Aktualisierungen in großen Workspaces.
- Einrichtung und Statusanzeige für einen dauerhaft laufenden Prozess.
- Ob und wie ein geöffnetes Dashboard Änderungen selbst erkennt.

## Akzeptanzkriterien

- Eine Änderung an Frontend-Code oder Tests erneuert Agent-Index und Dashboard.
- Mehrere schnelle Speicheraktionen führen nicht zu parallelen Scans.
- Änderungen während eines Scans gehen nicht verloren.
- Generierte Dateien lösen keinen Kreislauf aus; unveränderte Projekte werden
  nicht neu analysiert.
- Fehler bleiben sichtbar, ohne gültige ältere Ausgaben als aktuell auszugeben.
