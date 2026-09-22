# Vorschlag: Dateien und Projekte aus dem Dashboard in der IDE öffnen

Status: Idee, noch nicht implementiert
Datum: 2026-09-22

## Nutzen

Von einer Fundstelle im GoreGraph-Dashboard direkt zur betreffenden Datei und Zeile im lokalen Editor wechseln. So entfällt das manuelle Suchen nach dem Projekt und der Funktion. Zusätzlich könnte sich das gesamte Projekt in der IDE öffnen lassen.

## Einbindung ins Dashboard

- Einen kleinen Button „Im Editor öffnen“ an vorhandenen Datei- und Zeilenangaben anbieten, beispielsweise im Service-Code, an Schnittstellen-Nachweisen und in Tests & Tooling.
- „Projekt öffnen“ im Service-Kontext anbieten, wenn ein lokaler Projektpfad bekannt ist.
- Die bestehenden Ansichten und Verbindungen beibehalten; die Navigation lediglich ergänzen.
- Bei fehlender Editor-Anbindung weiterhin Pfad und Zeile anzeigen und kopierbar machen.

## Benötigte Informationen

- Lokaler Workspace- beziehungsweise Projektpfad.
- Relativer Pfad der Quelldatei und, soweit vorhanden, Zeile und optional Spalte.
- Bevorzugter Editor beziehungsweise IDE und dessen unterstützte Öffnungsmethode.

Ein relativer Quellpfad allein reicht nicht zuverlässig aus: Ein exportiertes Dashboard kann auf einem anderen Rechner liegen, und Projekte können dort andere Verzeichnisse haben. Eine lokale Zuordnung des Workspace-Pfads sollte deshalb möglich sein. Für den Sprung zur Funktion kann zunächst ihre bekannte Deklarationszeile verwendet werden.

## Mögliche technische Umsetzung

Als Optionen kommen eine von der IDE unterstützte URL-/Protokoll-Anbindung oder eine lokale GoreGraph-Anbindung infrage, die eine ausdrücklich ausgewählte IDE öffnet. Die konkret unterstützten Mechanismen für beispielsweise VS Code und IntelliJ müssen vor der Umsetzung geprüft werden.

Das Öffnen erfolgt ausschließlich durch einen bewussten Klick. Eine lokale Anbindung sollte nur geprüfte Dateien innerhalb der zugeordneten Projekte öffnen und keine beliebigen Shell-Befehle aus Dashboard-Daten ausführen. Eine reine HTML-Datei kann lokale Programme nicht voraussetzungslos starten.

## Vor der Umsetzung entscheiden

- Welche IDEs zuerst unterstützt werden sollen.
- Ob die Einstellung pro Rechner oder pro Workspace gilt.
- Wie geteilte Exporte und abweichende lokale Projektpfade zugeordnet werden.
- Wie fehlende Dateien, unbekannte Zeilen und eine nicht eingerichtete IDE verständlich angezeigt werden.

## Abgrenzung

Dies ist eine ergänzende Bedienfunktion für das Dashboard. Die Idee benötigt keine Änderung der fachlichen Index- oder Analyseergebnisse und soll den bestehenden KI-Kontext- und Tokenersparnis-Workflow unverändert lassen. Die Öffnungsfunktion ist nicht Teil der aktuell umgesetzten Tests-&-Tooling-Unteransicht.
