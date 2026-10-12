# Vorschlag: belastbare Bestandsaufnahmen mit GoreGraph

Ursprünglicher Vorschlag: 2026-09-22. Historischer Status aktualisiert am 2026-10-12: Der Bestandsaufnahme-Modus und seine Quellenbelege wurden mit 1.4.3 umgesetzt; siehe [Tooling audits](TOOLING-AUDITS.md). Die folgenden Anforderungen dokumentieren die damalige Zielsetzung und sind keine aktuelle Liste offener Aufgaben.

## Anlass und Ziel

In einem Frontend-Monorepo sollte vor der Übernahme eines externen Playwright-Projekts geklärt werden, welche sinnvollen Storybook-Erweiterungen tatsächlich noch fehlen. Dafür werden Konfiguration, Stories, Fixtures, Testausführung, CI und Dokumentation gemeinsam benötigt. Eine Suche nach einem einzelnen Produktions-Einstiegspunkt reicht für diese Fragestellung nicht aus.

GoreGraph soll solche Bestandsaufnahmen mit überprüfbaren Quellen unterstützen und zwischen vorhanden, vorbereitet, deaktiviert, in CI integriert und tatsächlich erfolgreich ausgeführt unterscheiden. Der bisherige präzise Workflow für konkrete Änderungen soll erhalten bleiben.

Zum Zeitpunkt des Vorschlags war die interne Implementierung nicht untersucht. Die folgenden Abschnitte bewahren die damaligen fachlichen Anforderungen; den aktuellen Stand beschreibt die oben verlinkte Dokumentation.

## Ursprünglicher Befund

Eine gewöhnliche Kontextabfrage fand für eine breite Storybook-/Playwright-Bestandsaufnahme keinen zuverlässigen Produktions-Einstiegspunkt. Benötigt wurden zusammengehörige Quellen für Konfiguration, Stories, Fixtures, Runner, CI und Dokumentation. Der interne Projektbericht mit konkreten Pfaden und Inventarzahlen wird hier nicht veröffentlicht. Entscheidend war die Trennung zwischen statischen Deklarationen, wirksamer Aktivierung und nachgewiesener Ausführung.

## Vorgeschlagene Erweiterung

### 1. Vollständigen Query-Transport zuerst prüfen

Eine mehrsätzige deutsche Anfrage mit Begriffen wie Storybook, Playwright, Accessibility und CI muss den gewünschten Evidenzumfang bis zum Retrieval erhalten. Originalanfrage und gegebenenfalls normalisierte Suchanfrage sollen diagnostisch unterscheidbar sein. Ein gekürztes Anzeigefeld darf nicht als alleiniger Nachweis für Query-Verlust interpretiert werden.

Die zulässigen Grenzen für `budget_tokens` sollten möglichst bereits im MCP-Eingabeschema sichtbar sein; die serverseitige Validierung bleibt bestehen.

### 2. Einen Modus für Bestandsaufnahmen vorsehen

Der öffentliche Einstieg kann `task_context` bleiben. Intern sollte eine Bestandsaufnahme von einer konkreten Änderung an einem Produktionspfad unterschieden werden. Ein optionaler expliziter Modus ist eine mögliche API-Entscheidung, keine festgelegte Umsetzung.

Für Bestandsaufnahmen sind mehrere zusammengehörige Einstiegspunkte zulässig. Die Regel, dass genau ein zuverlässiger Produktions-Einstiegspunkt existieren muss, darf diesen Modus nicht automatisch als unzuverlässig verwerfen. Präzise Pfade, Quellenbereiche und Budgetgrenzen bleiben erforderlich.

### 3. Test- und Tooling-Zusammenhänge abbilden

Relevante Beziehungen sind unter anderem:

- Komponente -> Story -> Fixture/Provider -> Interaktionsprüfung.
- Storybook-Konfiguration -> aktiviertes Addon -> tatsächlich wirksamer Testmodus.
- Package-Skript -> Runner-Konfiguration -> Testauswahl -> CI-Job.
- Visueller Testfall -> Referenzbild -> Ausführungsumgebung -> CI-Anbindung.
- Dokumentierter Ist-Stand -> Implementierungsbeleg -> bewusst zurückgestellte Arbeit.

Eine Abhängigkeit in `package.json` beweist weder ihre Aktivierung noch ihre Ausführung in CI. Ein konfigurierter CI-Job beweist keinen erfolgreichen Pipeline-Lauf.

### 4. Abdeckung und negative Aussagen absichern

Der Context Pack sollte erkennen lassen, welche Teilbereiche vollständig geprüft wurden und welche offen bleiben. Ein Vorschlag für solche Teilbereiche ist Konfiguration, Stories, Fixtures, Test-Runner, CI und Betriebsdokumentation.

Aussagen wie "kein visueller CI-Job vorhanden" benötigen vollständige Abdeckung der relevanten CI-Dateien einschließlich lokaler oder externer Includes. Nicht gelieferte oder unzugängliche Includes müssen als unbekannt markiert werden. `source_coverage: complete` darf keine repository-weite Vollständigkeit suggerieren, wenn nur ein Teilpfad erfasst wurde.

Bei Budgetüberschreitung sind konkrete `source_omissions` mit Projekt, Pfad und begrenztem Zeilenbereich erforderlich. Ein Fallback darf nicht zugleich angefordert und durch fehlende Lesegrenzen praktisch unmöglich gemacht werden. Alternativ braucht es einen ausdrücklich geregelten, begrenzten Fallback-Vertrag; die agentenseitigen Arbeitsregeln müssen dazu passen.

### 5. CI-Analysen als weiteren Referenzfall aufnehmen

Die anschließende Playwright-Frage verbindet mehrere Repositories: Frontend-Änderungen und Änderungen an konsumierten Backend-Services sollen dieselben betroffenen App-Suiten auslösen. Ein nur von App A genutzter Service startet deren Suite; ein von App A und App B genutzter Service startet beide App-Suiten. Es werden keine eigenen Playwright-Suiten pro Backend-Service gewünscht. Die Läufe sollen nach Deployments in Integrations- und Testumgebungen automatisch starten und Fehler sichtbar melden, die Auslieferung jedoch ausdrücklich nicht blockieren. Die zuvor formulierte Forderung nach einem blockierenden Gate war eine Fehlinterpretation und ist verworfen.

Dafür sind Trigger, übergebene Parameter, Service-zu-App-Mehrfachzuordnungen, getestete Versionen, Testumgebungen, Build-/Deploy-Reihenfolge und die bewusst nicht blockierende Fehlerbehandlung relevant. Die Release-Prüfung muss insbesondere unterscheiden zwischen dem ausgewählten Frontend-Zielbranch und dem gegebenenfalls abweichenden Branch und Zielsystem des separaten Playwright-Projekts. Eine reine Datei- oder Symboltrefferliste kann diese Zusammenhänge nicht belegen. Backend-/Frontend-Zuordnungen dürfen nicht aus Namensähnlichkeit erfunden werden.

## Akzeptanzkriterien und Regressionstests

- Die ursprüngliche mehrsätzige Storybook-Anfrage liefert die belegten Konfigurations-, Runner-, CI- und Dokumentationsquellen oder konkrete begrenzte Auslassungen.
- Die Query bleibt über CLI-/MCP-Grenzen inhaltlich vollständig; deutsche Umlaute und Satzzeichen verlieren keine Suchanforderungen.
- Ein installiertes A11y-Addon mit `test: off` wird nicht als aktive automatische Prüfung ausgewiesen.
- Vorbereitete Screenshot-Tests ohne Referenzbilder werden nicht als fertige visuelle Absicherung gemeldet.
- Ein CI-Trigger mit `allow_failure: true` wird korrekt als nicht blockierende Prüfung ausgewiesen. Die gewünschte Fehlerbehandlung wird aus der Benutzeranforderung übernommen; ein blockierendes Gate wird nicht eigenständig als Ziel hinzugefügt.
- Unbekannte externe Includes, Infrastrukturwerte und Pipeline-Ergebnisse bleiben explizit unbekannt.
- Eine Bestandsaufnahme über mehrere Einstiegspunkte ist zulässig, ohne die strengeren Regeln für konkrete Produktionsänderungen abzuschwächen.
- Ein Fallback mit fehlenden Quellen bietet entweder konkrete freigegebene Lesebereiche oder einen klar definierten Eskalationsweg.
- Ein Index-Refresh wird nur bei belegter Freshness-/Index-Problematik als notwendiger Schritt behauptet.

## Empfohlene Reihenfolge

1. Query-Transport und MCP-Schema prüfen und mit reproduzierbaren Tests absichern.
2. Den beschriebenen Storybook-Fall als Erwartungsdatensatz anlegen.
3. Bestandsaufnahme-Modus und passende Coverage-/Fallback-Semantik spezifizieren.
4. Fehlende Beziehungen für Konfiguration, Tests, CI und Dokumentation ergänzen.
5. Den mehrprojektigen Playwright-/Backend-Trigger-Fall als zweiten Regressionstest aufnehmen.
6. Kontextqualität, Vollständigkeit und Tokenverbrauch gegen den bisherigen Workflow vergleichen.

Zum Zeitpunkt dieses ursprünglichen Vorschlags wurden weder GoreGraph-Code noch globale Agent-Regeln oder CI-Pipelines geändert. Für den heutigen Umfang und verbleibende Grenzen ist die verlinkte Tooling-Audit-Dokumentation maßgeblich.
