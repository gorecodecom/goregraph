package agent

type Request struct {
	Root         string `json:"root,omitempty"`
	Task         string `json:"task"`
	Query        string `json:"query,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Format       string `json:"format,omitempty"`
	Detail       string `json:"detail,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Continuation string `json:"continuation,omitempty"`
}

type Item struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary,omitempty"`
	Project     string         `json:"project,omitempty"`
	File        string         `json:"file,omitempty"`
	Line        int            `json:"line,omitempty"`
	Confidence  string         `json:"confidence,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	EvidenceIDs []string       `json:"evidence_ids,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

type TaskContextRecord struct {
	Target           string   `json:"target"`
	Services         []string `json:"services,omitempty"`
	Endpoints        []Item   `json:"endpoints,omitempty"`
	Files            []string `json:"files,omitempty"`
	Tests            []Item   `json:"tests,omitempty"`
	Risks            []Item   `json:"risks,omitempty"`
	EvidenceIDs      []string `json:"evidence_ids,omitempty"`
	Freshness        string   `json:"freshness"`
	CoverageWarnings []string `json:"coverage_warnings,omitempty"`
	SuggestedNext    string   `json:"suggested_next,omitempty"`
}

type Result struct {
	Schema           int      `json:"schema"`
	Task             string   `json:"task"`
	Freshness        string   `json:"freshness"`
	CoverageWarnings []string `json:"coverage_warnings,omitempty"`
	Items            []Item   `json:"items"`
	Count            int      `json:"count"`
	Truncated        bool     `json:"truncated"`
	Continuation     string   `json:"continuation,omitempty"`
	SuggestedNext    string   `json:"suggested_next,omitempty"`
}
