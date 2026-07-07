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
	Code        CodeIntelligenceRecord
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
	Type            string  `json:"type"`
	Relation        string  `json:"relation"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	SourceFile      string  `json:"source_file,omitempty"`
	SourceLocation  string  `json:"source_location,omitempty"`
}

type MethodRefRecord struct {
	Owner  string `json:"owner"`
	Method string `json:"method"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type CallGraphRecord struct {
	Edges []CallGraphEdgeRecord `json:"edges"`
}

type CallGraphEdgeRecord struct {
	ID              string          `json:"id"`
	From            MethodRefRecord `json:"from"`
	To              MethodRefRecord `json:"to"`
	Type            string          `json:"type"`
	Line            int             `json:"line,omitempty"`
	SourceFile      string          `json:"source_file,omitempty"`
	Confidence      string          `json:"confidence"`
	ConfidenceScore float64         `json:"confidence_score"`
	Reason          string          `json:"reason,omitempty"`
}

type SpringEndpointFlowRecord struct {
	HTTPMethod string                   `json:"http_method"`
	Path       string                   `json:"path"`
	Controller string                   `json:"controller"`
	Method     string                   `json:"method"`
	File       string                   `json:"file"`
	Line       int                      `json:"line"`
	Steps      []SpringEndpointFlowStep `json:"steps"`
}

type SpringEndpointFlowStep struct {
	Owner      string `json:"owner"`
	Method     string `json:"method"`
	Kind       string `json:"kind,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Confidence string `json:"confidence"`
}

type TestMapRecord struct {
	TestFile        string  `json:"test_file"`
	TestClass       string  `json:"test_class,omitempty"`
	TestMethod      string  `json:"test_method,omitempty"`
	TargetFile      string  `json:"target_file,omitempty"`
	TargetClass     string  `json:"target_class,omitempty"`
	TargetMethod    string  `json:"target_method,omitempty"`
	HTTPMethod      string  `json:"http_method,omitempty"`
	Path            string  `json:"path,omitempty"`
	Type            string  `json:"type"`
	Line            int     `json:"line,omitempty"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Reason          string  `json:"reason,omitempty"`
}

type AnalyzerRecord struct {
	Language  string   `json:"language"`
	Scope     string   `json:"scope"`
	Symbols   bool     `json:"symbols"`
	Relations bool     `json:"relations"`
	Calls     bool     `json:"calls"`
	Endpoints bool     `json:"endpoints"`
	Tests     bool     `json:"tests"`
	Workspace bool     `json:"workspace"`
	Outputs   []string `json:"outputs,omitempty"`
}

type CodeIntelligenceRecord struct {
	Functions    []CodeFunctionRecord `json:"functions,omitempty"`
	Routes       []CodeRouteRecord    `json:"routes,omitempty"`
	APIContracts []APIContractRecord  `json:"api_contracts,omitempty"`
}

type CodeFunctionRecord struct {
	Name     string           `json:"name"`
	Owner    string           `json:"owner,omitempty"`
	Kind     string           `json:"kind"`
	Language string           `json:"language"`
	File     string           `json:"file"`
	Line     int              `json:"line"`
	EndLine  int              `json:"end_line,omitempty"`
	Calls    []CodeCallRecord `json:"calls,omitempty"`
}

type CodeCallRecord struct {
	Receiver string `json:"receiver,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Method   string `json:"method"`
	Kind     string `json:"kind,omitempty"`
	Raw      string `json:"raw,omitempty"`
	Line     int    `json:"line"`
}

type CodeRouteRecord struct {
	Language           string   `json:"language"`
	Framework          string   `json:"framework"`
	Kind               string   `json:"kind"`
	App                string   `json:"app,omitempty"`
	Package            string   `json:"package,omitempty"`
	RouteID            string   `json:"route_id,omitempty"`
	HTTPMethod         string   `json:"http_method"`
	Path               string   `json:"path"`
	Handler            string   `json:"handler,omitempty"`
	RenderedComponents []string `json:"rendered_components,omitempty"`
	File               string   `json:"file"`
	Line               int      `json:"line"`
	Confidence         string   `json:"confidence"`
	ConfidenceScore    float64  `json:"confidence_score,omitempty"`
	Reason             string   `json:"reason,omitempty"`
}

type CodeFlowRecord struct {
	Language   string         `json:"language"`
	Framework  string         `json:"framework"`
	Kind       string         `json:"kind"`
	App        string         `json:"app,omitempty"`
	Package    string         `json:"package,omitempty"`
	RouteID    string         `json:"route_id,omitempty"`
	HTTPMethod string         `json:"http_method"`
	Path       string         `json:"path"`
	Handler    string         `json:"handler,omitempty"`
	File       string         `json:"file"`
	Line       int            `json:"line"`
	Steps      []CodeFlowStep `json:"steps"`
}

type CodeFlowStep struct {
	Name       string `json:"name"`
	Owner      string `json:"owner,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Language   string `json:"language,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason,omitempty"`
}

type APIContractRecord struct {
	Language         string             `json:"language"`
	App              string             `json:"app,omitempty"`
	Package          string             `json:"package,omitempty"`
	HTTPMethod       string             `json:"http_method"`
	Path             string             `json:"path"`
	RawPath          string             `json:"raw_path,omitempty"`
	Query            string             `json:"query,omitempty"`
	QueryParams      []QueryParamRecord `json:"query_params,omitempty"`
	ServiceCandidate string             `json:"service_candidate,omitempty"`
	UnsafeDynamic    bool               `json:"unsafe_dynamic,omitempty"`
	Caller           string             `json:"caller,omitempty"`
	File             string             `json:"file"`
	Line             int                `json:"line"`
	Confidence       string             `json:"confidence"`
	ConfidenceScore  float64            `json:"confidence_score"`
	Reason           string             `json:"reason,omitempty"`
}

type QueryParamRecord struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type ContractMatchRecord struct {
	APIHTTPMethod     string  `json:"api_http_method"`
	APIPath           string  `json:"api_path"`
	APIRawPath        string  `json:"api_raw_path,omitempty"`
	APIFile           string  `json:"api_file"`
	APILine           int     `json:"api_line,omitempty"`
	APIApp            string  `json:"api_app,omitempty"`
	BackendHTTPMethod string  `json:"backend_http_method,omitempty"`
	BackendPath       string  `json:"backend_path,omitempty"`
	BackendHandler    string  `json:"backend_handler,omitempty"`
	BackendFile       string  `json:"backend_file,omitempty"`
	BackendLine       int     `json:"backend_line,omitempty"`
	ServiceCandidate  string  `json:"service_candidate,omitempty"`
	Issue             string  `json:"issue,omitempty"`
	Confidence        string  `json:"confidence"`
	ConfidenceScore   float64 `json:"confidence_score"`
	Reason            string  `json:"reason,omitempty"`
}

type DiagnosticsRecord struct {
	Entrypoints                []DiagnosticRouteRecord        `json:"entrypoints"`
	RiskyContracts             []ContractMatchRecord          `json:"risky_contracts"`
	WorkspaceResolvedContracts []WorkspaceContractMatchRecord `json:"workspace_resolved_contracts,omitempty"`
	UnscannedServices          []DiagnosticServiceRecord      `json:"unscanned_services"`
	EndpointsWithoutTests      []SpringEndpointRecord         `json:"endpoints_without_tests"`
	WeakFlows                  []DiagnosticFlowRecord         `json:"weak_flows"`
	LikelyTests                []TestMapRecord                `json:"likely_tests"`
}

type DiagnosticRouteRecord struct {
	HTTPMethod string `json:"http_method"`
	Path       string `json:"path"`
	RouteID    string `json:"route_id,omitempty"`
	Handler    string `json:"handler,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type DiagnosticServiceRecord struct {
	Service   string `json:"service"`
	Contracts int    `json:"contracts"`
	Reason    string `json:"reason"`
}

type DiagnosticFlowRecord struct {
	HTTPMethod string `json:"http_method"`
	Path       string `json:"path"`
	RouteID    string `json:"route_id,omitempty"`
	Handler    string `json:"handler,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line,omitempty"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

type WorkspaceRegistryRecord struct {
	Root      string                   `json:"root"`
	Current   string                   `json:"current,omitempty"`
	Generated string                   `json:"generated_at,omitempty"`
	Projects  []WorkspaceProjectRecord `json:"projects"`
}

type WorkspaceProjectRecord struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	AbsPath   string `json:"abs_path,omitempty"`
	Kind      string `json:"kind"`
	Service   string `json:"service,omitempty"`
	Indexed   bool   `json:"indexed"`
	Status    string `json:"status"`
	OutputDir string `json:"output_dir,omitempty"`
}

type WorkspaceContextRecord struct {
	Root            string                   `json:"root"`
	Current         string                   `json:"current,omitempty"`
	LoadedIndexes   []WorkspaceProjectRecord `json:"loaded_indexes"`
	Projects        []WorkspaceProjectRecord `json:"projects"`
	KnownServices   []string                 `json:"known_services,omitempty"`
	MissingServices []string                 `json:"missing_services,omitempty"`
}

type WorkspaceContractMatchRecord struct {
	APIProject        string  `json:"api_project"`
	APIHTTPMethod     string  `json:"api_http_method"`
	APIPath           string  `json:"api_path"`
	APIFile           string  `json:"api_file"`
	APILine           int     `json:"api_line,omitempty"`
	APICaller         string  `json:"api_caller,omitempty"`
	BackendProject    string  `json:"backend_project,omitempty"`
	BackendService    string  `json:"backend_service,omitempty"`
	BackendHTTPMethod string  `json:"backend_http_method,omitempty"`
	BackendPath       string  `json:"backend_path,omitempty"`
	BackendHandler    string  `json:"backend_handler,omitempty"`
	BackendFile       string  `json:"backend_file,omitempty"`
	BackendLine       int     `json:"backend_line,omitempty"`
	ServiceCandidate  string  `json:"service_candidate,omitempty"`
	Issue             string  `json:"issue"`
	Confidence        string  `json:"confidence"`
	ConfidenceScore   float64 `json:"confidence_score"`
	Reason            string  `json:"reason,omitempty"`
}

type WorkspaceFeatureFlowRecord struct {
	FrontendProject    string                   `json:"frontend_project"`
	FrontendRouteID    string                   `json:"frontend_route_id,omitempty"`
	FrontendRoutePath  string                   `json:"frontend_route_path,omitempty"`
	FrontendRouteFile  string                   `json:"frontend_route_file,omitempty"`
	FrontendRouteLine  int                      `json:"frontend_route_line,omitempty"`
	FrontendComponent  string                   `json:"frontend_component,omitempty"`
	FrontendCaller     string                   `json:"frontend_caller,omitempty"`
	FrontendSteps      []CodeFlowStep           `json:"frontend_steps,omitempty"`
	FrontendConfidence string                   `json:"frontend_confidence,omitempty"`
	FrontendReason     string                   `json:"frontend_reason,omitempty"`
	FrontendFile       string                   `json:"frontend_file"`
	FrontendLine       int                      `json:"frontend_line,omitempty"`
	HTTPMethod         string                   `json:"http_method"`
	Path               string                   `json:"path"`
	BackendProject     string                   `json:"backend_project"`
	BackendService     string                   `json:"backend_service,omitempty"`
	BackendController  string                   `json:"backend_controller,omitempty"`
	BackendMethod      string                   `json:"backend_method,omitempty"`
	BackendFile        string                   `json:"backend_file,omitempty"`
	BackendLine        int                      `json:"backend_line,omitempty"`
	BackendSteps       []SpringEndpointFlowStep `json:"backend_steps,omitempty"`
	Tests              []TestMapRecord          `json:"tests,omitempty"`
	TestReason         string                   `json:"test_reason,omitempty"`
	Confidence         string                   `json:"confidence"`
	Reason             string                   `json:"reason,omitempty"`
}

type PackageGraphRecord struct {
	Nodes []PackageNodeRecord `json:"nodes"`
	Edges []PackageEdgeRecord `json:"edges"`
}

type PackageNodeRecord struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Kind           string   `json:"kind"`
	PackageManager string   `json:"package_manager,omitempty"`
	Scripts        []string `json:"scripts,omitempty"`
}

type PackageEdgeRecord struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	Type            string  `json:"type"`
	FromPath        string  `json:"from_path,omitempty"`
	ToPath          string  `json:"to_path,omitempty"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Reason          string  `json:"reason,omitempty"`
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
	Name         string                 `json:"name"`
	File         string                 `json:"file"`
	Line         int                    `json:"line"`
	Owner        string                 `json:"owner,omitempty"`
	Visibility   string                 `json:"visibility,omitempty"`
	ReturnType   string                 `json:"return_type,omitempty"`
	Parameters   []JavaParameterRecord  `json:"parameters,omitempty"`
	Annotations  []JavaAnnotationRecord `json:"annotations,omitempty"`
	Calls        []JavaCallRecord       `json:"calls,omitempty"`
	HTTPRequests []JavaHTTPCallRecord   `json:"http_requests,omitempty"`
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
	Receiver    string `json:"receiver,omitempty"`
	TargetOwner string `json:"target_owner,omitempty"`
	Method      string `json:"method"`
	Line        int    `json:"line"`
}

type JavaHTTPCallRecord struct {
	HTTPMethod string `json:"http_method"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
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
	RequestKind string                `json:"request_kind,omitempty"`
	Consumes    string                `json:"consumes,omitempty"`
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
	Path         string                  `json:"path"`
	GroupID      string                  `json:"group_id,omitempty"`
	ArtifactID   string                  `json:"artifact_id,omitempty"`
	Version      string                  `json:"version,omitempty"`
	Parent       string                  `json:"parent,omitempty"`
	Dependencies []MavenDependencyRecord `json:"dependencies,omitempty"`
}

type MavenDependencyRecord struct {
	GroupID    string `json:"group_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
	Version    string `json:"version,omitempty"`
	Scope      string `json:"scope,omitempty"`
}

type MavenGraphRecord struct {
	Nodes []MavenNodeRecord `json:"nodes"`
	Edges []MavenEdgeRecord `json:"edges"`
}

type MavenNodeRecord struct {
	ID       string `json:"id"`
	GroupID  string `json:"group_id,omitempty"`
	Artifact string `json:"artifact_id,omitempty"`
	Version  string `json:"version,omitempty"`
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Parent   string `json:"parent,omitempty"`
}

type MavenEdgeRecord struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	Type            string  `json:"type"`
	Scope           string  `json:"scope,omitempty"`
	FromPath        string  `json:"from_path,omitempty"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Reason          string  `json:"reason,omitempty"`
}

type NodePackageRecord struct {
	Path           string   `json:"path"`
	Name           string   `json:"name,omitempty"`
	Version        string   `json:"version,omitempty"`
	Private        bool     `json:"private"`
	PackageManager string   `json:"package_manager,omitempty"`
	Workspaces     []string `json:"workspaces,omitempty"`
	Scripts        []string `json:"scripts,omitempty"`
	Dependencies   []string `json:"dependencies,omitempty"`
}
