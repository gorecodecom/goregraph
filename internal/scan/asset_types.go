package scan

// AssetIndexRecord records source-backed asset objects and their static references.
// It contains no claim about rendered geometry or runtime behavior.
type AssetIndexRecord struct {
	SchemaVersion int                     `json:"schema_version"`
	Nodes         []AssetNodeRecord       `json:"nodes"`
	References    []AssetReferenceRecord  `json:"references"`
	Diagnostics   []AssetDiagnosticRecord `json:"diagnostics"`
}
type AssetNodeRecord struct {
	ID         string         `json:"id"`
	File       string         `json:"file"`
	Line       int            `json:"line"`
	Language   string         `json:"language"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	LocalID    string         `json:"local_id,omitempty"`
	GUID       string         `json:"guid,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}
type AssetReferenceRecord struct {
	From       string           `json:"from"`
	To         string           `json:"to,omitempty"`
	File       string           `json:"file"`
	Line       int              `json:"line"`
	Property   string           `json:"property"`
	GUID       string           `json:"guid,omitempty"`
	LocalID    string           `json:"local_id,omitempty"`
	Resolution SymbolResolution `json:"resolution"`
	Candidates []string         `json:"candidates,omitempty"`
	Reason     string           `json:"reason"`
}
type AssetDiagnosticRecord struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func canonicalAssetDiagnostics(assets AssetIndexRecord) []CanonicalDiagnosticRecord {
	var records []CanonicalDiagnosticRecord
	for _, item := range assets.Diagnostics {
		title, explanation, check := "Asset-Daten unvollständig", item.Message, "Prüfe die angegebene Quelldatei."
		severity := SeverityWarning
		switch item.Code {
		case "asset_export_stale":
			title = "Asset-Export veraltet"
			explanation = "Die Quelldatei fehlt im Index oder ihre SHA-256 stimmt nicht mit dem Export überein. Seine Objektdaten werden nicht als aktueller Kontext übernommen."
			check = "Erzeuge den Export ausdrücklich erneut und lass den aktivierten Watcher aktualisieren."
		case "blender_export_required":
			title = "Blender-Export erforderlich"
			explanation = "Die binäre Blender-Datei ist erfasst. Objekt-, Rigging- und Geometriedaten benötigen einen aktuellen, ausdrücklich erzeugten Export."
			check = "Exportiere die benötigten Blender-Daten mit dem bereitgestellten Exporter."
			severity = SeverityInfo
		case "unity_duplicate_guid":
			title = "Unity-GUID mehrfach vergeben"
			explanation = "Mehrere erfasste Assets verwenden dieselbe GUID. Beziehungen bleiben deshalb mehrdeutig."
			check = "Prüfe die betroffenen .meta-Dateien in Unity."
		case "unity_missing_local_reference":
			title = "Lokale Unity-Referenz fehlt"
			explanation = "Eine fileID verweist auf kein serialisiertes Objekt in derselben Asset-Datei."
			check = "Prüfe die referenzierte Komponente oder das Prefab in Unity."
		case "unity_invalid_asset", "unity_invalid_metadata":
			title = "Unity-Datei nicht auswertbar"
			explanation = "Die Datei enthält fehlerhafte oder nicht unterstützte serialisierte YAML-Daten."
			check = "Prüfe das Serialisierungsformat und die Datei."
		case "asset_invalid_export", "asset_export_duplicate_id", "asset_export_limit", "asset_export_outside_project":
			title = "Asset-Export nicht verwendbar"
			explanation = "Der Export erfüllt die Anforderungen an Format, eindeutige Objektidentitäten, Größenlimits oder Projektzuordnung nicht."
			check = "Erzeuge den Export mit der aktuellen Vorlage innerhalb des zugehörigen Projekts."
		}
		records = append(records, CanonicalDiagnosticRecord{ID: stableID("asset-diagnostic", item.File, item.Code), Code: item.Code, Title: title, Category: "asset_analysis", Severity: severity, Confidence: ConfidenceExact, Resolution: ResolutionUnresolved, Explanation: explanation, PossibleImpact: "Die angegebenen Asset-Daten stehen der Analyse nicht vollständig zur Verfügung.", AffectedArtifacts: []string{item.File}, NextChecks: []string{check}})
	}
	return records
}
