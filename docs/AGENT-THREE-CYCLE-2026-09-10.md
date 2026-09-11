# GoreGraph: drei Fehlerbehebungs- und Prüfrunden vom 10.09.2026

Alle drei beauftragten Runden sind abgeschlossen. Die lokale Version ist weiterhin
**1.4.1 als Entwicklungsstand**, kein Release. Die gezielten Fehler sind behoben
und geprüft; die vollständigen Qualitäts- und Effizienzziele sind noch nicht alle
erreicht.

## Vergleich mit der gespeicherten Referenz

Effektive Tokens = ungecachter Input plus Output. Reasoning-Tokens sind bereits
im Output enthalten und werden nicht nochmals addiert. Das ist keine Kostenrechnung.
„Testdateien“ zählt korrekt genannte relevante Referenzdateien, keine bestandenen
oder fehlgeschlagenen Softwaretests.

| Lauf | Zeit | Effektive Tokens | Zeit gegenüber Referenz | Tokenersparnis | Kernkriterien | Testdateien |
|---|---:|---:|---:|---:|---:|---:|
| Referenz ohne GoreGraph, unverändert wiederverwendet | 10:00 min | 165.839 | – | – | 12/12 | separate gespeicherte Bewertung |
| Runde 1 | 6:54 min | 88.443 | −31,0 % | 46,7 % | 12/12 | 5/7 |
| Runde 2 | 7:22 min | 88.411 | −26,3 % | 46,7 % | 12/12 | 4/7 |
| Runde 3, aktuell installiert | 8:06 min | 108.888 | −19,0 % | 34,3 % | 12/12 | 5/7 |

Die neue Messreihe führt ausschließlich GoreGraph-Läufe aus. Die 15 Dateien der
Referenz `local-update-baseline-cli` bleiben bytegleich. Aufgabe, Modell,
Einstellungen und eingefrorener Workspace bleiben gleich; die Adaptive-Anleitung
ändert sich in Runde 2. Die vom Agenten formulierten Suchanfragen variieren.
Die Werte beweisen deshalb keine isolierte Ursache-Wirkung einzelner Patches und
keine allgemeine Effizienz für beliebige Aufgaben.

## Umgesetzte Änderungen

1. Adaptive Korrekturpläne und Wiederholungsverhalten werden als Evidenzbedarf
   erkannt, ohne daraus einen belegten fehlenden Laufzeitaufruf zu machen.
2. Anbieterdateien können anhand bereits ausgewählter exakter Modell-/Vertragsfakten
   oder einer exakten Vertragsdeklaration im tatsächlich gelieferten aktuellen
   Quellausschnitt zugeordnet werden. Projekt, Pfad, Zeilenbereich, Genauigkeit
   und Ausschluss von Test-/Konfigurationsquellen begrenzen die Auswahl.
3. Die Adaptive-Anleitung verlangt getrennte relevante Produktions-, Konfigurations-
   und Testinventare sowie engere Inhaltssuchen nach der Dateifindung. Bereits
   ausgegebene Suchtreffer sollen beim Nachlesen ausgespart werden. Die Messung
   zeigt, dass diese Anleitung noch nicht zuverlässig eingehalten wird.
4. Bei einem expliziten adaptiven Korrekturplan mit genau einem ausgewählten
   DELETE-Einstieg erhält der primäre Lösch-/Cleanup-Test den vorhandenen
   Prioritätsbonus. Ein allgemeiner Retry-Test verdrängt ihn dadurch nicht mehr.
   Strict v1, andere Methoden, mehrdeutige Einstiege und die alte Missing-Transition-
   Behandlung bleiben unverändert.

Die konkrete Anfrage aus Runde 2 liefert nach Runde 3 den Management-Test und den
Housekeeping-Test statt des allgemeinen Retry-Tests: 3,74 Sekunden, 3.987/4.000
Tokens, ursprünglicher Persistenzaufruf weiterhin enthalten. Die frühere
Korrekturplan-Kontrolle gewann in Runde 1 zwei Testhinweise; ihre lokale Laufzeit
stieg gegenüber der vorherigen Einzelmessung von etwa 2,67 auf 4,66 Sekunden.
Das wird nicht als lokale Beschleunigung ausgegeben.

## Prüfung und Nachvollziehbarkeit

- Vollständige Go-Suite, `go vet`, gezielte Regressionen, Formatierung und Diff-Prüfung bestanden.
- Abschließend 21 Suchkontrollen mit 152 Prüfungen bestanden: 20 feste Kontrollen
  plus der neue Einstieg aus Runde 2. Historische Strict-Ausgaben, Budgets,
  primäre Mutation, Konfidenz-/Projektgrenzen und Quellenintegrität geprüft.
- Aufgabenreviews und Gesamt-Review abgeschlossen. Gefundene Grenzfallfehler bei
  Genauigkeit, Test-/Konfigurationspfaden und Strict-Kompatibilität wurden vor
  Installation korrigiert; verworfene Kandidaten bleiben archiviert.
- Alle 374 eingefrorenen Service-Dateien und ihre Scan-Indizes sind unverändert.
  Ein Neuscan war nicht erforderlich. Die Testservices wurden nicht gebaut,
  verändert oder zur Laufzeit getestet; die Codex-Läufe analysieren Quellen lesend.
- Keine Commits, Veröffentlichung, Merge oder Release. Vorherige Änderungen in
  der Arbeitskopie bleiben erhalten.

| Lauf | Befehle | Zurückgegebene Bytes | Letzte Quellantwort | Danach bis Abschluss | Nachgewiesene erneut gelesene Zeilen |
|---|---:|---:|---:|---:|---:|
| 1 | 44 | 245.509 | 283,31 s | 130,81 s | mindestens 168 |
| 2 | 44 | 165.368 | 330,69 s | 111,76 s | mindestens 236 |
| 3 | 66 | 220.238 | 320,95 s | 165,00 s | mindestens 175 |

Die Wiederholungen werden nur gezählt, wenn ausgegebene Zeilen mit den unveränderten
Quellen übereinstimmen; nicht unterstützte/mehrdeutige Leser bleiben ausgeschlossen.
Runde 2 enthält einen korrigierten falschen projektbezogenen Pfad (Exit 2). Die
vier Exit-1-Suchen in Runde 3 sind leere, begrenzte Suchergebnisse ohne Fehlerausgabe.

Die Zitierprüfung bestätigt 32/34/34 vorhandene Inventardateien und 40/48/47 gültige
Zeilenbereiche. In Runde 2 wurden HTTP-Statuscodes vom alten Prüfskript zunächst
als Zeilen missverstanden; in Runde 3 kürzt der Anzeigetext Pfade, die vollständigen
Markdown-Ziele sind jedoch korrekt. Die ergänzende Zielprüfung und manuelle Prüfung
lösen diese Prüfskript-Fehlalarme auf. Originalantworten bleiben unverändert.

## Verbleibende Probleme

- Zwei relevante Mail-Testreferenzen fehlen weiterhin im letzten Ergebnis.
- Quellen werden weiterhin wiederholt gelesen; gezielte Hinweise und Anleitung
  allein verhindern das nicht zuverlässig.
- Der letzte Lauf benötigt mehr Zeit/Tokens als Runde 1/2 und verfehlt den vorab
  gesetzten Höchstwert von 105.403 effektiven Tokens trotz Gewinn zur Referenz.
- Dateinavigation hängt weiterhin von der Formulierung und ausgewählten Belegen ab;
  fehlende Repository-Hinweise bei deutscher Fachsprache bleiben dokumentiert.
- 12/12 Kernkriterien ersetzen weder vollständige Testdatei-Abdeckung noch eine
  Laufzeitprüfung unbekannter Datenbankeffekte, Deployments und Nebenläufigkeit.

Frühes Beenden hätte gleichzeitig 12/12 Kernkriterien, 7/7 korrekt zugeordnete
Testreferenzen, keine nachgewiesenen Wiederholungen/Scope-Verstöße, höchstens
600,300493 Sekunden und höchstens 105.403 effektive Tokens sowie bestandene
Code-/Integritätsprüfungen erfordert. Das gelang nicht; daher wurden alle drei
Runden ausgeführt. Adaptive v2 bleibt opt-in, keine allgemeine Release-Freigabe.

## Installierter Stand und Artefakte

Version: `1.4.1`, Commit-Kennung: `d67d1f4ab3c3-dirty`, Build: `2026-09-10T20:58:40Z`.

SHA-256: `c9625c6a9873e78a1a1beebc5c570ecdaebad189aa6de0dea9799f0e125a0546`.

Installiert unter `/Users/gorecode/go/bin/goregraph` und
`/opt/homebrew/bin/goregraph-local`; der vorhandene Symlink bleibt erhalten.
Sicherungen und exakte Installationsdaten liegen in den jeweiligen
`three-cycle-N-final/install-record.json`.

Private Rohantworten, Zeit-/Tokenprotokolle, Qualitäts- und Leseprüfungen:
/Users/gorecode/.codex/visualizations/2026/09/10/01a08a04-dfec-7502-a22a-af1ed717fd03/benchmark-0442483-macos

Dort liegen `three-cycle-1-cli`, `three-cycle-2-cli`, `three-cycle-3-cli`, die
Kandidaten/Backups unter `three-cycle-N-final` sowie abgelehnte Vorversuche.
Der Arbeitsnachweis liegt im Projekt unter
`.superpowers/sdd/2026-09-10-three-cycle-evidence/`; er wird nicht gelöscht, weil
noch keine Integration/Commit-Historie die uncommitteten Änderungen ersetzt.

## Getroffene Abwägungen

1. Bestehende Arbeitskopie mit Sicherungen weiterverwenden: vorherige Änderungen
   bleiben erhalten; die spätere Integration bleibt manuell.
2. Vorhandene Freigaben für Installation und Übermittlung nutzen: keine weitere
   Freigabeschleife für bereits autorisierte Aktionen; weitergehende Aktionen sind
   davon nicht gedeckt.
3. Dateihinweise von Beweisen für fehlende Aufrufe trennen: vermeidet unberechtigte
   Laufzeitaussagen, benötigt aber eine zusätzliche Auswahlbedingung samt Tests.
4. Runde 2/3 erst anhand der jeweiligen Messung konkret planen: keine erfundenen
   Folgefixes, dafür zusätzliche Diagnosezeit zwischen den Läufen.
5. Exakte Deklarationen in aktuellen gelieferten Ausschnitten für Navigation nutzen:
   erhält die Bedeutung der ursprünglichen Faktenliste. Mögliche Fehlzuordnung
   wird durch Projekt-/Pfad-/Zeilen-/Konfigurations- und Negativtests begrenzt.
6. Bereits gelieferte Produktionspfade nicht doppelt ausgeben, nur um ein Prüffeld
   zu füllen. Der lokale Prüffall wertet Metadaten und vorhandene Belege gemeinsam;
   dadurch mögliche übersehene Repository-Lücken bleiben ausdrücklich offen und
   werden nicht als vollständiger Recall ausgegeben.
