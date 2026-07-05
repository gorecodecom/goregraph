package scan

type Result struct {
	ScannedFiles int
	SkippedFiles int
	OutputDir    string
}

type Index struct {
	Files       []FileRecord
	Symbols     []SymbolRecord
	Relations   []RelationRecord
	JavaSources []JavaSourceRecord
	Workspace   WorkspaceIndex
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
	Tool        string       `json:"tool"`
	Schema      int          `json:"schema"`
	OutputDir   string       `json:"output_dir"`
	Files       int          `json:"files"`
	Skipped     int          `json:"skipped"`
	Generated   []string     `json:"generated"`
	ProjectRoot string       `json:"project_root,omitempty"`
	GeneratedAt string       `json:"generated_at,omitempty"`
	Git         *GitMetadata `json:"git,omitempty"`
}

type GitMetadata struct {
	Commit string `json:"commit,omitempty"`
	Branch string `json:"branch,omitempty"`
	Dirty  *bool  `json:"dirty,omitempty"`
}

type AuditRecord struct {
	Tool             string   `json:"tool"`
	Version          string   `json:"version"`
	Command          string   `json:"command"`
	ProjectRoot      string   `json:"project_root"`
	OutputDir        string   `json:"output_dir"`
	StartedAt        string   `json:"started_at"`
	FinishedAt       string   `json:"finished_at"`
	FilesRead        int      `json:"files_read"`
	FilesSkipped     int      `json:"files_skipped"`
	Generated        []string `json:"generated"`
	NetworkUsed      bool     `json:"network_used"`
	ExternalCommands bool     `json:"external_commands"`
	Warnings         []string `json:"warnings,omitempty"`
}

type RichSymbolRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Language       string `json:"language"`
	File           string `json:"file"`
	Line           int    `json:"line,omitempty"`
	Owner          string `json:"owner,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
}

type RichRelationRecord struct {
	ID              string  `json:"id"`
	From            string  `json:"from"`
	To              string  `json:"to"`
	Type            string  `json:"type"`
	Language        string  `json:"language,omitempty"`
	Line            int     `json:"line,omitempty"`
	SourceLocation  string  `json:"source_location,omitempty"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Internal        bool    `json:"internal,omitempty"`
}

type RichGraph struct {
	Directed bool            `json:"directed"`
	Nodes    []RichGraphNode `json:"nodes"`
	Edges    []RichGraphEdge `json:"edges"`
}

type RichGraphNode struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Kind           string `json:"kind"`
	Language       string `json:"language,omitempty"`
	SourceFile     string `json:"source_file,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
}

type RichGraphEdge struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Relation        string  `json:"relation"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	SourceFile      string  `json:"source_file,omitempty"`
	SourceLocation  string  `json:"source_location,omitempty"`
}

type JavaSourceRecord struct {
	File        string                 `json:"file"`
	Package     string                 `json:"package,omitempty"`
	Imports     []JavaImportRecord     `json:"imports,omitempty"`
	Types       []JavaTypeRecord       `json:"types,omitempty"`
	Methods     []JavaMethodRecord     `json:"methods,omitempty"`
	Fields      []JavaFieldRecord      `json:"fields,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
	Constants   map[string]string      `json:"constants,omitempty"`
}

type JavaImportRecord struct {
	Name     string `json:"name"`
	Static   bool   `json:"static"`
	Line     int    `json:"line"`
	Internal bool   `json:"internal"`
	File     string `json:"file,omitempty"`
}

type JavaTypeRecord struct {
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"`
	Package     string                 `json:"package,omitempty"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Extends     string                 `json:"extends,omitempty"`
	Implements  []string               `json:"implements,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaMethodRecord struct {
	Name        string                 `json:"name"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Owner       string                 `json:"owner,omitempty"`
	Visibility  string                 `json:"visibility,omitempty"`
	ReturnType  string                 `json:"return_type,omitempty"`
	Parameters  []JavaParameterRecord  `json:"parameters,omitempty"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
	Calls       []JavaCallRecord       `json:"calls,omitempty"`
}

type JavaFieldRecord struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	File        string                 `json:"file"`
	Line        int                    `json:"line"`
	Owner       string                 `json:"owner,omitempty"`
	Final       bool                   `json:"final"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaParameterRecord struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Annotations []JavaAnnotationRecord `json:"annotations,omitempty"`
}

type JavaAnnotationRecord struct {
	Name       string            `json:"name"`
	Arguments  string            `json:"arguments,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Line       int               `json:"line"`
}

type JavaCallRecord struct {
	Receiver string `json:"receiver,omitempty"`
	Method   string `json:"method"`
	Line     int    `json:"line"`
}

type SpringIndex struct {
	Applications []SpringApplicationRecord `json:"applications,omitempty"`
	Components   []SpringComponentRecord   `json:"components,omitempty"`
	Endpoints    []SpringEndpointRecord    `json:"endpoints,omitempty"`
	Dependencies []SpringDependencyRecord  `json:"dependencies,omitempty"`
	Repositories []SpringRepositoryRecord  `json:"repositories,omitempty"`
	Entities     []SpringEntityRecord      `json:"entities,omitempty"`
	Beans        []SpringBeanRecord        `json:"beans,omitempty"`
}

type SpringApplicationRecord struct {
	Name             string `json:"name"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	ScanBasePackages string `json:"scan_base_packages,omitempty"`
}

type SpringComponentRecord struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Package     string   `json:"package,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
}

type SpringEndpointRecord struct {
	HTTPMethod  string                `json:"http_method"`
	Path        string                `json:"path"`
	Controller  string                `json:"controller"`
	Method      string                `json:"method"`
	File        string                `json:"file"`
	Line        int                   `json:"line"`
	RequestType string                `json:"request_type,omitempty"`
	ReturnType  string                `json:"return_type,omitempty"`
	Parameters  []JavaParameterRecord `json:"parameters,omitempty"`
}

type SpringDependencyRecord struct {
	From      string `json:"from"`
	To        string `json:"to"`
	FromFile  string `json:"from_file"`
	ToFile    string `json:"to_file,omitempty"`
	Field     string `json:"field,omitempty"`
	Injection string `json:"injection"`
	Line      int    `json:"line"`
}

type SpringRepositoryRecord struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Entity     string `json:"entity,omitempty"`
	EntityFile string `json:"entity_file,omitempty"`
	IDType     string `json:"id_type,omitempty"`
}

type SpringEntityRecord struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Table   string `json:"table,omitempty"`
	Package string `json:"package,omitempty"`
}

type SpringBeanRecord struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Config     string `json:"config,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	MethodName string `json:"method_name,omitempty"`
}

type WorkspaceIndex struct {
	MavenPackages []MavenPackageRecord `json:"maven_packages,omitempty"`
	NodePackages  []NodePackageRecord  `json:"node_packages,omitempty"`
}

type MavenPackageRecord struct {
	Path       string `json:"path"`
	GroupID    string `json:"group_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
	Version    string `json:"version,omitempty"`
	Parent     string `json:"parent,omitempty"`
}

type NodePackageRecord struct {
	Path           string   `json:"path"`
	Name           string   `json:"name,omitempty"`
	Version        string   `json:"version,omitempty"`
	Private        bool     `json:"private"`
	PackageManager string   `json:"package_manager,omitempty"`
	Workspaces     []string `json:"workspaces,omitempty"`
	Scripts        []string `json:"scripts,omitempty"`
}
