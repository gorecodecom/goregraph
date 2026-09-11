# Kombinierte Quellsuche und Quellausgabe

Historischer Messstand. Der neuere [CLI-/Beleg-Nachlauf](AGENT-READER-PRECISION-2026-09-11.md) dokumentiert die aktuelle Installation und ein gemischtes Folgeergebnis; die folgenden Messwerte bleiben unverändert.

Stand: 11.09.2026. Die freigegebene Erweiterung, lokale Installation und zwei unveränderte Codex-CLI-Läufe sind abgeschlossen. **Beide Antworten erreichen 12/12 feste Kernkriterien und 7/7 bekannte Testreferenzen. Beide Läufe sind schneller als die gespeicherte Referenz; der Vorsprung im zweiten Lauf ist klein.** Einzelne Antwortungenauigkeiten bleiben ausdrücklich bestehen.

## Gemessener Vergleich

| Messgröße | Gespeichert ohne GoreGraph | Kombinierter Leser 1 | Kombinierter Leser 2 |
|---|---:|---:|---:|
| Laufzeit | 600,30 s | 544,16 s | 590,03 s |
| Erster belegter Einstieg | 44,25 s | 20,34 s | 18,19 s |
| Letzte Werkzeugantwort | 370,80 s | 400,53 s | 456,90 s |
| Zeit danach bis Abschluss | 229,50 s | 143,63 s | 133,13 s |
| Eingabetokens | 2.258.257 | 1.290.468 | 1.291.081 |
| Davon gecachte Eingabe | 2.108.928 | 1.189.760 | 1.222.400 |
| Ausgabetokens | 16.510 | 24.575 | 27.103 |
| Gesamttokens inklusive Cache | 2.274.767 | 1.315.043 | 1.318.184 |
| Effektive Tokens | 165.839 | 125.283 | 95.784 |
| Befehle | 23 | 24 | 32 |
| Fachliche Kernpunkte | 12/12 historisch | 12/12 | 12/12 |
| Bekannte Testreferenzen | hier nicht neu bewertet | 7/7 | 7/7 |
| Ausgegebene Befehlsbytes | hier nicht neu bewertet | 173.911 | 152.255 |

Gegenüber der gespeicherten Referenz beträgt die Zeitersparnis **9,35 % / 1,71 %**, die Ersparnis effektiver Tokens **24,46 % / 42,24 %**. Der Mittelwert der zwei Läufe, zugleich ihr Median, ist **567,10 Sekunden / 110.533,5 effektive Tokens**: 5,53 % weniger Zeit und 33,35 % weniger effektive Tokens als die Referenz.

Gegenüber dem vorherigen vollständigen GoreGraph-Paar (769,79/605,25 Sekunden und 123.863/109.660 effektive Tokens) sinken die Mittelwerte um **17,52 % Zeit und 5,33 % effektive Tokens**. Der erste neue Lauf benötigt einzeln etwas mehr effektive Tokens als beide vorherigen Läufe; eine durchgängige Tokenverbesserung jedes einzelnen Laufs liegt somit nicht vor. Die vorherigen Ergebnisse bleiben im [Inventar-/Wiederverwendungsbericht](AGENT-INVENTORY-REUSE-2026-09-11.md) erhalten.

Effektive Tokens bedeuten Eingabe minus gecachte Eingabe plus Ausgabe. Reasoning-Tokens sind bereits in der Ausgabe enthalten. Es werden weder kostenlose Cache-Tokens noch eine entsprechende Geldersparnis behauptet. Das sind zwei Läufe gegen eine historische Referenz, kein statistisch belastbarer allgemeiner Geschwindigkeitsnachweis.

## Umsetzung

`goregraph read` akzeptiert pro exakt benannter Datei jetzt wahlweise bekannte `ranges` oder einen `find`-Selektor. Die Suche und die passenden nummerierten Quellausschnitte kommen gemeinsam zurück. Beispiel:

```sh
goregraph read . --request '{"files":[{"path":"src/Handler.java","find":{"pattern":"handle|validate","before":2,"after":5,"max_matches":4}}]}'
```

- Go-Regexp-Muster werden je Zeile auf bereits geschwärztem Inhalt geprüft. Matchanzahl und Positionen verraten keine verborgenen Konfigurationswerte.
- Überlappende Kontextfenster werden zusammengeführt. Vorhandene `seen`-Belege werden wie bei bekannten Bereichen abgezogen; zurück kommt der kumulative Beleg.
- `match_lines` und `match_count` sind Suchmetadaten. Nur `sections` enthält neu gelieferten Code. Weitere relevante Treffer lassen sich über `find.start_line` mit dem zurückgegebenen `next_start_line` abrufen.
- Die bisherigen Datei-, Bereichs-, Zeilen- und Byte-Grenzen bleiben erhalten. Zu große Anfragen scheitern ausdrücklich und atomar, ohne teilweise gelieferte Quellen oder erfundene Lesebelege.
- Die adaptive Anleitung verwendet nach der Dateisuche bevorzugt kombinierte Anfragen. Exakt begrenzte Prüfberechtigungen verwenden weiterhin `ranges`. Strict-v1, Leseberechtigungen, Sperren, Pfadprüfung und Konfigurationsredaktion bleiben erhalten.

Die Änderung umfasst den bestehenden Leser, einen kleinen Find-Helfer, CLI-Hilfe/Schema, die adaptive Anleitung und Verhaltenstests. Keine neue Abhängigkeit und keine benchmark-spezifischen Namen oder Lösungshinweise in Produktlogik/Anleitung.

## Tatsächliche Nutzung und Restaufwand

| Messgröße | Lauf 1 | Lauf 2 |
|---|---:|---:|
| Leseraufrufe erfolgreich / versucht | 16/18 | 16/19 |
| Befehlsantworten mit Find-Ergebnissen | 14 | 15 |
| Zurückgegebene Quellzeilen | 2.123 | 1.770 |
| Technisch übersprungene Zeilen | 165 | 193 |
| Ignorierte Belege | 0 | 0 |
| Verifizierte Quellzeilen insgesamt | 2.257 | 1.896 |
| Befehle mit verifizierten Quellzeilen | 19/24 | 17/32 |
| Erneut gelieferte verifizierte Quellzeilen | 0 | 0 |

Im vorherigen Paar wurden 305/194 wiederholte Quellzeilen nachgewiesen. Jetzt sind es **0/0 in den verifizierbaren Ausgaben**. Die Prüfung vergleicht Ausgabezeilen exakt mit den unveränderten Quellen; die Abdeckung oben bleibt sichtbar. Das ist keine Aussage, dass sämtliche möglichen Ausgabeformate oder internen Dateilesezugriffe vollständig erfasst wären. Der Leser liest Dateien weiterhin zur Aktualitätsprüfung. Die gesamte Tokenersparnis ist nicht allein den übersprungenen Zeilen zuzuschreiben.

Lauf 1 enthält zwei korrigierte Leserfehler: eine Überschreitung von 24 KiB Ausgabe und ein falsch platziertes `start_line`-Feld. Lauf 2 enthält zusätzlich eine Überschreitung der kombinierten Bereichs-/Zeilengrenze, insgesamt drei korrigierte Leserfehler; zwei weitere erfolglose Befehle sind reine Dateisuchen ohne Treffer. Kein fehlgeschlagener Aufruf zählt als gelieferte Quelle. Diese Versuche und ihre Kosten sind in allen Messwerten enthalten.

Die festen Ausgabegrenzen und das manuelle Zusammensetzen von JSON bleiben Bedienungsaufwand. Der neue Leser verhindert doppelte Quellausgabe bei mitgeführten Belegen, aber keine unnötigen Suchentscheidungen des Modells. 143,63/133,13 Sekunden fallen nach der letzten Werkzeugantwort für weitere Verarbeitung und Antwortausgabe an.

## Qualität und Grenzen der Antworten

Die unabhängige, unverblindete statische Prüfung bestätigt beide Male 12/12 Kernkriterien und 7/7 Datei-/Rollenreferenzen. Das bedeutet keine sieben vollständig geprüften Testsuiten und keinen Testlauf der Services.

- Lauf 1: alle 42 Inventarpfade und 69 numerischen Tabellenbereiche gültig. Mail-Referenzen sind nur oberflächlich belegt. Zwei größere Query-Test-Zitatbereiche umfassen auch nicht gelieferte Zwischenzeilen; die gelesenen Teile tragen die allgemeine Referenzrolle, nicht die Behauptung vollständiger Prüfung dieser Bereiche.
- Lauf 2: alle 38 Inventarpfade gültig, 52/53 Bereiche innerhalb physischer Inhaltszeilen und 53/53 nach der bestehenden API-Zählung. Der gemeinsame Konfigurationstyp endet physisch bei Zeile 40; der Leser zählt die leere Schlusszeile 41 mit. Keine nichtleere Codezeile wurde dadurch erfunden. Zwei Mail-Tests sind korrekt als Metadatenreferenzen gekennzeichnet; auch die breiten Einzel-Löschtest-Zitate enthalten nicht gelesene Zwischenzeilen.
- Lauf 2 bezeichnet einen wiederholten öffentlichen DELETE mit 404 zu pauschal als nicht idempotent. HTTP-Idempotenz verlangt dieselbe beabsichtigte Wirkung, nicht denselben Statuscode. Das getrennte Problem einer fehlenden Wiederaufnahme nach teilweise erfolgreicher Löschung bleibt relevant; die Begriffsverwechslung ist trotzdem ein Antwortfehler.
- Oracle-Inhalt, Datenbankschema/Kaskaden, Deploymentwerte und verteilte Transaktionsgarantien bleiben unbewiesen. Vorgeschlagene Löschreihenfolgen sind Designentscheidungen mit Fehlerfällen, keine bereits verifizierten globalen Transaktionslösungen.

Die Verbesserung adressiert den gemessenen Such-/Leseaufwand. Sie macht die Antworten weder generell fehlerfrei noch löst sie den breiteren [Entwicklungs-Akzeptanztest](AGENT-DEVELOPMENT-ACCEPTANCE.md).

## Installation und Verifikation

Lokal installiert: **1.4.1 als Entwicklungsstand**, Commit `d67d1f4ab3c3-dirty`, gebaut `2026-09-11T10:17:13Z`, SHA-256:

`a67cac6a955fb9944acae0948e7c86b7d6c064ea929930876ec7d57ed3d0f92e`

Die Go-bin- und Homebrew-Installationen sowie der öffentliche Homebrew-Symlink stimmen mit dem gemessenen Kandidaten überein; beide Vorgängerdateien sind gesichert.

- Vollständiges `go test ./...`: bestanden, 138,55 Sekunden. `go vet ./...`: bestanden.
- Unabhängiger Code-Review: Spezifikation und Qualität bestanden, keine offenen Codebefunde.
- 23 lokale Kontextabfragen mit 180 Kontrollen bestanden; alle 23 Antworten sind gegenüber dem vorherigen Kandidaten unverändert.
- Alle 115 adaptiven Quellabschnitte lassen sich mit ihren Belegen ohne erneute Ausgabe wiederverwenden.
- 55 zusätzliche Find-Aufrufe an sechs echten Dateien prüfen 118 erwartete Treffer und 361 eindeutige Ausgabezeilen: vollständige Folgeseiten, korrekte Ausschnitte, keine Wiederholung, Wiederverwendung zwischen Find und Ranges.
- Beide Benchmarkläufe verwenden dieselbe Binärdatei, Anleitung, Basisaufgabe, Modell (`gpt-5.6-sol`, high), Argumente und Messskripte. Die natürlich formulierte erste Suchfrage darf variieren.
- Nach beiden Läufen sind alle 374 eingefrorenen Dateien, Indizes, der vollständige Build-Quellbestand und alle 15 Baseline-Dateien unverändert. **Kein neuer Scan, kein erneuter Lauf ohne GoreGraph und keine Service-Tests oder -Änderungen.** Kein Commit, Push, Merge oder Release.

## Entscheidungen während der Umsetzung

1. Das vorhandene schmutzige Feature-Checkout wurde mit vollständiger Vorher-Kopie weiterverwendet. Das erhält sämtliche Vorarbeiten; ein Abgrenzungsfehler müsste aus dem gesicherten Delta zurückgenommen werden.
2. Die Erweiterung verwendet begrenzte Suche pro Datei auf geschwärzten Zeilen mit ausdrücklich sichtbaren Folgeseiten. Sie vermeidet getrennte Codeausgaben aus Suche und Lesen; ungeeignete Muster oder Paketgrößen können trotzdem zusätzliche Aufrufe verursachen.

Private Rohprotokolle, Antworten, Qualitätsberichte, unveränderte Prompts, Prüfskripte, Hashes und Binärsicherungen liegen unter `combined-read/`, `combined-read-final/` und `combined-read-1-cli/` beziehungsweise `combined-read-2-cli/` im vorhandenen lokalen Benchmark-Verzeichnis. `combined-read/comparison.json` ist die maschinenlesbare Gegenüberstellung. Keine dieser Referenzlösungen wird in den Benchmark-Auftrag eingespeist.
