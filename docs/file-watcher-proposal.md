# Dateiwächter: GoreGraph-Ausgaben nach Dateiänderungen aktualisieren

Status: Grundfunktion implementiert; mögliche Erweiterungen stehen unten
Stand: 2026-09-25

## Anlass und Ziel

Nach Änderungen an Frontend-Code, Tests oder Konfiguration können der Agent-Index
und das Dashboard einen älteren Stand zeigen. Heute aktualisiert
`goregraph update <root> --target all` beide Projektionen in einem Lauf. Ein
optionaler lokaler Dateiwächter führt diesen Befehl nach Änderungen aus,
ohne dass ein Codex-Agent den Aktualisierungsschritt übernehmen muss.

Der Wächter soll die Analyse nicht als inkrementelle Verarbeitung einzelner
Dateien ausgeben: Ein geändertes Projekt wird weiterhin vollständig analysiert.
Der Vorteil ist, dass die Aktualisierung bereits vor der nächsten Agent-Anfrage
oder Dashboard-Nutzung abgeschlossen sein kann.

## Beobachteter Umfang

- Prüfe Projektverzeichnisse auf angelegte, geänderte, umbenannte und
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

1. Erst nach einem manuellen Start für einen gewählten Projekt- oder
   Workspace-Root läuft der Wächter. Er prüft den vorhandenen Stand. Fehlt ein
   Index, baut er Agent-Index und Dashboard; andernfalls prüft er die aktuellen
   Eingaben.
2. Er prüft die vom Scanner ausgewählten Dateiinhalte alle drei Sekunden und
   wartet nach einer Änderung zwei Sekunden Ruhezeit, damit mehrere
   Speicheraktionen einen einzigen Aktualisierungslauf auslösen.
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

Installation und Updates von GoreGraph starten den Wächter nicht und richten
keinen Autostart ein. Der Nutzer startet ihn erstmals ausdrücklich für einen
Projekt- oder Workspace-Root. Bei diesem ersten Start wählt er zwischen „nur
jetzt“ (Vorgabe) und „bei künftiger Anmeldung am Rechner automatisch starten“.
Der Autostart gilt nur für ausdrücklich gewählte Roots, nicht für jeden Aufruf
der GoreGraph-CLI. Ohne interaktives Terminal bleibt Autostart aus, sofern er
nicht ausdrücklich als Option gesetzt wurde. Ein Autostart wird nur für den
aktuellen Benutzer eingerichtet, nicht als systemweiter Dienst.

Status und Autostart-Einstellung müssen einsehbar und nachträglich änderbar
sein. Der Wächter lässt sich jederzeit stoppen; das Entfernen des Autostarts
ist eine eigene, ausdrücklich sichtbare Aktion. Ein erneuter Start darf keinen
zweiten Wächter für denselben Root erzeugen. Nach Anmeldung, Neustart oder
Ruhezustand prüft er die aktuellen Eingaben, damit verpasste Datei-Ereignisse
nicht zu einem veralteten Index führen. Ein Git-Hook allein reicht nicht aus,
da er Änderungen vor einem Commit nicht erfasst. Ohne aktivierten Wächter
bleiben die bestehenden manuellen Update-Befehle gültig.

## Plattformen und Konfiguration

Manueller Start, Stop, Status und Autostart-Umschaltung sind unter Windows,
macOS und Linux gleich bedienbar. Die Inhaltsprüfung nutzt die Auswahl- und
Ausschlussregeln des Scanners; neue Unterverzeichnisse sind damit beim nächsten
Lauf enthalten. Nach Neustart oder Ruhezustand gleicht der Wächter die aktuellen
Inhalte erneut ab. Lesefehler erscheinen im Status, statt unbemerkt veraltete
Daten anzuzeigen.

Nur nach ausdrücklichem Opt-in richtet GoreGraph einen Autostart für den
aktuellen Benutzer ein:

- Windows: Aufgabe mit Anmeldetrigger in der Aufgabenplanung.
- macOS: benutzerspezifischer LaunchAgent.
- Linux: systemd-Benutzerdienst, sofern vorhanden; in einer passenden
  Desktop-Sitzung alternativ XDG-Autostart.

Die kleine benutzerspezifische Konfiguration speichert die ausdrücklich
gewählten Roots und die Autostart-Einstellung. Eine überschreibbare
Laufzeitdatei hält Herzschlag, letzten Lauf und Fehler fest; es entsteht kein
Dateiverlauf pro Änderung. Status zeigt den verwendeten Startmechanismus und
etwaige Einrichtungsfehler. Beim Ausschalten entfernt GoreGraph den eigenen
Autostart-Eintrag wieder. Ist unter Linux kein unterstützter Startmechanismus
verfügbar, meldet GoreGraph das und lässt den manuellen Betrieb zu. Installation
und Upgrade ändern bestehende Autostart-Einstellungen nicht eigenmächtig.

## Sichtbarkeit in der CLI-Hilfe

`goregraph help` beziehungsweise `goregraph --help` erwähnt den Wächter
weit oben vor den übrigen Befehlen. Ist
das aktuelle Verzeichnis einem Projekt oder Workspace zuzuordnen, zeigt die
Hilfe dafür getrennt den tatsächlichen Laufzustand und die Autostart-Einstellung,
beispielsweise `Dateiwächter: aus · Autostart: an`. Dazu nennt sie den Befehl zum
manuellen Start und den für ausführliche Statusinformationen. Die bloße
Autostart-Konfiguration darf nicht als „läuft“ angezeigt werden: Der Status
muss prüfen, ob der Wächterprozess für diesen Root tatsächlich aktiv ist.

Ohne eindeutig erkannten Root zeigt die Hilfe nur den Hinweis auf den Wächter
und den Statusbefehl mit explizitem Pfad, keinen geratenen Zustand. Hilfe und
Status bleiben schnell und lesend; sie starten weder Wächter noch Indexlauf.
Der ausführliche Status nennt Root, Laufzustand, Autostart, Startmechanismus,
letzten erfolgreichen Lauf und einen möglichen Fehler. Die Befehle heißen
`goregraph watch start|stop|status|autostart|run`.

## Mögliche Erweiterungen

- Datei-Ereignis-Backends als Ergänzung zur periodischen Inhaltsprüfung, falls
  große Workspaces einen schnelleren oder sparsameren Auslöser benötigen.
- Standard für Ruhezeit, CPU- und I/O-Aufwand sowie die maximale Häufigkeit von
  Aktualisierungen in großen Workspaces.
- Messen und Begrenzen der Polling-Kosten in sehr großen Workspaces.
- Ob und wie ein geöffnetes Dashboard Änderungen selbst erkennt.

## Akzeptanzkriterien

- Installation und Upgrade lassen den Wächter aus; die erste Aktivierung und
  jeder Autostart erfordern eine ausdrückliche Entscheidung des Nutzers.
- Manueller Betrieb und Status funktionieren unter Windows, macOS und Linux;
  Autostart lässt sich über den jeweils verfügbaren Benutzermechanismus
  einrichten und vollständig entfernen.
- Autostart kann später unabhängig vom laufenden Wächter ein- und ausgeschaltet
  werden; Status zeigt beides getrennt an.
- Die normale CLI-Hilfe zeigt den Wächter oben und für einen eindeutig erkannten
  Root den echten Laufzustand getrennt vom Autostart; sie löst keinen Scan aus.
- Eine Änderung an Frontend-Code oder Tests erneuert Agent-Index und Dashboard.
- Mehrere schnelle Speicheraktionen führen nicht zu parallelen Scans.
- Änderungen während eines Scans gehen nicht verloren.
- Generierte Dateien lösen keinen Kreislauf aus; unveränderte Projekte werden
  nicht neu analysiert.
- Fehler bleiben sichtbar, ohne gültige ältere Ausgaben als aktuell auszugeben.
