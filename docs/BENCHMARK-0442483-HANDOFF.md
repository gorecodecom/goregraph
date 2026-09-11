# Benchmark 0442483: Übergabe für die Fortsetzung

Stand: 11.09.2026. Diese Datei ist für Entwickler und Prüfer bestimmt und darf
nicht als Kontext an einen Benchmark-Agenten gegeben werden: Sie enthält die
Referenzlösung und Ergebnisse. Die beiden vollständigen Agent-Prompts stehen
weiter unten separat und ohne Lösungshinweise.

## Fortsetzungsregel vom 10.09.2026

Auf ausdrücklichen Nutzerwunsch wird der Vergleich **ohne GoreGraph nicht bei
jeder Fixrunde erneut ausgeführt**. Verwende den gespeicherten vollständigen
Mac-Referenzlauf `local-update-baseline-cli`: 600,30 Sekunden, 165.839 ungecachte
Eingabe- plus Ausgabetokens, 12/12 Abdeckungskriterien. Definitionen und Grenzen
stehen im [Installationsvergleich](AGENT-INSTALLED-COMPARISON-2026-09-10.md).
Ein neuer Baseline-Lauf benötigt einen neuen ausdrücklichen Nutzerauftrag.

Der später gestartete Lauf `latency-baseline-cli` wurde auf Nutzerwunsch beendet
und darf nicht als abgeschlossener Vergleich gewertet werden. Weitere Fixrunden
messen den GoreGraph-Workflow gegen die gespeicherte Referenz; kennzeichne dabei,
dass die Referenz aus einem früheren Lauf stammt.

## Aktueller Nachlauf: Antwortprüfung und Mehrfachsuche

Der neueste [Auto-Paging-Nachlauf](AGENT-READER-AUTOPAGE-2026-09-11.md) ist
abgeschlossen. Nach zwei diagnostischen Runden erreicht der dritte unveränderte
CLI-Lauf **12/12 Kernkriterien, 7/7 erforderliche Testidentitäten und null
fehlgeschlagene GoreGraph-Befehle**. Er benötigt 505,816 Sekunden und 79.464
effektive Tokens. Gegenüber der unverändert wiederverwendeten Referenz ist er
15,74 % schneller und spart 52,08 % effektive Tokens. In den verifizierten
Leserausgaben gibt es keine wiederholt gelieferten Quellzeilen.

Der abschließende Answer-Checker-Fix löst gleichnamige Kurzpfade anhand eindeutig
passender gelieferter Zeilenbereiche auf. Die gespeicherte Antwort aus Lauf 3 ist
damit mechanisch gültig: 50 Referenzen, 52 Bereiche, 12 sichere Reparaturen und
keine Findings. Installiert ist der geprüfte lokale 1.4.1-Entwicklungsstand vom
11.09.2026, 15:09:29 UTC, SHA `0723dfdd2bb6…`. Vollständige Go-Suite, vet,
Integritätsprüfungen, Quellen, Indizes und Baseline bestehen. Kein Rescan, kein
erneuter Lauf ohne GoreGraph und kein Release.

## Vorheriger Nachlauf: Antwortprüfung und Mehrfachsuche

Der [Nachlauf zur Antwortprüfung](AGENT-ANSWER-GUARD-2026-09-11.md) ist lokal
installiert: 1.4.1 Entwicklung, 12:57:46 UTC, SHA `fc815c8e3870…`.
Mehrere Suchselektoren pro Datei werden unabhängig ausgeführt und gemeinsam
ausgegeben. `answer-check` prüft Dateipfade und gelieferte Zitatbereiche und kann
eindeutige Kurzpfade ergänzen. Fachliche Richtigkeit bleibt separat zu prüfen.
Lokale Tests und unabhängiger Code-Review bestehen. Der erste diagnostische Lauf
(716,87 Sekunden/101.877 effektive Tokens) bleibt mit seinen Fehlern erhalten.
Nach gezielter Nachbesserung besteht der abschließende Lauf die mechanische Prüfung
ohne Änderungen und den unabhängigen fachlichen Review: **12/12 Kernkriterien,
7/7 exakte Testreferenzen, kein wesentlicher semantischer Fehler gefunden**.

Abschluss: **627,60 Sekunden und 93.047 effektive Tokens**. Gegenüber der gespeicherten
Referenz: 43,89 % weniger effektive Tokens, aber weiterhin 4,55 % mehr Zeit.
23 statt 44 Befehle; ein statt fünf zurückgewiesene Leseranfragen. 22 erneut
gelieferte Quellzeilen bleiben nachweisbar. Die mechanische Prüfung benötigt
0,120 Sekunden; Rohantwort und finale Antwort sind bytegleich.

Alle 374 Quellen, Indizes und 15 Baseline-Dateien sind unverändert. Kein Rescan
oder Service-Test. Die beiden Stufen haben bewusst getrennte Kandidaten/Anleitungen
und dürfen nicht als unverändertes Kandidatenpaar dargestellt werden. Die breitere
Entwicklungs-Akzeptanz und ein allgemeiner Geschwindigkeitsnachweis bleiben offen.

## Vorheriger Nachlauf: CLI-Leserfehler und präzisere Quellenbelege

Der [Nachlauf vom 11.09.2026](AGENT-READER-PRECISION-2026-09-11.md) ist umgesetzt,
lokal installiert und zweimal mit demselben Kandidaten geprüft. Entwicklungsstand
1.4.1, 11:05:25 UTC, SHA a2938d66112e…; keine neue Release-Version.

Die beiden historischen start_line-Fehlanfragen funktionieren nun exakt wie ihre
kanonischen Varianten. Die adaptive Anleitung fordert präzise gelieferte Zitate
und tatsächliche Testaktionen/Assertions. Beide Antworten vermeiden nach
Zuordnung ungelesene Java-Zitatlücken, erfüllen den Ausgabeauftrag aber nicht
gleich zuverlässig: **12/12 und 11/12 Kernkriterien**. Beide haben sieben
erkennbare Testidentitäten/Rollen, jedoch nur 6/7 beziehungsweise 1/7 vollständig
ausgeschriebene Testpfade. Lauf 2 verletzt wegen 32 verkürzter Inventarpfade das
feste Pfadkriterium. Lauf 1 enthält ungenaue Regressionstestformulierungen.

Zeit/effektive Tokens: **582,26 s / 103.649** und **680,44 s / 110.728**.
Das sind 37,50 % / 33,23 % weniger effektive Tokens als die gespeicherte Referenz,
aber nur Lauf 1 ist schneller. Mittelwert: 631,35 s und 107.188,5 effektive Tokens.
Verifizierte wiederholt ausgegebene Quellzeilen: 0/64 bei 17/29 beziehungsweise
25/36 Befehlen mit verifizierten Quellzeilen. Suchsyntax und doppelte Find-Pfade
verursachen weiterhin korrigierte Fehler. Kein allgemeiner Qualitäts- oder
Geschwindigkeitsnachweis.

Go-Suite/vet, unabhängiger Code-Review, historische Fehlerreplays, 23 unveränderte
Kontextantworten mit 180 Kontrollen, 115 Quellbelege und 55 Find-Prüfaufrufe bestehen.
Quellen, Indizes, Build und alle 15 Baseline-Dateien sind unverändert. Kein Scan,
kein neuer Lauf ohne GoreGraph, keine Service-Tests oder Service-Änderungen.
Die vollständigen Rohdaten und getrennten Parserbewertungen liegen unter
reader-precision* im vorhandenen privaten Benchmark-Verzeichnis.

## Vorheriger Nachlauf: kombinierte Quellsuche und Quellausgabe

Der [kombinierte Leser vom 11.09.2026](AGENT-COMBINED-READ-2026-09-11.md) ist
implementiert, lokal installiert und mit zwei identischen Kandidatenläufen geprüft.
Entwicklungsstand 1.4.1, 10:17:13 UTC, SHA `a67cac6a955f…`.

Beide Antworten erreichen **12/12 Kernkriterien und 7/7 bekannte Testreferenzen**.
Zeit/effektive Tokens: **544,16 s / 125.283** und **590,03 s / 95.784**.
Gegenüber der gespeicherten Referenz sind das 9,35 % / 1,71 % weniger Zeit und
24,46 % / 42,24 % weniger effektive Tokens. Der Mittelwert liegt bei 567,10 s und
110.533,5 effektiven Tokens. Zwei Läufe sind kein allgemeiner Leistungsnachweis.

In den verifizierbaren Ausgaben sind keine erneut gelieferten Quellzeilen mehr
nachgewiesen (19/24 beziehungsweise 17/32 Befehle mit verifizierten Quellzeilen).
Zu große Anfragen und falsch platzierte JSON-Felder verursachen noch korrigierte
Fehlversuche. Breite Zitatbereiche enthalten teils ungelesene Zwischenzeilen;
Mail-Testrollen sind teilweise nur als Referenzen belegt. Lauf 2 verwechselt an
einer Stelle HTTP-Idempotenz mit identischen Statuscodes. Die feste Punktzahl
bedeutet keine fehlerfreie Antwort oder Release-Freigabe.

Go-Suite/vet, unabhängiger Code-Review, 180 Kontrollen aus 23 unveränderten
Kontextantworten, 115 wiederverwendete Quellabschnitte und 55 reale Find-Prüfaufrufe
bestehen. Quellen, Indizes und 15 Baseline-Dateien sind unverändert; kein Scan
oder erneuter Lauf ohne GoreGraph. Rohdaten liegen unter `combined-read*` im
vorhandenen privaten Benchmark-Verzeichnis; dieser Nachlauf ist historisch.

## Vorheriger Nachlauf: Testinventar und Quellenwiederverwendung

Der [vorherige Nachlauf](AGENT-INVENTORY-REUSE-2026-09-11.md) erreichte ebenfalls
12/12 Kernpunkte und 7/7 Testreferenzen in beiden Läufen, bei 769,79 s / 123.863
und 605,25 s / 109.660 effektiven Tokens. Der damalige Kandidat `14d3c9701f67…`
bleibt mit seinen Ergebnissen unter `inventory-reuse-followup*` erhalten. Die
neue kombinierte Such-/Lesefunktion war darin noch nicht enthalten.

## Historischer Stand nach drei Fix- und Prüfrunden

Die drei ausdrücklich beauftragten Adaptive-CLI-Runden sind abgeschlossen;
[Details, Fixes und Grenzen](AGENT-THREE-CYCLE-2026-09-10.md).
Alle drei erreichen 12/12 Kernkriterien. Zeit/effektive Tokens: 414,11 s/88.443,
442,45 s/88.411 und 485,95 s/108.888. Die letzte Version ist lokal als
1.4.1-Entwicklungsstand vom 10.09.2026, 20:58:40 UTC installiert.
Go-Suite/vet, 21 lokale Suchkontrollen mit 152 Prüfungen und Gesamt-Review bestehen;
Quellen/Indizes unverändert, kein Neuscan. Die gespeicherte Baseline wurde nie
wiederholt. Weiter offen: nur 5/7 relevante Testdateien im letzten Ergebnis,
wiederholte Quelllesevorgänge und keine stabile Verbesserung gegenüber dem besten
Adaptive-Lauf. Keine Release-Freigabe. Die folgenden Messwerte sind historisch.

## Historisches Ergebnis vom 09.09.2026

Der damalige lokale GoreGraph-1.4.1-Stand findet den richtigen Einstiegspunkt, liefert
aber noch keinen hinreichenden Kontext für die vollständige projektübergreifende
Fehleranalyse. Die gemessene Tokenreduktion ist deshalb **kein Nachweis einer
Einsparung bei vergleichbarer Qualität** und keine Release-Freigabe.

| Variante | Läufe | Effektive Tokens, Median | Gesamttokens, Median | Sekunden, Median |
|---|---:|---:|---:|---:|
| ohne GoreGraph | 3 | 137215 | 1191512 | 453,8 |
| mit GoreGraph | 3 | 28915 | 71155 | 166,5 |

- Effektive Tokenreduktion: 78,93 %.
- Gesamttokenreduktion: 94,03 %.
- Laufzeitreduktion: rund 63,3 %, entsprechend etwa 2,7-facher Geschwindigkeit.
- Effektive Tokens = Input minus gecachter Input plus Output.
- Gesamttokens = Input plus Output, einschließlich gecachtem Input.
- Das ist keine Kostenrechnung: gecachte Tokens sind nicht pauschal kostenlos.
- Drei Läufe pro Variante sind eine kleine Stichprobe. Cache-Anteile und Laufzeit
  schwanken; der erste neue GoreGraph-Lauf hat besonders viel gecachten Input.

Die vorher angezeigten 84,46 % effektive / 96,58 % gesamte Tokenreduktion sind
**ungültig**: Die drei damaligen GoreGraph-Aufrufe scheiterten an Leserechten.
Sie sind separat unter `fehlstarts/read-only-context-failures` archiviert und
werden nicht mehr im aktuellen Vergleich berücksichtigt.

Die historischen etwa 86 % Ersparnis stammen aus einer anderen Messreihe mit
anderem Prompt/Setup. Sie sind weder durch dieses Ergebnis widerlegt noch für
diesen Kandidaten bestätigt.

## Inhaltliche Prüfung der neuen Antworten

Geprüft wurden alle drei neuen `answer.md` und die abgeschlossenen Tool-Aufrufe
in ihren `events.jsonl`, im Vergleich zur vorhandenen Referenz und den Baselines.
Dies ist eine Prüfung der Kernkriterien, keine abgeschlossene Einzelbewertung
sämtlicher Pfad-/Zeilenangaben und kein vollständiger 12-Punkte-Score.

| Kernkriterium | Neue GoreGraph-Läufe |
|---|---|
| Öffentlicher Lösch-Einstiegspunkt korrekt | 3 von 3 |
| Beide betroffenen Aufgabenarten erkannt | 1 von 3 |
| Vollständiger Korrekturpfad über gemeinsamen Client und Task-Service | 0 von 3 |
| Konkrete vorhandene Testdateien benannt | 0 von 3 |
| GoreGraph-Kontextaufruf technisch erfolgreich | 3 von 3 |

Die ersten beiden Antworten bleiben bei unspezifischen abhängigen Informationen;
sie bestimmen weder die Aufgabenarten noch den verantwortlichen Provider.
Die dritte nennt `CadasterRegTaskEntity` und `CadasterRegChangeTaskEntity`, liefert
aber nicht die erforderliche gemeinsame Client-/Provider-Kette und bezeichnet
eine Änderung in `ms-common` als bisher nicht begründbar. Alle drei benennen
fehlende Testevidenz. Die Baseline-Antworten ohne GoreGraph gehen bei diesen
Kernpunkten deutlich weiter.

Jeder neue Lauf führte genau einen erfolgreichen `goregraph context`-Aufruf und
einen anschließenden Leseaufruf für drei begrenzte Auslassungsbereiche aus.
Der Kontext meldete `Confidence: MEDIUM` und `Source coverage: partial`;
`Source unrepresented` war 1, 1 bzw. 2. Die sichtbaren Leseaufrufe hielten sich
an die begrenzten Nachlesestellen; eine vollständige formale Regelauswertung
bleibt separat von der fachlichen Bewertung.

**Diagnose des Werkzeugproblems:** Der Einstieg wird gefunden, die Auswahl der
weiterführenden Evidenz ist unzureichend. Die strengen `strict-v1`-Regeln erlauben
dem Agenten nur gelieferte Quellbereiche und begrenzte `source_omissions`;
fehlende relevante Evidenz lässt sich damit nicht frei nachrecherchieren.
Das ist anhand der Antworten und Aufrufe belegt; die genaue Ranking-/Auswahlursache
im GoreGraph-Code ist noch zu untersuchen. Nicht vorschnell nur das Tokenbudget
vergrößern oder den Benchmark-Prompt um die bekannte Lösung ergänzen.

Die vorherige Vorabprüfung war zu schwach: richtiger Einstieg, zwölf Quellbereiche
aus drei Projekten und kein Fallback beweisen keine vollständige fachliche Abdeckung.

## Referenzfehler und erwarteter Korrekturpfad

Symptom: Beim Entfernen einer Vorschrift aus einem Kataster bleiben normale
Vorschriftenaufgaben und Änderungsaufgaben bestehen. Es sind zwei laufende
Services plus eine gemeinsame Client-Bibliothek, keine drei laufenden Services.

Bestehender Einstieg:

```text
DELETE /cadasters/{cadasterId}/regulations/{objectId:.+}
CadasterRegulationController.deleteFromCadaster
  -> CadasterRegulationOperationsService.deleteRegulationFromCadaster
     -> Zugriff / Zuordnung prüfen
     -> Oracle del_regulation_f
  -> Benutzertracking und Erfolgsantwort
```

Zu untersuchende fehlende Bereinigung, ausdrücklich keine bestehende Aufrufkante:

```text
ms-cadasterregulation: Löschablauf
  -> ms-common: CadasterTaskMgmtService, neue Bereinigungsoperation
  -> ms-cadastertask: Management-Controller / Task-Service
     -> normale Vorschriftenaufgaben UND Änderungsaufgaben
```

Beide Familien müssen passend zu `cadasterId` und `objectId` behandelt werden;
andere Kataster/Vorschriften bleiben unberührt. Vorhandene Protokollierungs-,
Lösch- und gegebenenfalls Mail-Nebenwirkungen müssen aus Quellen belegt werden.
Oracle-Funktionskörper, Cascades und serviceübergreifende Transaktion sind nicht
durch den Java-Aufruf bewiesen. Neue Methoden, Routen und Lookup-Verfahren müssen
als Ergänzung/Designentscheidung bezeichnet werden. Die historischen Namen einer
späteren Lösung zu erraten ist kein Erfolgskriterium.

Zentrale vorhandene Dateien unter `microservices/`:

| Projekt | Pfad relativ zum Projekt | Rolle |
|---|---|---|
| ms-cadasterregulation | src/main/java/com/weka/vd/api/cadasterregulation/controller/CadasterRegulationController.java | Öffentlicher Einstieg, Tracking |
| ms-cadasterregulation | src/main/java/com/weka/vd/api/cadasterregulation/service/CadasterRegulationOperationsService.java | Bestehender Löschpfad |
| ms-common | src/main/java/com/weka/common/cadastertask/CadasterTaskMgmtService.java | Gemeinsamer Client |
| ms-common | src/main/java/com/weka/common/cadastertask/CadasterTaskMgmtConfig.java | Client-Konfiguration |
| ms-cadastertask | src/main/java/com/weka/vd/api/cadastertask/controller/CadasterTaskMgmtController.java | Interne Management-API |
| ms-cadastertask | src/main/java/com/weka/vd/api/cadastertask/service/CadasterTaskService.java | Bestehende Aufgaben-Löschabläufe |
| ms-cadastertask | src/main/java/com/weka/vd/api/cadastertask/repository/CadasterRegTaskRepository.java | Normale Aufgaben |
| ms-cadastertask | src/main/java/com/weka/vd/api/cadastertask/repository/CadasterRegChangeTaskRepository.java | Änderungsaufgaben |
| ms-cadasterregulation | src/test/java/com/weka/vd/api/cadasterregulation/controller/CadasterRegulationControllerDeleteTest.java | Einstiegstests |
| ms-cadastertask | src/test/java/com/weka/vd/api/cadastertask/controller/CadasterTaskMgmtControllerTest.java | Management-Contract-Tests |

## Stand von Code, Installation und Workspace

- GoreGraph-Implementierung: Commit `5bbb9edc6944842a1d348989dbaf46e52f38904d`
  auf `main`. Die vorliegende Übergabe wird danach separat committed.
- Lokale Version: 1.4.1, kein Release-Tag, keine Paketveröffentlichung.
- Getestete Windows-Binärdatei wurde vor dem Commit aus dem damaligen Arbeitsstand
  gebaut: Buildzeit `2026-09-09T14:42:09Z`, Commitlabel
  `081b405383692c27cbae254e949c6ea947f4d6f8-dirty`, Go 1.26.4, Schema 3.
- SHA256 der getesteten Binärdatei:
  `D61D9C486B4C6C313BFAF518B37D519CCC1EA0DF0546906CFED86CC0EF396C79`.
- Installation auf dem bisherigen Client:
  `C:/Users/goretzkh/scoop/apps/goregraph/1.4.1/goregraph.exe`;
  Scoop `current` und Shim wählen diese lokale Version.
- Testworkspace:
  `C:/Users/goretzkh/projects/testing/0442483-backend-before-fix`.
- Die drei Projekte und der Workspace wurden am 09.09.2026 mit lokaler 1.4.1 neu
  gescannt; 368 erfasste Quelldateien blieben unverändert. Die späteren Read-only-
  und Retrieval-Fixes benötigten keinen neuen Scan auf diesem Client.
- Getrennte Index-/Agent-/Dashboard-Ausgaben waren vollständig erzeugt.
  Das sagt nichts über die fachliche Vollständigkeit einer konkreten Query aus.

| Projekt | Eingefrorener Git-Stand |
|---|---|
| ms-cadasterregulation | 43c93a4a45be7af46f0ccd62065c74a62e58606d |
| ms-cadastertask | 6dc6f612d1c4de92b4adbdea9467099b33309f3e |
| ms-common | 2e46fc6f80f9bccf472ad82708615df28280577e |

Bereits korrigiert: gemeinsame Lesesperren ohne Schreibzugriff, sichere
Windows-Pfadauflösung im Read-only-Sandbox und begrenzte Suche nach Query-Begriffen
in bereits indexierten Mutation-Handlern. Die breitere Relevanzlücke bleibt offen.
Relevante Einstiegspunkte für die Entwicklung:
`internal/agent/context_source_search.go`, `context_rank.go`, `context_source.go`,
`context_verification.go`, `context.go` sowie `internal/agentguide/instruction.go`.

Zusätzlicher offener Gate: [AGENT-DEVELOPMENT-ACCEPTANCE.md](AGENT-DEVELOPMENT-ACCEPTANCE.md)
dokumentiert die bisher nicht bestandene breitere Retrieval-Prüfung. Die drei
oben beschriebenen Queries schließen diesen Gate nicht.

## Fortsetzung am anderen Client

1. Repository auf den neuesten `main`-Stand bringen und diese Datei lesen.
2. Für Codearbeit genügen Repository und synthetische Fixtures. Für erneute
   Prüfung dieses privaten Falls zusätzlich Testworkspace und Benchmark-Artefakte
   über einen geeigneten internen Transfer auf den anderen Client übernehmen.
   Dieser Git-Push überträgt weder private Servicequellen noch Rohlogs oder Runner.
3. Den vollständigen folgenden Artefaktordner übernehmen, insbesondere Runner,
   Checker, Promptdateien, sechs Ergebnisordner, Hashnachweise und Prüferreferenz:

```text
C:/Users/goretzkh/.codex/visualizations/2026/09/09/01a0856a-77b7-7d73-b1ea-fcdbb87a25d6/goregraph-benchmark-0442483
```

Wichtige Dateien: `benchmark.ps1`, `context-check.ps1`, `test-start.ps1`,
`test-context-check.ps1`, `test-ohne-goregraph.txt`, `test-mit-goregraph.txt`,
`bewertung-nur-fuer-pruefer.md`, `baseline-preserved-sha256.json`,
`sources-before-rescan.json`, `rescan-1.4.1-verification.json`, `ergebnisse/`.
Je Lauf sind `answer.md`, `events.jsonl`, `metrics.json`, `settings.json`,
`arguments.json`, `prompt.txt` und `stderr.log` vorhanden.
Die nachstehenden Prompts und Messwerte sind zusätzlich direkt in Git gesichert.

4. Neue Maschine: PowerShell 7, Codex CLI und lokale GoreGraph-Version prüfen.
   Die alte Scoop-Installation wird nicht mit Git übertragen. Beim Neuaufbau
   Buildidentität dokumentieren; bei anderem Workspace-Pfad Ausgaben auf der
   neuen Kopie vorbereiten/gegebenenfalls neu scannen. Fehlende Lockdateien vor
   dem Read-only-Lauf einmal mit Schreibrechten initialisieren.
5. Zuerst die drei tatsächlichen Queries unten direkt gegen GoreGraph prüfen.
   Fachliche Kriterien testen: beide Aufgabenfamilien, gemeinsamer Client,
   Provider, relevante Persistenz und vorhandene Testpfade. Keine Lösungshinweise
   oder privaten Namen in allgemeine Rankinglogik einbauen; generische
   Regressionstests ergänzen. Gezielt fehlende Evidenz nachladbar machen und
   Änderungen am Protokoll getrennt vom bisherigen `strict-v1` auswerten.
6. Erst nach bestandener direkter fachlicher Prüfung neue kostenpflichtige
   Agent-Läufe starten. Die bisherigen Baselines bewahren; die drei aktuellen
   GoreGraph-Ergebnisse als abgeschlossene Diagnose-Serie archivieren und nicht
   mit einer verbesserten Variante zusammenmitteln.
7. Bei Clientwechsel sind Pfade, CLI-/Config-Hashes und eventuell das Modell
   anders. Historische Baselines bleiben Referenz, aber neue Messungen dürfen
   nicht durch Manipulation von `settings.json` als identisch ausgegeben werden.
   Der Runner verweigert gemischte Einstellungen absichtlich. Kein neuer
   Ohne-GoreGraph-Lauf ist für die reine Weiterentwicklung erforderlich; ein
   streng kontrollierter neuer Clientvergleich braucht nachgewiesene gleiche
   Bedingungen oder eine getrennte neue Baseline.

Die ursprünglichen Einzeiler im übernommenen Runner-Ordner lauten:

```powershell
pwsh -NoProfile -File ./benchmark.ps1 -Mode mit
pwsh -NoProfile -File ./benchmark.ps1 -Mode vergleich
```

Für eine neue Serie und abweichenden Workspace Parameter setzen:

```powershell
pwsh -NoProfile -File ./benchmark.ps1 -Mode mit -Workspace 'C:/Pfad/zur/Testkopie' -Results 'C:/Pfad/Serie2'
```

Eine Serie mit ausschließlich `mit`-Läufen lässt sich vom bestehenden Runner
nicht als gepaarter Vergleich auswerten. Keine alten oder fehlgeschlagenen Läufe
löschen, um eine bessere Kennzahl zu erzeugen. Ein kontrollierter Vergleich muss
Varianten, Setupänderungen und Qualitätsbefunde offen ausweisen.

## Messsetup und Einzelwerte

Codex CLI: 0.153.4. Modell: `CLI configuration default`, kein explizites `-Model`.
Ein konkreter aufgelöster Modellname wurde in `settings.json` nicht festgehalten;
auf einem anderen Client darf nicht derselbe Default vorausgesetzt werden.
Reasoning: `high`; Sandbox: `read-only`; Genehmigungen: `never`.
`codex exec` lief mit `--json --ephemeral --color never --skip-git-repo-check`,
`-C <workspace>`, `-c model_reasoning_effort="high"`, `-o <answer.md>` und `-`.
Der Runner übergibt den vollständigen mehrzeiligen Prompt per UTF-8-stdin;
interaktive mehrzeilige Eingabe ist nicht nötig. Er misst die vom CLI gemeldeten
Tokens aus `turn.completed`, keine Schätzung des antwortenden Agenten.

Setup-Hashes der gespeicherten Läufe:

- Codex: `444A3F0008050605CAE73CD9B7A2DCAC61294062DFAAB56DD20430FD6498518B`
- Config: `CF378E2168F80D0021458821989067491B1EAFED281FAFCE235DD6DAAD5EC464`
- Prompt ohne: `C057B36166B04B9D8D17DEFC45B140D4F99FCA16D1CD3CBF04201547CA38B0AC`
- Prompt mit: `7BC29F1F16CA44D562945EEEC7BBD203CB49B83DD9EAF8E5127A81A41FDAD08F`

Die Hashes beziehen sich auf Originaldateien, einschließlich Kodierung und
Zeilenenden. Die Markdown-Blöcke bewahren den Text; beim Herauslösen können
andere Zeilenenden einen anderen Datei-Hash ergeben.

| Lauf | Input | Cached Input | Output | Effektiv | Gesamt | Sekunden |
|---|---:|---:|---:|---:|---:|---:|
| 20260909-152457-ohne-61fcd963 | 1196418 | 1070976 | 11773 | 137215 | 1208191 | 441.054 |
| 20260909-153226-ohne-524095e6 | 994375 | 887680 | 13053 | 119748 | 1007428 | 453.767 |
| 20260909-154013-ohne-13e8697a | 1178746 | 1048448 | 12766 | 143064 | 1191512 | 464.741 |
| 20260909-164937-mit-0275e1af | 66411 | 61056 | 4658 | 10013 | 71069 | 160.524 |
| 20260909-165451-mit-575ee980 | 66419 | 42240 | 4736 | 28915 | 71155 | 166.51 |
| 20260909-165818-mit-970d6ddd | 66970 | 42112 | 5236 | 30094 | 72206 | 191.351 |

## Vollständiger Agent-Prompt: ohne GoreGraph

```text
Analysiere ausschließlich lesend den aktuellen Quellstand dieses Workspaces.

Beobachtetes Verhalten:
Das Entfernen einer Vorschrift aus einem Kataster wird gegenüber dem Benutzer erfolgreich abgeschlossen. Danach ist der fachliche Zustand jedoch nicht vollständig konsistent: In späteren Abläufen können weiterhin Informationen auftauchen, die sich auf die entfernte Vorschrift beziehen.

Ermittle die wahrscheinlichste Ursache und einen minimalen Korrekturplan. Setze weder verantwortliche Komponenten noch betroffene Datentypen voraus; leite sie aus der verfügbaren Evidenz ab.

Rahmen:
- Keine Änderungen, Installationen, Builds oder Testausführungen.
- Keine Subagenten, Netzwerkzugriffe, Git-Befehle, Git-Metadaten oder anderen Branches.
- Keine Dateien außerhalb des aktuellen Workspaces und keine früheren Antworten, Tickets, Musterlösungen oder Benchmark-Ergebnisse verwenden.
- Unterscheide belegten Ist-Zustand, Schlussfolgerungen und vorgeschlagene Änderungen. Behaupte nicht, dass vorgeschlagene Methoden oder Schnittstellen bereits existieren.
- Kennzeichne nicht belegbare Details ausdrücklich als unbekannt. Beschreibe statisch plausible Tests, ohne eine erfolgreiche Ausführung zu behaupten.
- Laufzeit, Tokens und Werkzeugstatistiken werden extern gemessen. Ermittle oder schätze diese Werte nicht selbst.

Liefere eine kompakte, aber vollständige Antwort mit:
1. Diagnose und Konfidenz: Ursache, Evidenz und verbleibende Alternativerklärungen.
2. Öffentlichem Einstiegspunkt: HTTP-Methode, Route und implementierende Methode.
3. Bestehender Aufrufkette bis zu den Persistenzoperationen. Zeige die vermutete Lücke gesondert; zeichne fehlende Aufrufe nicht als bestehende Kanten ein.
4. Betroffenen Projekten, Datenvarianten, Zuordnungsmerkmalen und fachlichen Nebenwirkungen.
5. Minimalem Korrekturplan über die erforderlichen Projektgrenzen. Behandle interne Schnittstellen, Authentifizierung, Konfiguration, Persistenz, Fehlerfälle und Wiederholungen. Trenne vorhandene Mechanismen von offenen Designentscheidungen; gib keine Zugangsdaten aus.
6. Getrennten Inventaren vorhandener Produktions-, Konfigurations- und Testdateien. Pro Datei: exakter workspace-relativer Pfad, Symbol, belegte Zeile beziehungsweise Zeilenbereich, Rolle und Einordnung als Änderungsziel oder Referenz. Kennzeichne nur über Metadaten bekannte Dateien; erfinde keine Zeilenangaben.
7. Konkreten Regressionstests mit Ausgangszustand, Aktion und erwarteter Wirkung, einschließlich Abgrenzungs- und Fehlerfällen.
8. Offenen Evidenzlücken. Erkläre, welche Aussagen damit nicht abgesichert sind.

Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.
```

## Vollständiger Agent-Prompt: mit GoreGraph

```text
Analysiere ausschließlich lesend den aktuellen Quellstand dieses Workspaces.

Beobachtetes Verhalten:
Das Entfernen einer Vorschrift aus einem Kataster wird gegenüber dem Benutzer erfolgreich abgeschlossen. Danach ist der fachliche Zustand jedoch nicht vollständig konsistent: In späteren Abläufen können weiterhin Informationen auftauchen, die sich auf die entfernte Vorschrift beziehen.

Ermittle die wahrscheinlichste Ursache und einen minimalen Korrekturplan. Setze weder verantwortliche Komponenten noch betroffene Datentypen voraus; leite sie aus der verfügbaren Evidenz ab.

Rahmen:
- Keine Änderungen, Installationen, Builds oder Testausführungen.
- Keine Subagenten, Netzwerkzugriffe, Git-Befehle, Git-Metadaten oder anderen Branches.
- Keine Dateien außerhalb des aktuellen Workspaces und keine früheren Antworten, Tickets, Musterlösungen oder Benchmark-Ergebnisse verwenden.
- Unterscheide belegten Ist-Zustand, Schlussfolgerungen und vorgeschlagene Änderungen. Behaupte nicht, dass vorgeschlagene Methoden oder Schnittstellen bereits existieren.
- Kennzeichne nicht belegbare Details ausdrücklich als unbekannt. Beschreibe statisch plausible Tests, ohne eine erfolgreiche Ausführung zu behaupten.
- Laufzeit, Tokens und Werkzeugstatistiken werden extern gemessen. Ermittle oder schätze diese Werte nicht selbst.

Liefere eine kompakte, aber vollständige Antwort mit:
1. Diagnose und Konfidenz: Ursache, Evidenz und verbleibende Alternativerklärungen.
2. Öffentlichem Einstiegspunkt: HTTP-Methode, Route und implementierende Methode.
3. Bestehender Aufrufkette bis zu den Persistenzoperationen. Zeige die vermutete Lücke gesondert; zeichne fehlende Aufrufe nicht als bestehende Kanten ein.
4. Betroffenen Projekten, Datenvarianten, Zuordnungsmerkmalen und fachlichen Nebenwirkungen.
5. Minimalem Korrekturplan über die erforderlichen Projektgrenzen. Behandle interne Schnittstellen, Authentifizierung, Konfiguration, Persistenz, Fehlerfälle und Wiederholungen. Trenne vorhandene Mechanismen von offenen Designentscheidungen; gib keine Zugangsdaten aus.
6. Getrennten Inventaren vorhandener Produktions-, Konfigurations- und Testdateien. Pro Datei: exakter workspace-relativer Pfad, Symbol, belegte Zeile beziehungsweise Zeilenbereich, Rolle und Einordnung als Änderungsziel oder Referenz. Kennzeichne nur über Metadaten bekannte Dateien; erfinde keine Zeilenangaben.
7. Konkreten Regressionstests mit Ausgangszustand, Aktion und erwarteter Wirkung, einschließlich Abgrenzungs- und Fehlerfällen.
8. Offenen Evidenzlücken. Erkläre, welche Aussagen damit nicht abgesichert sind.

Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller's problem statement and requested evidence scope in the query.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; make the file reader itself range-bounded, for example with sed -n, and never pipe a whole-file reader such as nl through a downstream range filter. Do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
For change plans, include separate exact existing production-file and test-file inventories from files, source_sections, production_plan_files, plan_files, or bounded omission reads; name every supplied production_plan_files identity in the production-file inventory with its role because naming metadata is not reading source; name every supplied plan_files identity in the test-file inventory with its use because naming metadata is not reading source, provider_test entries may be test targets, and mock_pattern or retry_pattern entries are reference patterns, not change targets. Never read production_plan_files or plan_files unless source_omissions lists the same exact path with a bounded range; do not invent future filenames, and keep future route, authentication, status, lookup implementation, dependent persistence and cascade behavior, and cross-service transaction ordering as unknown design decisions unless rendered source proves them.
When authentication or configuration is requested, report supplied server authorization policy, client authentication construction and configuration fields, and exact paths of supplied production and test-profile resources together in one coherent answer section; name every supplied configuration_resources identity with its project, profile, and key groups, and distinguish current evidence, required additions, and unknown deployment values.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
```

## Tatsächliche GoreGraph-Queries aus den drei neuen Läufen

Diese Queries wurden vom jeweiligen Agenten aus dem identischen Mit-Prompt formuliert. Die leichten Unterschiede sind Bestandteil der beobachteten Variantenempfindlichkeit. Es handelt sich um Tool-Queries, nicht um zusätzliche ursprüngliche Benutzer-Prompts.

### 20260909-164937-mit-0275e1af

```text
Das Entfernen einer Vorschrift aus einem Kataster wird gegenüber dem Benutzer erfolgreich abgeschlossen. Danach ist der fachliche Zustand jedoch nicht vollständig konsistent: In späteren Abläufen können weiterhin Informationen auftauchen, die sich auf die entfernte Vorschrift beziehen. Ermittle die wahrscheinlichste Ursache und einen minimalen Korrekturplan, ohne verantwortliche Komponenten oder betroffene Datentypen vorauszusetzen. Benötigte Evidenz: öffentlicher Einstiegspunkt mit HTTP-Methode, Route und implementierender Methode; bestehende Aufrufkette bis zu Persistenzoperationen und mögliche fehlende Aufrufe; betroffene Projekte, Datenvarianten, Zuordnungsmerkmale und fachliche Nebenwirkungen; erforderliche Projektgrenzen, interne Schnittstellen, Authentifizierung, Konfiguration, Persistenz, Fehlerfälle und Wiederholungen; vorhandene Produktions-, Konfigurations- und Testdateien mit Symbolen und Quellzeilen; statisch plausible Regressionstests mit Abgrenzungs- und Fehlerfällen sowie offene Evidenzlücken.
```
### 20260909-165451-mit-575ee980

```text
Das Entfernen einer Vorschrift aus einem Kataster wird gegenüber dem Benutzer erfolgreich abgeschlossen. Danach ist der fachliche Zustand jedoch nicht vollständig konsistent: In späteren Abläufen können weiterhin Informationen auftauchen, die sich auf die entfernte Vorschrift beziehen. Ermittle die wahrscheinlichste Ursache und einen minimalen Korrekturplan, ohne verantwortliche Komponenten oder betroffene Datentypen vorauszusetzen. Benötigte Evidenz: öffentlicher Einstiegspunkt mit HTTP-Methode, Route und implementierender Methode; bestehende Aufrufkette bis zu Persistenzoperationen und vermutete Lücke; betroffene Projekte, Datenvarianten, Zuordnungsmerkmale und fachliche Nebenwirkungen; erforderliche Projektgrenzen, interne Schnittstellen, Authentifizierung, Konfiguration, Persistenz, Fehlerfälle und Wiederholungen; vorhandene Produktions-, Konfigurations- und Testdateien mit Symbolen und Quellzeilen; statisch plausible Regressionstests mit Abgrenzungs- und Fehlerfällen sowie verbleibende Alternativerklärungen und Evidenzlücken.
```
### 20260909-165818-mit-970d6ddd

```text
Das Entfernen einer Vorschrift aus einem Kataster wird gegenüber dem Benutzer erfolgreich abgeschlossen. Danach ist der fachliche Zustand jedoch nicht vollständig konsistent: In späteren Abläufen können weiterhin Informationen auftauchen, die sich auf die entfernte Vorschrift beziehen. Ermittle die wahrscheinlichste Ursache aus der verfügbaren Evidenz: öffentlicher HTTP-Einstiegspunkt und implementierende Methode, bestehende Aufrufkette bis zu Persistenzoperationen, betroffene Projekte, Datenvarianten, Zuordnungsmerkmale und fachliche Nebenwirkungen. Benötigt werden Belege für einen minimalen Korrekturplan über erforderliche Projektgrenzen mit internen Schnittstellen, serverseitiger Autorisierung, clientseitiger Authentifizierung, Konfigurationsfeldern und Profilressourcen, Persistenz, Fehlerfällen und Wiederholungen sowie vorhandene Produktions-, Konfigurations- und Testdateien und statisch plausible Regressionstests einschließlich Abgrenzungs- und Fehlerfällen. Verantwortliche Komponenten und Datentypen sind aus der Evidenz abzuleiten; verbleibende Alternativerklärungen und Evidenzlücken sind zu ermitteln.
```

## Bewertungsraster für die Fortsetzung

Je ein Punkt nur für konkrete, korrekte und belegte Aussagen; derzeit wurde kein vollständiger Score vergeben:

1. Öffentlicher Endpoint einschließlich Zuordnung zum Controller.
2. Tatsächlich bestehende Aufrufkette bis zur Persistenzgrenze.
3. Fehlende Bereinigung als plausible Ursache, sauber von unbekannten DB-Details getrennt.
4. Notwendige Kette über Vorschriften-Service, gemeinsamen Client und Task-Service.
5. Beide Aufgabenfamilien.
6. Passende Zuordnung über cadasterId und objectId; andere Kataster/Vorschriften bleiben unberührt.
7. Interner Contract als erforderliche Ergänzung, ohne erfundene Ist-Route.
8. Vorhandene Server-/Client-Authentifizierung und Konfigurationsorte; keine erfundenen Deploymentwerte.
9. Persistenzoperationen und betroffene Repositories/Entities; keine unterstellten Cascades.
10. Geschäftliche Nebenwirkungen wie Protokollierung und belegte Mail-/Löschabläufe.
11. Korrekte bestehende Produktions-/Konfigurations-/Testpfade und passende Rollen.
12. Fehler-, Wiederholungs- und Teststrategie mit Ausgangsdaten, Aktion und erwarteter Wirkung.

Zusätzlich separat zählen:

- Gültige vorhandene Pfade / alle als vorhanden ausgegebenen Pfade.
- Korrekte Symbol-/Zeilenbelege / alle prüfbaren Belege.
- Abdeckung der als notwendig bewerteten Produktions- und Testdateien.
- Falsch als bestehend dargestellte Aufrufkanten, Methoden oder Contracts.
- Unbelegte Details und fehlende Kernpunkte.

Pfadgenauigkeit allein reicht nicht: eine richtige, aber unvollständige Liste
kann eine schlechte Analyse sein. Metadaten-Nennungen sind nicht mit gelesener
Source-Evidenz gleichzusetzen. Bewertung möglichst blind ohne Variantenlabel;
unklare oder unvollständige Antworten nicht wegen niedrigen Verbrauchs bevorzugen.
