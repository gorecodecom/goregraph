package scan

import (
	"fmt"
	"path/filepath"
)

type BuildTarget string

const (
	BuildTargetAgent     BuildTarget = "agent"
	BuildTargetDashboard BuildTarget = "dashboard"
	BuildTargetAll       BuildTarget = "all"
)

type OutputLayout struct {
	Root     string
	Manifest string
}

func NewProjectOutputLayout(root string) OutputLayout {
	return newOutputLayout(root)
}

func NewWorkspaceOutputLayout(root string) OutputLayout {
	return newOutputLayout(root)
}

func newOutputLayout(root string) OutputLayout {
	return OutputLayout{
		Root:     root,
		Manifest: filepath.Join(root, "manifest.json"),
	}
}

func (l OutputLayout) Index(name string) string {
	return filepath.Join(l.Root, "index", name)
}

func (l OutputLayout) Agent(name string) string {
	return filepath.Join(l.Root, "agent", name)
}

func (l OutputLayout) Dashboard(name string) string {
	return filepath.Join(l.Root, "dashboard", name)
}

func ParseBuildTarget(value string) (BuildTarget, error) {
	target := BuildTarget(value)
	if err := target.Validate(); err != nil {
		return "", err
	}
	return target, nil
}

func (t BuildTarget) Validate() error {
	switch t {
	case BuildTargetAgent, BuildTargetDashboard, BuildTargetAll:
		return nil
	default:
		return fmt.Errorf("unknown build target %q; accepted values: agent, dashboard, all", t)
	}
}

func (t BuildTarget) IncludesAgent() bool {
	return t == BuildTargetAgent || t == BuildTargetAll
}

func (t BuildTarget) IncludesDashboard() bool {
	return t == BuildTargetDashboard || t == BuildTargetAll
}

type ProjectionStatus struct {
	GeneratedAt string   `json:"generated_at,omitempty"`
	Complete    bool     `json:"complete"`
	Files       []string `json:"files,omitempty"`
}

type OutputManifest struct {
	Tool        string           `json:"tool"`
	Schema      int              `json:"schema"`
	Scope       string           `json:"scope"`
	OutputDir   string           `json:"output_dir"`
	ProjectRoot string           `json:"project_root,omitempty"`
	Files       int              `json:"files,omitempty"`
	Skipped     int              `json:"skipped,omitempty"`
	Index       ProjectionStatus `json:"index"`
	Agent       ProjectionStatus `json:"agent"`
	Dashboard   ProjectionStatus `json:"dashboard"`
	Git         *GitMetadata     `json:"git,omitempty"`
}

type Manifest = OutputManifest
