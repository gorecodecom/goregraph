# CLI-Leserfehler und präzisere Quellenbelege

Stand: 11.09.2026. Umsetzung, lokale Installation und zwei unveränderte Codex-CLI-Läufe sind abgeschlossen. **Die reproduzierten CLI-Schemafehler sind behoben. Die neuen Antworten haben präzisere Quellenbelege, aber die Gesamtqualität verbessert sich nicht durchgängig: 12/12 und 11/12 Kernkriterien, verkürzte Pfade und ein langsamerer zweiter Lauf.**

## Gemessener Vergleich

| Messgröße | Gespeichert ohne GoreGraph | Lauf 1 | Lauf 2 |
|---|---:|---:|---:|
| Laufzeit | 600,30 s | 582,26 s | 680,44 s |
| Erster belegter Einstieg | 44,25 s | 13,36 s | 25,89 s |
| Letzte Werkzeugantwort | 370,80 s | 353,79 s | 448,07 s |
| Zeit danach bis Abschluss | 229,50 s | 228,47 s | 232,37 s |
| Eingabetokens | 2.258.257 | 1.500.410 | 2.251.265 |
| Davon gecachte Eingabe | 2.108.928 | 1.422.848 | 2.168.064 |
| Ausgabetokens | 16.510 | 26.087 | 27.527 |
| Gesamttokens inklusive Cache | 2.274.767 | 1.526.497 | 2.278.792 |
| Effektive Tokens | 165.839 | 103.649 | 110.728 |
| Befehle | 23 | 29 | 36 |
| Fachliche Kernkriterien | 12/12 historisch | 12/12 | 11/12 |

Effektive Tokens sinken gegenüber der gespeicherten Referenz um **37,50 % / 33,23 %**. Die Laufzeit verändert sich um **-3,01 % / 13,35 %** (Minus bedeutet schneller).

Der Mittelwert der zwei Läufe, zugleich ihr Median, beträgt **631,35 Sekunden und 107.188,5 effektive Tokens**. Das entspricht 5,17 % Laufzeitveränderung und 35,37 % weniger effektiven Tokens gegenüber der historischen Referenz. Gegenüber dem vorherigen vollständigen GoreGraph-Paar (567,10 s / 110.533,5 effektive Tokens im Mittel) sind es 11,33 % Laufzeit- und -3,03 % Tokenveränderung.

Effektive Tokens sind Eingabe minus gecachte Eingabe plus Ausgabe. Reasoning ist bereits in Ausgabe enthalten. Keine Geldersparnis wird daraus abgeleitet. Zwei sequenzielle Läufe gegen eine gespeicherte historische Referenz sind kein statistisch belastbarer allgemeiner Leistungsnachweis.

## Qualität: Belege präziser, vollständige Pfade nicht stabil

Die unabhängige, unverblindete statische Prüfung verwendet unverändert zwölf Kernkriterien und sieben bekannte Testreferenzen. Sie ersetzt keinen Testlauf der Services.

| Prüfung | Lauf 1 | Lauf 2 |
|---|---:|---:|
| Fachliche Kernkriterien | 12/12 | 11/12 |
| Erkennbare Testidentitäten und Rollen | 7/7 | 7/7 |
| Testreferenzen mit exakt ausgeschriebenem vollständigem Pfad | 6/7 | 1/7 |
| Tatsächliche Tabellenbereiche mit gelieferten Belegen nach Zuordnung | 51/51 | 74/74 |

**Lauf 1:** Die Mail-Controller-Testdatei erscheint nur mit ihrem Dateinamen in einer Metadatenliste; der vollständige Pfad war im Lauf entdeckt worden. Beide Mail-Referenzen weisen korrekt darauf hin, dass ihre Inhalte nicht geprüft wurden. Fünf weitere bekannte Testreferenzen sind durch Aktionen und Assertions belegt. Die feste Kernbewertung bleibt erfüllt, die vollständige Pfadausgabe ist trotzdem unzureichend.

Zwei vorgeschlagene Regressionstests sind ungenau: Aufgaben anderer Benutzer/Zustände dürfen nur dann erhalten bleiben, wenn sie zu einem anderen fachlichen Schlüsselpaar gehören. Ein Test lokaler Atomizität muss bei partieller Löschung fehlschlagen; bloßes Beobachten des Datenbankverhaltens genügt nicht als Akzeptanzkriterium. Die Korrekturplanung ist daher nicht unmittelbar als fehlerfreie Testspezifikation verwendbar.

**Lauf 2:** 32 von 38 Inventarzeilen enthalten wörtlich verkürzte Pfade mit `/.../` ohne vollständiges Linkziel. Die Dateien lassen sich aus den tatsächlich entdeckten, unveränderten Quellen eindeutig zuordnen; das macht die ausgegebenen Pfade aber nicht vollständig. Deshalb scheitert das feste Kriterium 11 für korrekte vorhandene Produktions-, Konfigurations- und Testpfade. Alle sieben Testidentitäten und ihre Rollen bleiben erkennbar; fünf sind inhaltlich gelesen, zwei korrekt als Metadatenreferenzen gekennzeichnet.

Auch Lauf 2 formuliert den Zusammenhang zwischen HTTP 404 und Idempotenz zu ungenau: Gleiche Statuscodes sind keine Voraussetzung für Idempotenz. Relevant ist hier die Wiederaufnahme nach einer teilweise abgeschlossenen Löschung. Eine DEV-Konfigurationsdatei wird außerdem zu pauschal als nur per Dateisuche bekannt bezeichnet, obwohl geschwärzte Datasource-Schlüssel vorlagen; taskbezogene Werte wurden dadurch nicht belegt.

### Rohprüfungen und ergänzende Bewertung

Im ersten Lauf zählt der rohe Parser 53 Tabellenbereiche. Davon sind zwei ausdrücklich HTTP-Statuscodes (403/404) vor den eigentlichen `Z.`-Angaben. Nach Ausschluss dieser Fehlinterpretationen sind alle 51 tatsächlichen Zitatbereiche vollständig in gelieferten Quellabschnitten enthalten. Vier Bereiche umfassen die leere EOF-Zeile: 47/51 liegen innerhalb physischer Inhaltszeilen, 51/51 innerhalb der bestehenden API-Zählung.

Im zweiten Lauf kann die rohe Prüfung die verkürzten Pfade nicht zuordnen und meldet 0/74 Bereiche als vollständig geliefert. Eine getrennte Diagnose ordnet ausschließlich eindeutige, im tatsächlichen Lauf entdeckte Pfade zu: 60 Java-Bereiche sind vollständig geliefert. Die weiteren 14 Konfigurationsbereiche wurden als nummerierte, geschwärzte Ausgabe geliefert; Schlüssel und Zeilenpositionen sind belegt, Werte und Laufzeitkonfiguration bleiben unbekannt. 63/74 Bereiche liegen innerhalb physischer Inhaltszeilen, 74/74 innerhalb der bestehenden API-Zählung. Die Rohantwort und die Rohprüfungen wurden nicht nachträglich repariert.

Im vorherigen Paar enthielten 5 von 69 beziehungsweise 4 von 53 Tabellenzitaten tatsächlich ungelesene Zwischenzeilen. Solche verdeckten Java-Zitatlücken sind in diesem Paar nach korrekter Zuordnung nicht mehr nachgewiesen. Das ist eine Verbesserung dieses Merkmals, keine allgemeine Fehlerfreiheit. Insbesondere bleiben die Pfadverstöße und die Schwächen vorgeschlagener Tests sichtbar. Oracle-Implementierung, DDL/Kaskaden, externe Konfiguration und verteilte Atomizität sind weiterhin nicht nachgewiesen.

## Umsetzung und technische Prüfung

Die CLI akzeptiert jetzt zwei zuvor tatsächlich fehlgeschlagene, eindeutig normalisierbare Schreibweisen: `start_line` plus `end_line` auf Dateiebene wird zu `ranges`; ein `start_line` neben `find` wird zum Suchcursor innerhalb von `find`. Die dokumentierte kanonische Schreibweise bleibt bevorzugt. Mehrdeutige Kombinationen, ungültige Zahlen und unbekannte Felder werden weiterhin abgewiesen, bevor Quellen ausgegeben werden.

Die adaptive Anleitung verlangt getrennte Zitate für getrennt gelieferte Ausschnitte, konkrete Testaktionen und Assertions für Aussagen über Testverhalten sowie Kontrollfluss-/Zustandsbelege für Wiederholung und Atomizität. Dateinamen und Testklassen bleiben Navigationshinweise. Die Anleitung enthält keine benchmark-spezifischen Lösungshinweise.

Die Änderung ist auf CLI-Normalisierung, deren Tests, Hilfe/Schema und die adaptive Anleitung begrenzt. Agent-API, Strict-Anleitung, Berechtigungen, Redaktion und Reader-Grenzen bleiben erhalten; keine neue Abhängigkeit.

- Vollständiges `go test ./...`: bestanden, 146,97 Sekunden; `go vet ./...`: bestanden.
- Unabhängiger Code-Review: Spezifikation und Qualität bestanden, keine offenen Codebefunde.
- Beide historischen Fehlanfragen funktionieren nun und liefern exakt dieselben JSON-Ergebnisse einschließlich Belegen wie die bisherigen kanonischen Anfragen.
- 23 Kontextabfragen und 180 Kontrollen bestanden; alle 23 Antworten sind gegenüber dem Vorgänger unverändert.
- Wiederverwendung aller 115 adaptiven Quellabschnitte sowie 55 Find-Aufrufe an sechs echten Dateien bestanden: 118 Treffer und 361 eindeutige Ausgabezeilen.

## Nutzung und verbleibender Bedienaufwand

| Messgröße | Lauf 1 | Lauf 2 |
|---|---:|---:|
| Leseraufrufe erfolgreich / versucht | 17/20 | 22/24 |
| Zurückgegebene Leserzeilen | 1.673 | 1.568 |
| Übersprungene Leserzeilen | 258 | 113 |
| Ignorierte Belege | 1 | 0 |
| Verifizierte Quellzeilen insgesamt | 1.808 | 1.794 |
| Befehle mit verifizierten Quellzeilen | 17/29 | 25/36 |
| Wiederholt ausgegebene verifizierte Quellzeilen | 0 | 64 |
| Befehlsausgabe in Bytes | 146.613 | 164.787 |

Der erste Lauf enthält drei korrigierte Leserfehler: 24-KiB-Ausgabegrenze, ungültiger regulärer Ausdruck und dieselbe Datei doppelt in einer Find-Anfrage. Ein Beleg für eine andere Datei wurde korrekt ignoriert und der angeforderte Quellbereich vollständig geliefert. Der zweite Lauf enthält zwei korrigierte Anfragen mit doppeltem Find-Pfad sowie eine Textsuche ohne Treffer. Kein `start_line`-Schemafehler trat erneut auf. Alle Fehlversuche sind in Zeit und Tokens enthalten.

Die 64 nachgewiesenen Wiederholungen im zweiten Lauf entstehen aus Kontext → Leser (18), Leser → Textsuche (7), Kontext → Textsuche (4) und Textsuche → Leser (35). Die Anleitung allein verhindert solche Arbeitsweisen nicht zuverlässig. Die Zählung ist eine Untergrenze aus exakt verifizierten Ausgabezeilen bei der oben ausgewiesenen Parserabdeckung, keine vollständige Erfassung interner Dateizugriffe.

## Installation und Integrität

Lokal installiert ist **1.4.1 als Entwicklungsstand**, Commit `d67d1f4ab3c3-dirty`, gebaut `2026-09-11T11:05:25Z`, SHA-256:

`a2938d66112ed00f66dd365ee97e2548c856100191d194f8df7c03b5a18bdc34`

Go-bin, Homebrew-Datei und öffentlicher Symlink stimmen mit dem getesteten Kandidaten überein. Beide Vorgängerdateien sind gesichert. Beide CLI-Läufe verwenden dieselbe Binärdatei, Anleitung, Basisaufgabe, CLI-Argumente, Messskripte und `gpt-5.6-sol` mit Reasoning `high`. Die natürlich formulierte erste Suchfrage darf variieren.

Alle 374 eingefrorenen Quelldateien, Indizes, der vollständige Build-Quellbestand und die 15 gespeicherten Baseline-Dateien sind nach beiden Läufen unverändert. Kein neuer Scan war nötig. Kein erneuter Lauf ohne GoreGraph, keine Service-Tests oder Service-Änderungen, kein Commit, Push oder Release.

## Entscheidungen und verbleibende Arbeit

1. Das bestehende schmutzige Checkout wurde mit vollständiger Vorher-Kopie weiterverwendet. Vorarbeiten bleiben erhalten; eine Rücknahme müsste auf das gesicherte eigene Delta begrenzt werden.
2. Nur eindeutige CLI-Kurzformen werden in die unveränderte API übersetzt. Das behebt den reproduzierten Bedienfehler, erweitert aber die zu prüfende Syntax; Konflikte bleiben ausdrückliche Fehler.
3. Die Belegregeln sind allgemein formuliert. Eine bekannte HTTP-Begriffsverwechslung wurde nicht als erwartete Benchmark-Antwort eingespeist. Modellfehler müssen weiterhin unabhängig bewertet werden.

Die nächsten belegten Ansatzpunkte sind eine zuverlässigere Ausgabe vollständiger Pfade, weniger manuell fehleranfällige Find-Anfragen und eine Konsistenzprüfung der vorgeschlagenen Regressionstests. Weitere Anleitung allein ist nach diesen gemischten Ergebnissen kein belegter Garant für höhere Gesamtqualität. Der breitere [Entwicklungs-Akzeptanztest](AGENT-DEVELOPMENT-ACCEPTANCE.md) bleibt unabhängig offen.

Private Antworten, Logs, Qualitätsberichte, unveränderte Prompts, Hashes, Skripte und Binärsicherungen liegen im vorhandenen Benchmark-Verzeichnis unter `reader-precision/`, `reader-precision-final/` und `reader-precision-1-cli/` beziehungsweise `reader-precision-2-cli/`. `reader-precision/comparison.json` enthält die maschinenlesbare Gegenüberstellung. Rohprüfungen bleiben neben ausdrücklich getrennten manuellen beziehungsweise ergänzenden Bewertungen erhalten.
