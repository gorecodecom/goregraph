# Antwortprüfung und Mehrfachsuche — 11.09.2026

## Stand und Umfang

Entwicklungsstand 1.4.1, gebaut am 11.09.2026 um 12:57:46 UTC,
SHA-256 `fc815c8e387031ecccbebd4a426bd414e704be324389a8c2255df177db6734e6`.
Die lokale Installation ist aktualisiert. Dies ist kein Release.
Der diagnostische Lauf und der Bestätigungslauf sind abgeschlossen. Die geplanten
Leser-, Antwortprüfungs- und Anleitungskorrekturen sind implementiert und geprüft.

## Bestätigungslauf

Der unabhängige fachliche Review bestätigt **12/12 Kernkriterien und 7/7 exakte
Testreferenzen**. In diesem begrenzten Prüflauf wurde kein wesentlicher Fakten-,
Auswahl- oder Teststrategiefehler gefunden. Die HTTP-Verwechslung, der fehlende
Mehrschritt-Rollback-Nachweis und die unbedingte Mail-Beschreibung sind behoben.
Ein vorläufiger Einwand zur Benutzerrolle beim Mailversand wurde nach Prüfung
des vorgeschalteten Erstellerfilters zurückgenommen. Die zusätzliche Prüfung
des Reviewers wird nicht nachträglich als Quellenlektüre des CLI-Agenten gewertet.

| Messung | Gespeicherte Referenz ohne GoreGraph | Diagnose 1 | Bestätigung 2 |
| --- | ---: | ---: | ---: |
| CLI-Rohzeit | 600,30 s | 716,64 s | 627,33 s |
| Workflow einschließlich mechanischer Finalisierung | — | 716,87 s | 627,60 s |
| Effektive Tokens | 165.839 | 101.877 | 93.047 |
| Gesamttokens einschließlich Cache-Eingaben | 2.274.767 | 1.601.781 | 1.220.727 |
| Erste sichtbare Einstiegspunkt-Identität | 44,25 s | 17,85 s | 23,28 s |
| Letzte Werkzeugantwort | 370,80 s | 453,62 s | 463,98 s |
| Zeit danach bis zum Antwortabschluss | 229,50 s | 263,02 s | 163,35 s |
| Werkzeugbefehle | 23 | 44 | 23 |
| Zurückgewiesene Leseranfragen | — | 5 | 1 |

Der letzte Workflow spart **43,89 % effektive Tokens**, braucht aber weiterhin
**4,55 % mehr Zeit** als die gespeicherte Referenz. Gegenüber Diagnose 1 sinken
Workflow-Zeit um 12,45 % und effektive Tokens um 8,67 %. Das ist eine erfolgreiche
gezielte Verbesserung; ein allgemeiner Geschwindigkeitsgewinn ist nicht belegt.
Die Event-Zeitstempel für einzelne Befehle können gepuffert eintreffen und eignen
sich nicht für eine belastbare Aufteilung in CPU-, Netzwerk- und Modellzeit.

Die mechanische Finalisierung benötigt 0,120 Sekunden und keinen Modellaufruf.
Sie akzeptiert 47 explizite Dateiverweise und 85 zitierte Bereiche ohne Reparatur
oder Befund. Rohantwort und finale Antwort sind bytegleich. 37 Verweise haben
gelieferten Quelltext, vier schließen geschwärzte Schlüsselbereiche ein, sechs
belegen nur Dateimetadaten. Verdeckte Konfigurationswerte werden nicht bestätigt.

Ein separater Dateigrenzen-Parser meldet drei Abweichungen an terminalen Leerzeilen:
18/19, 240/241 und 65/66. Die Leser-API hat diese abschließenden leeren Zeilen
tatsächlich geliefert; es gibt dort keinen erfundenen nichtleeren Quelltext.
Die Rohmeldungen und die gesonderte Bewertung bleiben erhalten.

Der Leser besteht 16 von 17 Anfragen. Ein Ergebnis überschreitet 24 KiB und wird
erfolgreich kleiner wiederholt. 2.003 Quellzeilen werden ausgegeben und 213 durch
übertragene Belege übersprungen. In 17 der 23 Befehle sind 2.107
Quellzeilen bytegenau überprüft; darunter sind 22 wiederholte Zeilen vom ersten
Kontext zur späteren Controller-Abfrage. Dies ist eine belegte Untergrenze,
kein Nachweis vollständig vermiedener Doppel-Lektüre. Die neuen Mehrfachselektoren
werden in diesen beiden vollständigen Läufen nicht genutzt; ihre Korrektheit
ist durch die separat nachgespielten drei realen Fehlanfragen belegt.

Fünf Testrollen sind durch gelieferte Aktionen und Assertions gestützt; zwei
Mailtests bleiben korrekt als nur über Metadaten bekannte Referenzen gekennzeichnet.
Die 39 Dateiinventareinträge und eine zusätzliche zitierte Sicherheitsdatei haben
existierende vollständige Pfade. Laufzeitverhalten der vorgeschlagenen Service-
Korrektur ist damit nicht verifiziert: Der Testworkspace blieb absichtlich statisch.

## Erster diagnostischer Lauf

Dieser Lauf verwendete den Kandidaten `22991c1e8be4…` vom selben Tag,
12:32:57 UTC. Rohzeit 716,638827 Sekunden; Workflow einschließlich mechanischer Prüfung
716,872930 Sekunden; 101.877 effektive Tokens. Das sind 38,57 % weniger effektive
Tokens und 19,42 % mehr Workflow-Zeit als die gespeicherte Referenz.
44 Werkzeugbefehle enthalten fünf zurückgewiesene Leseranfragen und eine Suche
ohne Treffer. Der erste Einstieg ist nach 17,85 Sekunden sichtbar; die letzte
Werkzeugantwort liegt bei 453,62 Sekunden. Die Schlussantwort benötigt danach
weitere 263,02 Sekunden.

Die mechanische Prüfung benötigt 0,081 Sekunden, ergänzt keinen Pfad und meldet
fünf Fehler. Die Untersuchung zeigt fünf Fehlalarme: vier tatsächlich gelieferte
geschwärzte Konfigurationsbereiche fehlen im extrahierten Belegverzeichnis,
und eine gültige Verbindung zwischen zwei Markdown-Links wird abgewiesen.
Die ursprüngliche Antwort, der fehlgeschlagene Prüfbericht und alle Messwerte
bleiben erhalten. Parser und Belegextraktion wurden gezielt korrigiert. Ein
gesonderter Replay mit denselben Ausgaben und derselben Antwort besteht danach
mit null Fehlern und null Änderungen. Er ersetzt weder den fehlgeschlagenen
Originallauf noch seine fachliche Bewertung.

Alle sieben bekannten Testpfade sind vollständig vorhanden. Der unabhängige
fachliche Review vergibt 12/12 Abdeckungspunkte, findet jedoch eine falsche
Verknüpfung von wiederholtem HTTP-Status und notwendiger verteilter Koordination
und einen unvollständigen lokalen Rollback-Test sowie eine fehlende Bedingung bei
der Mail-Beschreibung. Die neue Anleitung trennt idempotente Wirkungen von
identischen HTTP-Antworten, verlangt Zustandsprüfungen nach späteren Schreibfehlern
und den Erhalt beobachteter Bedingungen für Nebenwirkungen. Sie nennt außerdem
die aggregierten Lesergrenzen und verlangt kleine, vorab aufgeteilte Anfragen.

Der Folgelauf erhält einen eigenen Kandidaten und Vertrag. Diese zwei Analysen
werden deshalb nicht als unverändertes Kandidatenpaar bezeichnet.

Die Korrekturen bestehen den unabhängigen Review und ihre gezielten Pakettests.
Elf allgemeine Extraktor-Regressionen bestehen; der ursprüngliche Extraktor und
seine Messartefakte bleiben erhalten. Retrieval- und Leserquellen sind gegenüber
dem mit 180 Kontrollen geprüften Kandidaten unverändert; zwei erneute Kontext-
Prüfungen auf dem abschließend installierten Build liefern bytegleiche Antworten.

## Ursache und Korrektur

Mehrere Suchselektoren für dieselbe Datei wurden bisher zurückgewiesen.
`read` führt sie jetzt unabhängig aus und vereinigt ihre gelieferten Ausschnitte.
Jeder Selektor behält Suchmuster, Cursor und Trefferlimit. `find_results` ordnet
die Ergebnisse über den ursprünglichen `request_index` zu. Die Quelle wird nur
einmal gelesen; überlappende Ausschnitte und übertragene Belege werden vereinigt.
Einzelanfragen behalten ihr Ausgabeformat. Ungültige reguläre Ausdrücke erhalten
eine Fehlermeldung mit Anfrageindex und Hinweisen zur notwendigen Maskierung.

Für abschließende Antworten gibt es den neuen Befehl `answer-check`. Er prüft
explizite Markdown-Dateiverweise und Zeilenangaben gegen ein vom Aufrufer
übergebenes Verzeichnis tatsächlich entdeckter Dateien und gelieferter Bereiche.
Eindeutig zuordenbare Kurzpfade können ersetzt werden. Mehrdeutige Pfade,
ungelesene Zeilen und nicht sicher zuordenbare Zitate bleiben sichtbare Fehler.
Fachliche Aussagen, Zeilennummern und Testvorschläge werden nicht umgeschrieben.
Die [Integrationsanleitung](AGENT-ANSWER-CHECK.md) beschreibt Grenzen und Aufruf.

Die adaptive Anleitung verlangt vollständige Dateiziele und widerspruchsfreie
Testbedingungen: Ausgangsdaten, Auswahlprädikat und erwartetes Fehlerverhalten
müssen zusammenpassen. Der strikte Leitfaden bleibt bytegleich.

## Lokale Verifikation

- Vollständige Go-Suite bestanden (142,46 Sekunden); `go vet ./...` bestanden.
  Danach gefundene Parserfehler wurden mit zusätzlichen RED/GREEN-Regressionen,
  gezielten Pakettests, Race-Prüfungen und vet überprüft.
- Unabhängiger Code-Review bestanden. Drei Korrekturrunden decken unter anderem
  maskierte Zeilenzellen, ungültige Bereichsfortsetzungen, lokale Dateilinks ohne
  Erweiterung, FIFO-Eingaben und Semikolonlisten ab.
- Drei zuvor gescheiterte Mehrfachsuchen funktionieren. Ihre Ausgabe entspricht
  exakt der Vereinigung der unabhängigen Einzelabfragen, ohne doppelte Zeilen.
- 23 Kontextantworten sind bytegleich zum vorher installierten Kandidaten.
  180 Kontrollen, 115 wiederverwendete Quellabschnitte und 55 Suchaufrufe bestehen.
- Beim Nachspielen einer alten Antwort werden 32 Kurzpfade eindeutig ergänzt.
  Bei einer anderen bleiben zwei tatsächlich mehrdeutige Profilnamen markiert.

## Messverfahren

Die neue Messung verwendet dieselbe Aufgabe, dasselbe Modell, dieselben
CLI-Argumente und den eingefrorenen Testworkspace. Die adaptive Anleitung enthält
die beschriebenen Änderungen. Die gespeicherte Messung ohne GoreGraph bleibt
unverändert: 600,300493 Sekunden und 165.839 effektive Tokens. Es gibt keinen
neuen Baseline-Lauf.

Nach der CLI-Analyse extrahiert der Messrunner einen Quellenbeleg aus den
tatsächlichen erfolgreichen Werkzeugausgaben und ruft `answer-check` auf.
Rohantwort und Rohmessung bleiben erhalten. Die mechanische Finalisierung wird
separat und in der gesamten Workflow-Zeit gemessen; sie benötigt keinen weiteren
Modellaufruf. Ein fehlerhafter Prüfbericht wird nicht als fertige Antwort ausgegeben.
Dieser zusätzliche Schritt gehört zum neuen Workflow; die alte Messung hatte ihn
nicht. Er wird nicht automatisch in beliebige externe Codex-Aufrufe eingeschleust.

Die fachliche Bewertung erfolgt danach unabhängig und ist wie bei den früheren
Messungen keine Laufzeit des untersuchten Analyseworkflows. Ein mechanischer
Prüfer kann weder die Herkunft eines vom Aufrufer gelieferten Belegverzeichnisses
authentifizieren noch eine Ursachenanalyse oder Teststrategie als korrekt bestätigen.

Der unabhängige Review des Messrunners bestätigt die getrennte Rohantwort und
die Zeit-/Tokenzählung. Die erste Extraktorversion übersah nicht unterstützte
Ausgabeformen ohne Diagnose. V2 erkennt zusätzlich eng begrenzte Ein-Datei-
Suchausgaben und protokolliert nicht unterstützte Formen. Auch diese Diagnose
beweist keine vollständige Extraktion. Die belegten Bereiche stammen aus
überprüftem Quelltext; ausgelassene Formen können eine zusätzliche manuelle
Prüfung erfordern. Für den Laufzeitvergleich ist
`workflow_seconds` maßgeblich, nicht der Rohzeit-Prozentsatz des alten Collectors.

Alle 374 Quelldateien, Indexdateien und 15 Baseline-Artefakte bleiben unverändert.
Ein neuer Scan war nicht nötig. Die Services wurden weder verändert noch gebaut
oder getestet. Die privaten Messdaten liegen im vorhandenen Mac-Testverzeichnis
unter `answer-guard*`; der nicht installierte Vorabkandidat bleibt dort erhalten.

## Entscheidungen und verbleibende Grenzen

Bestehende Arbeitskopieänderungen wurden mit einem Vorher-Abzug erhalten; ein
selektives Zurücknehmen benötigt deshalb das gesicherte Delta. Die mechanische
Prüfung bestätigt keine Semantik; dafür bleibt eine gesonderte Bewertung nötig.
Die unveränderte historische Baseline und die separat gemessene Finalisierung
ermöglichen eine transparente Gegenüberstellung, jedoch keine Behauptung eines
identischen alten Workflows oder eines allgemeinen Geschwindigkeitsgewinns.

Offen bleiben die oben quantifizierten Größen-/Wiederholungsfälle und der kleine
Zeitnachteil. Die breitere Entwicklungs-Akzeptanz und Release-Reife werden durch
diesen einzelnen erfolgreichen Bestätigungslauf nicht neu bewertet. Kein Commit,
Merge, Release oder weiterer Lauf ohne GoreGraph wurde ausgeführt.
