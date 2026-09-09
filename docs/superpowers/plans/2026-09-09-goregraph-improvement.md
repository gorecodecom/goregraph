# GoreGraph: vollständiger Verbesserungsplan

Stand: 9. September 2026. Planungsgrundlage: GoreGraph 1.4.0, Commit `081b405` und die lokale Projektprüfung.

**Ziel:** GoreGraph soll Menschen und Coding-Agents zuverlässig zu den richtigen Codezusammenhängen führen. Beide Einsatzbereiche haben dieselbe Priorität. Bei Coding-Agents zählt besonders, ob sie eine korrekte Lösung schneller und mit weniger tatsächlich verbrauchten Tokens erreichen.

Dieser Plan beschreibt die Umsetzung. Die Anwendung, installierte Version und WEKA-Indizes wurden dafür nicht verändert.

## Dokumente

1. [Architektur, Entscheidungen und Abnahmekriterien](../specs/2026-09-09-goregraph-improvement-design.md)
2. [A: Zuverlässige und schnelle Scans](2026-09-09-scan-reliability.md)
3. [B: Vertrauenswürdige Ausgaben und nützliches Dashboard](2026-09-09-output-trust.md)
4. [C: Wirksamkeit für Coding-Agents und Gesamtvalidierung](2026-09-09-agent-effectiveness.md)

Die technischen Detailpläne folgen der englischen Dokumentationssprache des Repositories. Sie enthalten betroffene Dateien, neue Schnittstellen, konkrete Regressionstests, Umsetzungsschritte und Prüfbefehle. Neue Dateinamen und Schnittstellen darin sind ausdrücklich geplante Bestandteile.

## Umfang und Reihenfolge

| Arbeitspaket | Ergebnis | Voraussetzung |
|---|---|---|
| A0: Ausgangslage messen | Reproduzierbare Performance-Werte und Einordnung bestehender Testfehler | Keine |
| A1: Ignore-Regeln und Dateiauswahl | Verschachtelte Build-Ausgaben bleiben aus Scan und Update ausgeschlossen | A0 |
| A2: Fortschritt und Abbruch | Sichtbare Phase/Datei, klare Zeitbudgets, kontrollierter Abbruch | A1 |
| A3: Script-Analyse beschleunigen | Einmalige Ermittlung von Gültigkeitsbereichen statt wiederholter Dateidurchläufe | A0, A2 |
| A4: Updates korrekt entscheiden | Quellen, Ignore-Regeln, Konfiguration und Analyzer-Revisionen bestimmen den Neuaufbau | A1–A3 |
| B1: Ausgaben sicher veröffentlichen | Wiederherstellbarer letzter erfolgreicher Stand auch nach Schreibfehlern | A2, A4 |
| B2: Zustand verständlich darstellen | Aktualität, Datenintegrität und Analyseabdeckung werden getrennt ausgewiesen | B1 |
| B3: Sichere Wiederverwendung | Unveränderte Script-Dateien müssen bei Änderungen nicht erneut vollständig analysiert werden | A3, A4, B1; Profiling bestätigt Nutzen |
| B4: Laden und Projektionen optimieren | Nur gemessene unnötige Arbeit wird entfernt; vorhandenes Lazy Loading wird genutzt | A0, B1, C0 |
| B5: Drei vollständige Nutzerwege | Request verfolgen, Änderungsfolgen verstehen und passende Tests finden | B2; B4 bei relevanten Ladeproblemen |
| C0: Unabhängige Aufgaben definieren | Bewertbare Coding- und Analyseaufgaben einschließlich schwieriger Gegenbeispiele | A0; früh beginnen |
| C1: Agent-Protokolle versionieren | Historischer strenger Ablauf bleibt vergleichbar; adaptiver Ablauf wird separat prüfbar | C0 |
| C2: Gezielte Nachprüfung ermöglichen | Konkrete begrenzte Folgeprüfungen und ehrliche Gründe für einen Fallback | C1, B2 |
| C3: Kontext relevanter machen | Weniger redundante Angaben, mehr unmittelbar benötigte Evidenz | C0, C2 |
| C4: Ganze Aufgaben vergleichen | Korrektheit, Zeit und Tokens einschließlich Nachsuchen und Korrekturen | A/B, C1–C3; Pilot früher möglich |
| C5: Gesamtfreigabe vorbereiten | Vollständige Tests, reale Abnahme und dokumentiertes Upgrade/Rollback | Alle fachlichen Prüfkriterien |

Die 16 Pakete sind getrennt überprüfbar. Sie sind keine Aufforderung zu 16 gleichzeitig laufenden Änderungen. Gemeinsame Scanner- und Ausgabeänderungen werden nacheinander integriert. Aufgaben- und Bewertungsdesign beginnen früh, damit Agent-Nützlichkeit die Umsetzung von Anfang an beeinflusst.

## Geplante Verbesserungen im Alltag

### Scans und Updates

- `.gitignore` funktioniert auch für verschachtelte Ordner und untergeordnete Ignore-Dateien.
- Die Anzeige nennt aktuelle Phase, Datei und tatsächlichen Fortschritt.
- Langsame Dateien haben ein Zeitbudget; unvollständige Analyse wird ausdrücklich ausgewiesen.
- Ein unverändertes Update extrahiert nichts erneut und schreibt keine neuen Generationen.
- Ein Analyzer- oder relevanter Konfigurationswechsel löst die nötige Neuanalyse aus.
- Abbrüche und Schreibfehler zerstören den letzten erfolgreichen Datenstand nicht.

### Dashboard

- Ein Frontend-Aufruf führt nachvollziehbar zu Backend, Implementierung und Quellbeleg.
- Änderungen lassen sich bis zu betroffenen Stellen und vorhandenen Tests verfolgen.
- Mehrdeutigkeit und fehlende Analysefähigkeit bleiben sichtbar.
- Alte, unvollständige und beschädigte Daten werden unterschiedlich dargestellt.
- Navigation, Rücksprung, Filter und Auswahl funktionieren über die vorhandenen Ansichten hinweg.

### Coding-Agents

- Der erste Context Pack liefert möglichst direkt die benötigten Quellen und Tests.
- Bereits gelieferte Quellen werden wiederverwendet; redundante Angaben verbrauchen weniger Budget.
- Bei einer belegten Lücke, veralteten Information oder einem Widerspruch ist eine gezielte Nachprüfung möglich.
- Wenn GoreGraph nicht genug weiß, fällt der Agent kontrolliert auf seinen normalen, vom Nutzer erlaubten Arbeitsablauf zurück.
- Historische Benchmarks bleiben unverändert reproduzierbar; ein neuer Ablauf muss seine Verbesserung erst beweisen.

## Wie Erfolg nachgewiesen wird

**Verbindliche Qualitätsbedingungen:** keine falsch als exakt ausgegebenen neuen Beziehungen, keine schlechtere korrekte Aufgabenabschlussrate, keine neuen kritischen Fehländerungen, erfolgreiche Wiederherstellung nach simulierten Schreibabbrüchen und bestehende Tests auf Windows/Linux/macOS.

**Vorgeschlagene Leistungsziele:** mindestens vierfach schnellere Analyse des ausgewählten langsamen Script-Falls; mindestens 50 Prozent weniger Zeit beim Update einer einzelnen Datei; mindestens 25 Prozent weniger effektive End-to-End-Tokens und mindestens 20 Prozent weniger Zeit bis zur verifizierten Aufgabenlösung im definierten Agent-Vergleich.

Diese Werte sind Ziele für kontrollierte Vergleiche, keine bereits gemessenen Verbesserungen. Paketgröße allein ist kein Erfolgskriterium. Erfasst werden auch Fehlschläge, Wiederholungen, Nachsuchen und Korrekturen. Erstnutzung einschließlich Indexaufbau und Nutzung eines bereits vorhandenen Index werden getrennt ausgewiesen.

Für das Dashboard müssen alle drei beschriebenen Nutzerwege mit den erwarteten Belegen erfolgreich durchlaufen werden. Für Agents sind zunächst neun Pilotläufe vorgesehen; die vorgeschlagene vollständige, separat zu budgetierende Vergleichsmatrix umfasst 144 Versuche. Bezahlte Läufe werden durch das Schreiben dieses Plans nicht gestartet.

## Technische Entscheidungen mit bewusster Begrenzung

- Bestehende Architektur schrittweise verbessern; kein Gesamtumbau zu Beginn.
- Bestehende Ausgabewege zunächst erhalten. Wiederherstellbare Schreibtransaktionen werden unter Windows explizit getestet; eine atomare Ersetzung beliebiger Verzeichnisbäume wird nicht unterstellt.
- Cache und zusätzliche Datenstrukturen nur dort einführen, wo Messungen einen Nutzen zeigen.
- Keine neue Datenbank, Sprachfamilie, Cloud-Komponente, automatische Git-Aktualisierung oder UI-Neuentwicklung in diesem Vorhaben.
- Quellschutz, Pfadgrenzen und das Zurückhalten von Konfigurationswerten bleiben erhalten.

## Abschluss und Auslieferung

Die Implementierung ist erst vollständig, wenn Scanner, Dashboard und Agent-Unterstützung ihre Kriterien erfüllen. Danach wird ein Kandidat neben der installierten Version geprüft, einschließlich eines isolierten WEKA-Abnahmelaufs. Erst mit den konkreten Ergebnissen folgen die betrieblichen Schritte für Veröffentlichung, Installation und Aktualisierung der echten Indizes.

Ein Parserwechsel oder eine neue Speicherstruktur wird nur neu geplant, wenn die vorgesehenen gezielten Verbesserungen die vereinbarten Kriterien nachweislich nicht erreichen. Fehlgeschlagene Effizienztests werden dokumentiert und nicht durch nachträgliches Absenken der Qualitätsanforderungen erfolgreich gerechnet.
