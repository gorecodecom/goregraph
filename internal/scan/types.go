package scan

type Result struct {
	ScannedFiles int
	SkippedFiles int
	OutputDir    string
}

type Index struct {
	Files     []FileRecord
	Symbols   []SymbolRecord
	Relations []RelationRecord
}

type FileRecord struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
	Kind     string `json:"kind"`
}

type SymbolRecord struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type RelationRecord struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type Manifest struct {
	Tool        string   `json:"tool"`
	Schema      int      `json:"schema"`
	OutputDir   string   `json:"output_dir"`
	Files       int      `json:"files"`
	Skipped     int      `json:"skipped"`
	Generated   []string `json:"generated"`
	ProjectRoot string   `json:"project_root,omitempty"`
}
