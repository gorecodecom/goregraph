# Testinventar und Wiederverwendung gelesener Quellen

Historischer Messstand. Die aktuelle Installation und der abgeschlossene nächste
Schritt stehen im [Bericht zum kombinierten Leser](AGENT-COMBINED-READ-2026-09-11.md).

Stand: 11.09.2026. Umsetzung, lokale Installation und zwei unveränderte Abschlussläufe sind abgeschlossen. **Beide Antworten erreichen 12/12 fachliche Kernpunkte und 7/7 bekannte Testreferenzen. Eine stabile Beschleunigung gegenüber der gespeicherten Referenz ist nicht erreicht.**

## Ergebnis des finalen Kandidaten

| Messgröße | Gespeichert ohne GoreGraph | Finaler Lauf 1 | Finaler Lauf 2 |
|---|---:|---:|---:|
| Laufzeit | 600,30 s | 769,79 s | 605,25 s |
| Erster belegter Einstieg | 44,25 s | 25,07 s | 20,86 s |
| Letzte Werkzeugantwort | 370,80 s | 499,22 s | 437,96 s |
| Zeit danach bis Abschluss | 229,50 s | 270,57 s | 167,29 s |
| Eingabetokens | 2.258.257 | 1.957.592 | 1.396.879 |
| Davon gecachte Eingabe | 2.108.928 | 1.858.816 | 1.311.104 |
| Ausgabetokens | 16.510 | 25.087 | 23.885 |
| Gesamttokens inklusive Cache | 2.274.767 | 1.982.679 | 1.420.764 |
| Effektive Tokens | 165.839 | 123.863 | 109.660 |
| Befehle | 23 | 31 | 36 |
| Fachliche Kernpunkte | 12/12 | 12/12 | 12/12 |
| Bekannte Testreferenzen | hier nicht erneut bewertet | 7/7 | 7/7 |
| Vorhandene Dateien im Inventar | hier nicht erneut bewertet | 41 | 38 |
| Ausgegebene Befehlsbytes | hier nicht erneut bewertet | 250.057 | 240.093 |

Die effektiven Tokens liegen **25,31 % / 33,88 % niedriger**, die Laufzeiten **28,23 % / 0,82 % höher** als die Referenz. Effektive Tokens sind Eingabe minus Cache plus Ausgabe; Reasoning ist bereits in der Ausgabe enthalten. Das ist keine Kostenrechnung und kein Nachweis, dass Cache-Tokens kostenlos wären.

Die Referenz stammt aus einem früheren Lauf und wurde auf Nutzerwunsch nicht wiederholt. Beide neuen Läufe verwenden dieselbe Binärdatei, dieselbe Anleitung, identische Basisaufgabe, Modell (`gpt-5.6-sol`, high) und Argumente sowie unveränderte Services und Indizes. Die vom Modell formulierte erste Suchfrage kann variieren. Zwei Läufe liefern keine statistisch abgesicherte oder allgemeine Leistungsaussage. Die Qualitätsprüfung ist eine unabhängige, unverblindete statische Prüfung, kein Lauf der Services oder ihrer Tests.

## Was geändert wurde

- Das adaptive Zusatzinventar berücksichtigt unterschiedliche relevante Testidentitäten einer tatsächlich belegten Nebenwirkungsfamilie. Doppelte Identitäten verbrauchen keinen Platz mehr; die Grenze von vier Zusatztests bleibt bestehen. Private Referenznamen stehen weder in Produktlogik noch im Benchmark-Auftrag.
- Tatsächlich gelesene, aktuelle und exakte Endpunkt- oder Modell-Typdeklarationen können das zugehörige Projekt für Navigation qualifizieren. Bei Typen wird die echte Deklaration an der indizierten Zeile geprüft. Methoden, Felder, Kommentare, nicht aktuelle Quellen und nicht angefragte Projekte zählen nicht. Dadurch hängt die Navigation weniger von der Sprache der Anfrage ab. Dies ist keine Aussage über einen bestehenden Service-Aufruf oder Laufzeitbesitz.
- Explizite Konfigurationsinventare erhalten bei fehlenden passenden Schlüsselgruppen niedrig priorisierte exakte Ressourcenidentitäten. Sie enthalten Pfad und Profil, keine behaupteten Schlüssel oder Werte.
- Die finale Budgetprüfung entfernt nur vollständig doppelte Dateimetadaten: identisches Projekt, Pfad, Zeilenintervall und Rolle zu einem aktuellen Quellabschnitt, ohne eigene Begründung oder Konfidenz. Unterschiede bleiben erhalten. Bei der neutralen Einstiegsfrage sparen zwei Einträge 87 Tokens; vier Konfigurationsdateien und vier Zusatztests passen bei 3.984/4.000 Tokens. Die sechs Quellabschnitte samt Lesebelegen und die Prüf-/Aussagemetadaten bleiben identisch.
- Der neue zustandslose CLI-Leser `goregraph read` liefert anhand mitgeführter Bereichsbelege nur noch nicht gelieferte Intervalle. Belege binden kanonischen Pfad und aktuellen Inhalt. Geänderte Inhalte werden nicht irrtümlich unterdrückt. Umfangs-, Pfad-, Symlink- und Ausgabebegrenzungen sowie Konfigurationsredaktion sind geprüft. Fehlende Sperrdateien führen zu einem ausdrücklichen Fehler, nicht zu heimlichen Schreibzugriffen.
- Die adaptive Anleitung verwendet diesen Leser für erlaubte Quellausschnitte, führt die Belege weiter und verlangt ergänzende Testsuche für jede betroffene Datenvariante, einschließlich `Test` und `Tests`. Das Zusatzinventar ist ausdrücklich nicht vollständig. Die Strict-Anleitung bleibt bytegleich.

## Tatsächliche Wiederverwendung und verbleibender Aufwand

| Messgröße | Finaler Lauf 1 | Finaler Lauf 2 |
|---|---:|---:|
| Leseraufrufe, erfolgreich / versucht | 12/12 | 9/10 |
| Zurückgegebene Zeilen nach API-Zählung | 1.150 | 1.531 |
| Technisch übersprungene Zeilen | 23 | 6 |
| Ignorierte Belege wegen Pfad/Inhaltswechsel | 0 | 0 |
| Erneut gelieferte Quellzeilen, Untergrenze | 305 | 194 |
| Davon Suche → Leser | 229 | 154 |
| Davon Leser → Leser | 0 | 1 |

Die einzelne Wiederholung zwischen Leseraufrufen in Lauf 2 entstand, weil der spätere Aufruf den vorhandenen Beleg nicht mitgab. Ein weiterer Leseversuch überschritt die 24-KiB-Ausgabegrenze und wurde ohne Quellausgabe abgelehnt; danach wurde der Umfang reduziert. Die übrigen erfolglosen Befehle waren Suchläufe ohne Treffer.

Die Wiederholungsprüfung zählt nur Zeilen, die exakt mit der eingefrorenen Quelle übereinstimmen. Sie erfasst in den beiden Läufen 23/31 beziehungsweise 22/36 Befehle mit solchen Zeilen; Dateisuchen, gekürzte Trefferausgaben und nicht erfolgreich gelieferte Antworten begrenzen die Abdeckung. Dies sind Untergrenzen, keine vollständigen I/O-Zähler. Der Leser muss Dateien zur Aktualitätsprüfung intern weiterhin lesen. Die gemessene gesamte Tokenersparnis lässt sich nicht allein den übersprungenen Zeilen zuschreiben.

**Der wesentliche verbleibende Engpass ist die getrennte Suche und anschließende Quellprüfung.** Suchausgaben enthalten bereits Code, der danach in einem größeren Leserintervall erneut erscheint. Dazu kommen viele einzelne Such-/Leseentscheidungen. Der Leser kann fremde Shell-Ausgaben nicht abfangen und benötigt die mitgeführten Belege. Ein sinnvoller nächster begrenzter Verbesserungsschritt wäre, Fundstellensuche und die zugehörigen Quellausschnitte in einem kontrollierten Leseaufruf zusammenzufassen. Diese Erweiterung ist in den oben gemessenen Kandidaten nicht enthalten.

## Genauigkeit der Quellenangaben

Alle 41 beziehungsweise 38 benannten Inventardateien existieren; alle sieben Testrollen wurden geprüft. Lauf 1 kennzeichnet jedoch zwei Mail-Tests als ungelesen, obwohl die Suche bereits Ausschnitte daraus geliefert hatte. Diese konservative, aber falsche Kennzeichnung ist ein Fehler in der Antwortprovenienz; die Rollen selbst stimmen. Lauf 2 verweist auf tatsächliche Ausschnitte.

Zwei beziehungsweise drei Tabellenbereiche schließen die leere logische Zeile am Dateiende ein. Der bestehende GoreGraph-Leser und die lokalen Prüfungen zählen mit `split("\n")`; der ursprüngliche Tabellenprüfer mit `splitlines()`. Deshalb bestehen **54/56 und 42/45 physische Inhaltsgrenzen**, aber **56/56 und 45/45 API-Zeilengrenzen**. Es wurde keine nichtleere Codezeile erfunden; die Aussagen werden durch die vorhandenen Zeilen gestützt. Beide Zählungen bleiben in den Auswertungen sichtbar.

Oracle-Inhalt, externe Deployment-Werte, Datenbankschema/Kaskaden und verteilte Transaktionsgarantien sind weiterhin ausdrücklich unbewiesen. Die zwölf Kernpunkte und sieben Referenzen bedeuten keine allgemeine Vollständigkeits- oder Release-Freigabe.

## Installation und Prüfung

Lokal installiert ist **1.4.1 als Entwicklungsstand**, gebaut **11.09.2026, 08:36:28 UTC**, Commit `d67d1f4ab3c3-dirty`, SHA-256:

`14d3c9701f6768f0e226a7dc35469a58c1ef39485129ee8f168d3be40d397cfd`

Beide Installationsorte und der öffentliche Homebrew-Symlink zeigen auf diesen Kandidaten; vorherige Binärdateien sind gesichert. Kein Release, Commit, Push oder Service-Quelltext wurde erzeugt beziehungsweise verändert.

- Vollständige Go-Suite: bestanden, 130,55 Sekunden; `go vet ./...`: bestanden.
- 23 lokale Abfragen: 180/180 Prüfungen bestanden, einschließlich Scope, Strict, Budget und Gleichheit der Quell-/Prüfmetadaten bei der Komprimierung.
- Wiederverwendung aller 115 gelieferten adaptiven Quellabschnitte: Belege akzeptiert, null erneut gelieferte Abschnitte.
- Getrennte Implementierungs- und Integrationsreviews: keine offenen Codebefunde.
- Nach beiden CLI-Läufen: Kandidat, installierte Dateien, vollständiger Build-Quellbestand, 374 eingefrorene Dateien, Indizes und 15 Baseline-Dateien unverändert. Kein neuer Scan nötig.

## Aufbewahrte Fehlversuche und Entscheidungen

Der erste Vergleich dieses Auftrags benötigte 635,57/477,26 Sekunden und 102.136/86.441 effektive Tokens. Er erreichte 11/12 beziehungsweise 12/12 Kernpunkte, aber jeweils nur 5/7 Referenzen. Die beiden Läufe bleiben erhalten. Zwei rein lokale Kandidaten bestanden zunächst Navigations- beziehungsweise Budgetkontrollen nicht und wurden vor der Installation korrigiert; keine fehlgeschlagene Kontrolle wurde stillschweigend gestrichen.

Die während der Arbeit getroffenen Entscheidungen und ihre Grenzen:

1. Das vorhandene schmutzige Feature-Checkout wurde mit separaten Vorher-Kopien erhalten, ohne Umzug oder Commit. Ein Abgrenzungsfehler müsste anhand dieser Kopien rückgängig gemacht werden.
2. Bereichsbelege werden vom Aufrufer weitergegeben; es gibt keinen persistenten Lesestatus. Das erhält die reine Leseausführung, verursacht aber Eingabeaufwand und lässt bei fehlendem Beleg Wiederholungen zu.
3. Der neue Leser verlangt bestehende Lesesperren. Alte, unvollständig initialisierte Ausgaben können dadurch ausdrücklich scheitern; der normale Kontext-Leseweg bleibt unverändert.
4. Aktuelle exakte Endpunkte und echte Modell-Deklarationen qualifizieren nur Navigation. Eine falsche Zuordnung könnte begrenzt irrelevante Hinweise liefern, aber keine Quelle oder bestehende Laufzeitverbindung beweisen.
5. Die belegten Lücken nach dem ersten Paar wurden durch eng begrenzte Auswahl- und Anleitungsänderungen weiterbearbeitet. Mehr Navigation und Regeln können zusätzlichen Aufwand verursachen; die erneuten Messungen zeigen diese Grenze ausdrücklich.
6. Nur exakt doppelte Dateimetadaten werden im begrenzten Budgetfall entfernt. Ein Gleichheitsfehler könnte zusätzliche Metadaten verbergen; unterschiedliche Rollen, Bereiche, Begründungen oder Konfidenzen bleiben deshalb erhalten und sind negativ getestet.

Private Rohprotokolle, Prompts, Hashes, Kandidaten, Messskripte und Qualitätsberichte liegen im lokalen Benchmark-Verzeichnis unter `inventory-reuse-*`; sie werden nicht in den Benchmark-Auftrag eingespeist. Die maschinenlesbare finale Auswertung ist `inventory-reuse-followup/comparison.json`.
