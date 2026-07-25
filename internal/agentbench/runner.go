package agentbench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorecodecom/goregraph/internal/agent"
)

const regressionSummaryHeader = "case\tquery\tbuild\trun\tattempt\ttokens\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tincluded_source_rereads\tcontext_millis\tlog\n"

var lowerCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type RunnerConfig struct {
	MatrixPath      string
	ExternalCase    string
	GoldenBinary    string
	GoldenCommit    string
	CandidateBinary string
	CandidateCommit string
	InstructionPath string
	Phase           string
	TargetCase      string
	Runs            int
	Output          string
	CodexArgs       []string
	AnalyzerPath    string
}

type regressionCase struct {
	id             string
	workspace      string
	contract       Contract
	contractBody   []byte
	queries        map[string]string
	sourceSnapshot string
	sourceHash     string
}

type regressionPlan struct {
	benchmarkCase *regressionCase
	build         string
	run           int
	attempt       int
	workspace     string
}

type runnerState struct {
	config       RunnerConfig
	cases        []*regressionCase
	caseByID     map[string]*regressionCase
	plans        []*regressionPlan
	codexPath    string
	bashPath     string
	matrixBody   []byte
	instruction  []byte
	snapshotRoot string
	privateBins  map[string]string
	packs        map[string]agent.ContextPack
	hashes       map[string]string
}

type transcriptMetrics struct {
	tokens                int64
	toolCalls             int64
	contextCalls          int64
	repeatedFullPacks     int64
	broadNavigationCalls  int64
	sourceReadCalls       int64
	includedSourceRereads int64
	contextMillis         int64
}

type processFailure struct {
	stage  string
	stdout []byte
	stderr []byte
	err    error
}

func (failure *processFailure) Error() string {
	return fmt.Sprintf("%s process failed: %v", failure.stage, failure.err)
}

func (failure *processFailure) Unwrap() error {
	return failure.err
}

func RunRegression(ctx context.Context, config RunnerConfig) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := validateRunner(config)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshotParent, err := filepath.EvalSymlinks(filepath.Dir(config.Output))
	if err != nil {
		return fmt.Errorf("resolve source snapshot parent: %w", err)
	}
	snapshotRoot, err := os.MkdirTemp(
		snapshotParent,
		".goregraph-agentbench-snapshot-*",
	)
	if err != nil {
		return fmt.Errorf("create source snapshot root: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, os.RemoveAll(snapshotRoot))
	}()
	state.snapshotRoot = snapshotRoot
	if err := state.createOutput(ctx); err != nil {
		return err
	}
	if err := state.prepareInputsAndSources(ctx); err != nil {
		return err
	}
	if err := state.writeIdentity(ctx); err != nil {
		return err
	}
	for _, plan := range state.plans {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := state.prepareWorkspaceAndScan(ctx, plan); err != nil {
			return err
		}
		if err := state.runParityQueries(ctx, plan); err != nil {
			return err
		}
		if err := state.runEndToEnd(ctx, plan); err != nil {
			return err
		}
	}
	return nil
}

func validateRunner(config RunnerConfig) (*runnerState, error) {
	if err := validateRunnerPaths(config); err != nil {
		return nil, err
	}
	if config.Phase != "smoke" && config.Phase != "full" {
		return nil, fmt.Errorf("phase must be smoke or full")
	}
	if config.Phase == "smoke" && config.Runs != 1 {
		return nil, fmt.Errorf("smoke phase requires exactly one run")
	}
	if config.Phase == "full" && config.Runs != 3 {
		return nil, fmt.Errorf("full phase requires exactly three gateable runs")
	}
	if !lowerCommitPattern.MatchString(config.GoldenCommit) {
		return nil, fmt.Errorf("golden commit must be lowercase 40-hex")
	}
	if !lowerCommitPattern.MatchString(config.CandidateCommit) {
		return nil, fmt.Errorf("candidate commit must be lowercase 40-hex")
	}
	if err := validateCodexArgs(config.CodexArgs); err != nil {
		return nil, err
	}

	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return nil, fmt.Errorf("resolve codex: %w", err)
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("resolve bash: %w", err)
	}
	instruction, err := os.ReadFile(config.InstructionPath)
	if err != nil {
		return nil, fmt.Errorf("read instruction: %w", err)
	}
	if len(bytes.TrimSpace(instruction)) == 0 {
		return nil, fmt.Errorf("instruction must not be empty")
	}

	matrixBody, err := os.ReadFile(config.MatrixPath)
	if err != nil {
		return nil, fmt.Errorf("read matrix: %w", err)
	}
	matrix, err := decodeValidatedMatrix(matrixBody)
	if err != nil {
		return nil, fmt.Errorf("load matrix: %w", err)
	}
	cases, err := loadRegressionCases(config, matrix)
	if err != nil {
		return nil, err
	}
	canonicalOutput, err := canonicalNewPath(config.Output)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}
	caseByID := make(map[string]*regressionCase, len(cases))
	for _, benchmarkCase := range cases {
		caseByID[benchmarkCase.id] = benchmarkCase
		if err := validateCopySource(benchmarkCase.workspace); err != nil {
			return nil, fmt.Errorf("validate %s workspace: %w", benchmarkCase.id, err)
		}
		canonicalWorkspace, err := filepath.EvalSymlinks(benchmarkCase.workspace)
		if err != nil {
			return nil, fmt.Errorf("resolve %s workspace: %w", benchmarkCase.id, err)
		}
		if pathWithin(canonicalOutput, canonicalWorkspace) {
			return nil, fmt.Errorf("output must be outside input workspace %s", benchmarkCase.id)
		}
	}
	if caseByID[config.TargetCase] == nil {
		return nil, fmt.Errorf("target case %q does not exist", config.TargetCase)
	}

	selected := cases
	if config.Phase == "smoke" {
		selected = []*regressionCase{caseByID["g1"]}
		if config.TargetCase != "g1" {
			selected = append(selected, caseByID[config.TargetCase])
		}
	}
	plans := make([]*regressionPlan, 0, len(selected)*config.Runs*2)
	for _, benchmarkCase := range selected {
		for run := 1; run <= config.Runs; run++ {
			builds := []string{"golden", "candidate"}
			if run%2 == 0 {
				builds[0], builds[1] = builds[1], builds[0]
			}
			for _, build := range builds {
				plans = append(plans, &regressionPlan{
					benchmarkCase: benchmarkCase,
					build:         build,
					run:           run,
					attempt:       1,
				})
			}
		}
	}
	return &runnerState{
		config:      config,
		cases:       cases,
		caseByID:    caseByID,
		plans:       plans,
		codexPath:   codexPath,
		bashPath:    bashPath,
		matrixBody:  append([]byte(nil), matrixBody...),
		instruction: append([]byte(nil), instruction...),
		privateBins: make(map[string]string),
		packs:       make(map[string]agent.ContextPack),
		hashes:      make(map[string]string),
	}, nil
}

func validateRunnerPaths(config RunnerConfig) error {
	paths := []struct {
		name       string
		path       string
		directory  bool
		executable bool
	}{
		{name: "matrix", path: config.MatrixPath},
		{name: "external case", path: config.ExternalCase, directory: true},
		{name: "golden binary", path: config.GoldenBinary, executable: true},
		{name: "candidate binary", path: config.CandidateBinary, executable: true},
		{name: "instruction", path: config.InstructionPath},
		{name: "analyzer", path: config.AnalyzerPath},
	}
	for _, input := range paths {
		if !filepath.IsAbs(input.path) {
			return fmt.Errorf("%s path must be absolute", input.name)
		}
		info, err := os.Lstat(input.path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", input.name, err)
		}
		if input.directory {
			if !info.IsDir() {
				return fmt.Errorf("%s must be a directory", input.name)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular file", input.name)
		}
		if input.executable && info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("%s must be executable", input.name)
		}
	}
	goldenInfo, err := os.Stat(config.GoldenBinary)
	if err != nil {
		return fmt.Errorf("inspect golden binary identity: %w", err)
	}
	candidateInfo, err := os.Stat(config.CandidateBinary)
	if err != nil {
		return fmt.Errorf("inspect candidate binary identity: %w", err)
	}
	if os.SameFile(goldenInfo, candidateInfo) {
		return fmt.Errorf("golden and candidate binaries must be different files")
	}
	if !filepath.IsAbs(config.Output) {
		return fmt.Errorf("output path must be absolute")
	}
	parent, err := os.Stat(filepath.Dir(config.Output))
	if err != nil || !parent.IsDir() {
		return fmt.Errorf("output parent must be an existing directory")
	}
	if _, err := os.Lstat(config.Output); err == nil {
		return fmt.Errorf("output path already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output path: %w", err)
	}
	return nil
}

func loadRegressionCases(config RunnerConfig, matrix Matrix) ([]*regressionCase, error) {
	expected := map[string]bool{
		"g2-java-missing-contract":         true,
		"g3-go-existing-flow":              true,
		"g4-typescript-consumer-provider":  true,
		"g5-java-persistence-side-effects": true,
		"g6-ambiguous-entrypoint":          true,
	}
	if len(matrix.Cases) != len(expected) {
		return nil, fmt.Errorf("matrix must contain exactly committed G2-G6 cases")
	}
	matrixRoot := filepath.Dir(config.MatrixPath)
	cases := make([]*regressionCase, 0, 6)
	external, err := loadRegressionCase(config.ExternalCase, "g1")
	if err != nil {
		return nil, fmt.Errorf("load external G1: %w", err)
	}
	cases = append(cases, external)
	seen := make(map[string]bool, len(matrix.Cases))
	for _, declared := range matrix.Cases {
		if !expected[declared.ID] {
			return nil, fmt.Errorf("unexpected matrix case %q", declared.ID)
		}
		if seen[declared.ID] {
			return nil, fmt.Errorf("duplicate matrix case %q", declared.ID)
		}
		seen[declared.ID] = true
		if declared.External {
			return nil, fmt.Errorf("committed matrix case %q must not be external", declared.ID)
		}
		if !safeComponent(declared.Directory) {
			return nil, fmt.Errorf("matrix case %q has unsafe directory", declared.ID)
		}
		benchmarkCase, err := loadRegressionCase(
			filepath.Join(matrixRoot, filepath.FromSlash(declared.Directory)),
			declared.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("load matrix case %s: %w", declared.ID, err)
		}
		cases = append(cases, benchmarkCase)
	}
	for id := range expected {
		if !seen[id] {
			return nil, fmt.Errorf("matrix is missing case %q", id)
		}
	}
	return cases, nil
}

func loadRegressionCase(root, expectedID string) (*regressionCase, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("case root must be a directory")
	}
	contractPath := filepath.Join(root, "contract.json")
	contractInfo, err := os.Lstat(contractPath)
	if err != nil {
		return nil, err
	}
	if !contractInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("contract must be a regular non-symlink file")
	}
	contractBody, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, err
	}
	contract, err := decodeValidatedContract(contractBody)
	if err != nil {
		return nil, err
	}
	if contract.ID != expectedID || !safeComponent(contract.ID) {
		return nil, fmt.Errorf("contract ID %q does not match %q", contract.ID, expectedID)
	}
	queries := make(map[string]string, len(contract.Queries))
	for _, query := range contract.Queries {
		if !safeComponent(query.ID) {
			return nil, fmt.Errorf("query ID %q is unsafe", query.ID)
		}
		if !safeComponent(query.File) {
			return nil, fmt.Errorf("query %q has unsafe file path", query.ID)
		}
		path := filepath.Join(root, filepath.FromSlash(query.File))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect query %s: %w", query.ID, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("query %q must be a regular file", query.ID)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(body)) == 0 {
			return nil, fmt.Errorf("query %q must not be empty", query.ID)
		}
		queries[query.ID] = string(body)
	}
	workspace := filepath.Join(root, "workspace")
	workspaceInfo, err := os.Lstat(workspace)
	if err != nil {
		return nil, err
	}
	if !workspaceInfo.IsDir() || workspaceInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace must be a non-symlink directory")
	}
	return &regressionCase{
		id: expectedID, workspace: workspace,
		contract:     contract,
		contractBody: append([]byte(nil), contractBody...),
		queries:      queries,
	}, nil
}

func decodeValidatedMatrix(body []byte) (Matrix, error) {
	var matrix Matrix
	if err := decodeRunnerJSON(body, &matrix); err != nil {
		return Matrix{}, err
	}
	if err := ValidateMatrix(matrix); err != nil {
		return Matrix{}, err
	}
	return matrix, nil
}

func decodeValidatedContract(body []byte) (Contract, error) {
	var contract Contract
	if err := decodeRunnerJSON(body, &contract); err != nil {
		return Contract{}, err
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func decodeRunnerJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func safeComponent(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		filepath.Base(value) == value &&
		!strings.ContainsAny(value, "/\\\t\r\n") &&
		!hasControl(value)
}

func safeRelativePath(value string) bool {
	if value == "" || strings.ContainsAny(value, "\t\r\n") || hasControl(value) {
		return false
	}
	native := filepath.FromSlash(value)
	return filepath.IsLocal(native) && filepath.Clean(native) == native
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func canonicalNewPath(path string) (string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func validateCopySource(source string) error {
	return fs.WalkDir(os.DirFS(source), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != "." && !safeRelativePath(path) {
			return fmt.Errorf("unsafe workspace path %q", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace symlink is not allowed: %s", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("workspace entry must be regular: %s", path)
		}
		return nil
	})
}

func validateCodexArgs(args []string) error {
	counts := make(map[string]int)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		next := func() (string, error) {
			index++
			if index >= len(args) {
				return "", fmt.Errorf("%s requires a value", argument)
			}
			return args[index], nil
		}
		switch {
		case argument == "exec":
			counts["exec"]++
		case argument == "-a" || argument == "--ask-for-approval":
			value, err := next()
			if err != nil || value != "never" {
				return fmt.Errorf("approval mode must be never")
			}
			counts["approval"]++
		case argument == "--ask-for-approval=never":
			counts["approval"]++
		case strings.HasPrefix(argument, "--ask-for-approval="):
			return fmt.Errorf("approval mode must be never")
		case argument == "-s" || argument == "--sandbox":
			value, err := next()
			if err != nil || value != "read-only" {
				return fmt.Errorf("sandbox must be read-only")
			}
			counts["sandbox"]++
		case argument == "--sandbox=read-only":
			counts["sandbox"]++
		case strings.HasPrefix(argument, "--sandbox="):
			return fmt.Errorf("sandbox must be read-only")
		case argument == "-m" || argument == "--model":
			value, err := next()
			if err != nil || strings.TrimSpace(value) == "" {
				return fmt.Errorf("model must not be empty or blank")
			}
			counts["model"]++
		case strings.HasPrefix(argument, "--model="):
			if strings.TrimSpace(strings.TrimPrefix(argument, "--model=")) == "" {
				return fmt.Errorf("model must not be empty or blank")
			}
			counts["model"]++
		case argument == "-c" || argument == "--config":
			value, err := next()
			if err != nil || !validReasoningConfig(value) {
				return fmt.Errorf("unsupported Codex config override")
			}
			counts["reasoning"]++
		case strings.HasPrefix(argument, "--config="):
			if !validReasoningConfig(strings.TrimPrefix(argument, "--config=")) {
				return fmt.Errorf("unsupported Codex config override")
			}
			counts["reasoning"]++
		case argument == "--color":
			value, err := next()
			if err != nil || value != "never" {
				return fmt.Errorf("color mode must be never")
			}
			counts["color"]++
		case argument == "--color=never":
			counts["color"]++
		case strings.HasPrefix(argument, "--color="):
			return fmt.Errorf("color mode must be never")
		case argument == "--skip-git-repo-check":
			counts["skip"]++
		case argument == "--ephemeral":
			counts["ephemeral"]++
		case argument == "--ignore-rules":
			counts["rules"]++
		case argument == "--ignore-user-config":
			counts["config"]++
		case argument == "--dangerously-bypass-approvals-and-sandbox" ||
			argument == "--dangerously-bypass-hook-trust" ||
			argument == "--search" ||
			argument == "--json" ||
			argument == "--add-dir" ||
			argument == "--cd" ||
			argument == "-C" ||
			strings.HasPrefix(argument, "--dangerously-bypass-approvals-and-sandbox=") ||
			strings.HasPrefix(argument, "--dangerously-bypass-hook-trust=") ||
			strings.HasPrefix(argument, "--search=") ||
			strings.HasPrefix(argument, "--json=") ||
			strings.HasPrefix(argument, "--add-dir=") ||
			strings.HasPrefix(argument, "--cd=") ||
			strings.HasPrefix(argument, "-C"):
			return fmt.Errorf("unsafe or script-controlled Codex argument: %s", argument)
		case strings.HasPrefix(argument, "-"):
			return fmt.Errorf("unsupported Codex benchmark argument: %s", argument)
		default:
			return fmt.Errorf("unexpected positional Codex argument: %s", argument)
		}
	}
	for _, required := range []string{
		"approval", "sandbox", "exec", "model", "reasoning",
		"color", "skip", "ephemeral", "rules", "config",
	} {
		if counts[required] != 1 {
			return fmt.Errorf("Codex arguments must contain %s exactly once", required)
		}
	}
	return nil
}

func validReasoningConfig(value string) bool {
	switch value {
	case "model_reasoning_effort=minimal",
		"model_reasoning_effort=low",
		"model_reasoning_effort=medium",
		"model_reasoning_effort=high",
		"model_reasoning_effort=xhigh",
		`model_reasoning_effort="minimal"`,
		`model_reasoning_effort="low"`,
		`model_reasoning_effort="medium"`,
		`model_reasoning_effort="high"`,
		`model_reasoning_effort="xhigh"`:
		return true
	default:
		return false
	}
}

func (state *runnerState) createOutput(ctx context.Context) error {
	if err := os.Mkdir(state.config.Output, 0o700); err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	for _, relative := range []string{
		"inputs/external-case",
		"identity",
		"cases",
		"runs",
		"reviews",
		"workspaces",
		"runtime/golden",
		"runtime/candidate",
	} {
		if err := os.MkdirAll(filepath.Join(state.config.Output, filepath.FromSlash(relative)), 0o700); err != nil {
			return err
		}
	}
	if err := copyExecutable(
		ctx,
		state.config.GoldenBinary,
		filepath.Join(state.config.Output, "runtime", "golden", "goregraph"),
	); err != nil {
		return err
	}
	if err := copyExecutable(
		ctx,
		state.config.CandidateBinary,
		filepath.Join(state.config.Output, "runtime", "candidate", "goregraph"),
	); err != nil {
		return err
	}
	state.privateBins["golden"] = filepath.Join(state.config.Output, "runtime", "golden")
	state.privateBins["candidate"] = filepath.Join(state.config.Output, "runtime", "candidate")
	return nil
}

func (state *runnerState) prepareInputsAndSources(ctx context.Context) error {
	if err := writeFileNew(
		filepath.Join(state.config.Output, "inputs", "matrix.json"),
		state.matrixBody,
		0o600,
	); err != nil {
		return err
	}
	if err := writeFileNew(
		filepath.Join(state.config.Output, "inputs", "instruction.txt"),
		state.instruction,
		0o600,
	); err != nil {
		return err
	}
	if err := writeFileNew(
		filepath.Join(state.config.Output, "inputs", "external-case", "contract.json"),
		state.caseByID["g1"].contractBody,
		0o600,
	); err != nil {
		return err
	}
	if err := writeFileNew(filepath.Join(state.config.Output, "summary.tsv"), []byte(regressionSummaryHeader), 0o600); err != nil {
		return err
	}

	var promptDigests strings.Builder
	promptDigests.WriteString("case\tquery\tsha256\n")
	for _, benchmarkCase := range state.cases {
		for _, query := range benchmarkCase.contract.Queries {
			prompt := state.prompt(benchmarkCase, query)
			fmt.Fprintf(&promptDigests, "%s\t%s\t%s\n", benchmarkCase.id, query.ID, hashBytes([]byte(prompt)))
		}
	}
	if err := writeFileNew(
		filepath.Join(state.config.Output, "inputs", "prompt-digests.tsv"),
		[]byte(promptDigests.String()),
		0o600,
	); err != nil {
		return err
	}

	for _, benchmarkCase := range state.cases {
		if err := ctx.Err(); err != nil {
			return err
		}
		benchmarkCase.sourceSnapshot = filepath.Join(
			state.snapshotRoot,
			benchmarkCase.id,
			"workspace",
		)
		if err := os.MkdirAll(
			filepath.Dir(benchmarkCase.sourceSnapshot),
			0o700,
		); err != nil {
			return err
		}
		if err := copyWorkspace(
			ctx,
			benchmarkCase.workspace,
			benchmarkCase.sourceSnapshot,
		); err != nil {
			return fmt.Errorf("freeze %s workspace: %w", benchmarkCase.id, err)
		}
		sourceHash, err := hashTree(ctx, benchmarkCase.sourceSnapshot)
		if err != nil {
			return err
		}
		benchmarkCase.sourceHash = sourceHash
	}
	return nil
}

func (state *runnerState) prepareWorkspaceAndScan(
	ctx context.Context,
	plan *regressionPlan,
) error {
	query := endToEndQuery(plan.benchmarkCase.contract)
	plan.workspace = filepath.Join(
		state.config.Output,
		"workspaces",
		plan.benchmarkCase.id,
		fmt.Sprintf("%s-%d-%d", plan.build, plan.run, plan.attempt),
	)
	if err := os.MkdirAll(filepath.Dir(plan.workspace), 0o700); err != nil {
		return err
	}
	if err := copyWorkspace(
		ctx,
		plan.benchmarkCase.sourceSnapshot,
		plan.workspace,
	); err != nil {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"prepare",
			nil,
			nil,
			transcriptMetrics{},
			err,
		)
	}
	workspaceHash, err := hashTree(ctx, plan.workspace)
	if err != nil {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"prepare",
			nil,
			nil,
			transcriptMetrics{},
			err,
		)
	}
	if workspaceHash != plan.benchmarkCase.sourceHash {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"prepare",
			nil,
			nil,
			transcriptMetrics{},
			fmt.Errorf("workspace source hash differs from frozen snapshot"),
		)
	}
	if err := state.rememberHash(
		plan.benchmarkCase.id,
		plan.build,
		"workspace",
		workspaceHash,
	); err != nil {
		return err
	}

	stdout, stderr, err := runProcess(ctx, state.binary(plan.build), []string{
		"workspace", "scan-all", plan.workspace,
		"--workspace", plan.workspace,
		"--no-update-gitignore",
	}, nil, nil)
	if err != nil {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"scan",
			stdout,
			stderr,
			transcriptMetrics{},
			err,
		)
	}
	indexPath := filepath.Join(
		plan.workspace,
		".goregraph-workspace",
		"agent",
		"context-index.json",
	)
	indexInfo, err := os.Lstat(indexPath)
	if err != nil || !indexInfo.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("context index must be a regular non-symlink file")
		}
		return state.retainInfrastructureFailure(
			plan,
			query,
			"scan",
			stdout,
			stderr,
			transcriptMetrics{},
			fmt.Errorf("inspect context index: %w", err),
		)
	}
	indexHash, err := hashFile(ctx, indexPath)
	if err != nil {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"scan",
			stdout,
			stderr,
			transcriptMetrics{},
			fmt.Errorf("hash context index: %w", err),
		)
	}
	if err := state.rememberHash(
		plan.benchmarkCase.id,
		plan.build,
		"index",
		indexHash,
	); err != nil {
		return state.retainInfrastructureFailure(
			plan,
			query,
			"scan",
			stdout,
			stderr,
			transcriptMetrics{},
			err,
		)
	}
	return nil
}

func (state *runnerState) writeIdentity(ctx context.Context) error {
	goldenHash, err := hashFile(ctx, state.binary("golden"))
	if err != nil {
		return err
	}
	candidateHash, err := hashFile(ctx, state.binary("candidate"))
	if err != nil {
		return err
	}
	identities := map[string]string{
		"golden-binary.sha256":    goldenHash + "\n",
		"candidate-binary.sha256": candidateHash + "\n",
		"golden-commit.txt":       state.config.GoldenCommit + "\n",
		"candidate-commit.txt":    state.config.CandidateCommit + "\n",
		"codex-args.txt":          strings.Join(state.config.CodexArgs, "\n") + "\n",
	}
	for name, body := range identities {
		if err := writeFileNew(filepath.Join(state.config.Output, "identity", name), []byte(body), 0o600); err != nil {
			return err
		}
	}
	stdout, stderr, err := runProcess(ctx, state.codexPath, []string{"--version"}, nil, nil)
	if err != nil {
		return fmt.Errorf("codex --version: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	if err := writeFileNew(
		filepath.Join(state.config.Output, "identity", "codex-version.txt"),
		stdout,
		0o600,
	); err != nil {
		return err
	}

	var order strings.Builder
	order.WriteString("case\tquery\tbuild\trun\tattempt\n")
	for _, plan := range state.plans {
		query := endToEndQuery(plan.benchmarkCase.contract)
		fmt.Fprintf(
			&order, "%s\t%s\t%s\t%d\t%d\n",
			plan.benchmarkCase.id, query.ID, plan.build, plan.run, plan.attempt,
		)
	}
	return writeFileNew(
		filepath.Join(state.config.Output, "identity", "run-order.tsv"),
		[]byte(order.String()),
		0o600,
	)
}

func (state *runnerState) runParityQueries(
	ctx context.Context,
	plan *regressionPlan,
) error {
	for _, query := range plan.benchmarkCase.contract.Queries {
		if query.EndToEnd {
			continue
		}
		elapsed, err := state.runContext(ctx, plan, query)
		if err != nil {
			return state.retainProcessFailure(
				plan,
				query,
				transcriptMetrics{contextMillis: elapsed},
				err,
			)
		}
		if state.hasBothPacks(plan.benchmarkCase.id, query.ID) {
			if err := state.writePackDiff(
				plan.benchmarkCase,
				query,
			); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
	}
	return nil
}

func (state *runnerState) runEndToEnd(ctx context.Context, plan *regressionPlan) error {
	query := endToEndQuery(plan.benchmarkCase.contract)
	contextMillis, err := state.runContext(ctx, plan, query)
	if err != nil {
		return state.retainProcessFailure(
			plan,
			query,
			transcriptMetrics{contextMillis: contextMillis},
			err,
		)
	}
	if state.hasBothPacks(plan.benchmarkCase.id, query.ID) {
		if err := state.writePackDiff(plan.benchmarkCase, query); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}

	runDirectory := filepath.Join(
		state.config.Output, "runs", plan.benchmarkCase.id, query.ID,
	)
	reviewDirectory := filepath.Join(
		state.config.Output, "reviews", plan.benchmarkCase.id, query.ID,
	)
	if err := os.MkdirAll(runDirectory, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(reviewDirectory, 0o700); err != nil {
		return err
	}
	baseName := fmt.Sprintf("%s-%d-%d", plan.build, plan.run, plan.attempt)
	logPath := filepath.Join(runDirectory, baseName+".jsonl")
	stderrPath := filepath.Join(runDirectory, baseName+".stderr")
	metricsPath := filepath.Join(runDirectory, baseName+".metrics.tsv")
	reviewPath := filepath.Join(reviewDirectory, baseName+".json")
	logFile, err := openFileNew(logPath, 0o600)
	if err != nil {
		return err
	}
	stderrFile, err := openFileNew(stderrPath, 0o600)
	if err != nil {
		_ = logFile.Close()
		return err
	}
	args := append([]string{}, state.config.CodexArgs...)
	args = append(args, "--json", "-C", plan.workspace, "-")
	command := exec.CommandContext(ctx, state.codexPath, args...)
	command.Stdin = strings.NewReader(state.prompt(plan.benchmarkCase, query))
	command.Stdout = logFile
	command.Stderr = stderrFile
	command.Env = environmentWithPath(state.privateBins[plan.build])
	runErr := command.Run()
	closeErr := errors.Join(logFile.Close(), stderrFile.Close())
	if closeErr != nil && runErr == nil {
		runErr = closeErr
	}
	if runErr != nil && ctx.Err() != nil {
		runErr = ctx.Err()
	}
	if runErr != nil {
		metrics := transcriptMetrics{contextMillis: contextMillis}
		if err := writeRunMetrics(metricsPath, metrics); err != nil {
			return err
		}
		reviewed := state.reviewTemplate(plan, metrics, &InvalidRun{
			InfrastructureFailure: true,
			Reason:                fmt.Sprintf("Codex failed: %v", runErr),
			RetainedLog:           logPath,
		})
		if err := writeJSONNew(reviewPath, reviewed); err != nil {
			return err
		}
		return fmt.Errorf("Codex %s %s run %d: %w", plan.benchmarkCase.id, plan.build, plan.run, runErr)
	}

	metrics, analyzerStderr, err := state.analyzeTranscript(ctx, logPath)
	metrics.contextMillis = contextMillis
	if err != nil {
		if len(analyzerStderr) > 0 {
			if appendErr := appendFile(stderrPath, analyzerStderr); appendErr != nil {
				return appendErr
			}
		}
		if writeErr := writeRunMetrics(metricsPath, metrics); writeErr != nil {
			return writeErr
		}
		reviewed := state.reviewTemplate(plan, metrics, &InvalidRun{
			InfrastructureFailure: true,
			Reason:                fmt.Sprintf("analyzer failed: %v", err),
			RetainedLog:           logPath,
		})
		if writeErr := writeJSONNew(reviewPath, reviewed); writeErr != nil {
			return writeErr
		}
		return err
	}
	if err := writeRunMetrics(metricsPath, metrics); err != nil {
		return err
	}
	if err := writeJSONNew(reviewPath, state.reviewTemplate(plan, metrics, nil)); err != nil {
		return err
	}
	return state.appendSummary(plan, query, metrics, logPath)
}

func (state *runnerState) runContext(
	ctx context.Context,
	plan *regressionPlan,
	query QueryVariant,
) (int64, error) {
	start := time.Now()
	stdout, stderr, err := runProcess(ctx, state.binary(plan.build), []string{
		"context", plan.workspace,
		"--query", strings.TrimSpace(plan.benchmarkCase.queries[query.ID]),
		"--budget-tokens", "4000",
		"--max-files", "12",
		"--format", "json",
	}, nil, nil)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return elapsed, &processFailure{
			stage:  "context",
			stdout: stdout,
			stderr: stderr,
			err:    err,
		}
	}
	pack, err := decodeContextPack(stdout)
	if err != nil {
		return elapsed, &processFailure{
			stage:  "context",
			stdout: stdout,
			stderr: stderr,
			err:    err,
		}
	}
	key := packKey(plan.benchmarkCase.id, query.ID, plan.build)
	if previous, exists := state.packs[key]; exists {
		if !reflect.DeepEqual(ProjectPack(previous), ProjectPack(pack)) {
			return elapsed, &processFailure{
				stage:  "context",
				stdout: stdout,
				stderr: stderr,
				err: fmt.Errorf(
					"Context projection changed across %s %s runs",
					plan.benchmarkCase.id,
					plan.build,
				),
			}
		}
		return elapsed, nil
	}
	state.packs[key] = pack
	caseDirectory := filepath.Join(state.config.Output, "cases", plan.benchmarkCase.id, query.ID)
	if err := os.MkdirAll(caseDirectory, 0o700); err != nil {
		return elapsed, err
	}
	if err := writeJSONNew(filepath.Join(caseDirectory, plan.build+"-pack.json"), pack); err != nil {
		return elapsed, err
	}
	for _, kind := range []string{"workspace", "index"} {
		if err := writeFileNew(
			filepath.Join(caseDirectory, plan.build+"-"+kind+".sha256"),
			[]byte(state.hashes[hashKey(plan.benchmarkCase.id, plan.build, kind)]+"\n"),
			0o600,
		); err != nil {
			return elapsed, err
		}
	}
	return elapsed, nil
}

func (state *runnerState) writePackDiff(
	benchmarkCase *regressionCase,
	query QueryVariant,
) error {
	path := filepath.Join(
		state.config.Output, "cases", benchmarkCase.id, query.ID, "pack-diff.json",
	)
	diff := DiffPacks(
		state.packs[packKey(benchmarkCase.id, query.ID, "golden")],
		state.packs[packKey(benchmarkCase.id, query.ID, "candidate")],
	)
	return writeJSONNew(path, diff)
}

func (state *runnerState) analyzeTranscript(
	ctx context.Context,
	logPath string,
) (transcriptMetrics, []byte, error) {
	tokenOutput, tokenStderr, err := runProcess(
		ctx, state.bashPath,
		[]string{state.config.AnalyzerPath, "--tokens", logPath},
		nil, nil,
	)
	if err != nil {
		return transcriptMetrics{}, tokenStderr, fmt.Errorf(
			"extract transcript tokens: %w: %s",
			err,
			strings.TrimSpace(string(tokenStderr)),
		)
	}
	tokens, err := parseNonnegativeInteger(strings.TrimSpace(string(tokenOutput)))
	if err != nil {
		return transcriptMetrics{}, nil, fmt.Errorf("parse transcript tokens: %w", err)
	}
	output, stderr, err := runProcess(
		ctx, state.bashPath,
		[]string{state.config.AnalyzerPath, logPath},
		nil, nil,
	)
	if err != nil {
		return transcriptMetrics{tokens: tokens}, stderr, fmt.Errorf(
			"analyze transcript: %w: %s",
			err,
			strings.TrimSpace(string(stderr)),
		)
	}
	fields := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(fields) != 9 {
		return transcriptMetrics{tokens: tokens}, nil, fmt.Errorf(
			"analyzer returned %d fields, want 9",
			len(fields),
		)
	}
	values := make([]int64, len(fields))
	for index, field := range fields {
		value, err := parseNonnegativeInteger(field)
		if err != nil {
			return transcriptMetrics{tokens: tokens}, nil, fmt.Errorf(
				"analyzer field %d: %w",
				index+1,
				err,
			)
		}
		values[index] = value
	}
	return transcriptMetrics{
		tokens:                tokens,
		toolCalls:             values[0],
		contextCalls:          values[1],
		repeatedFullPacks:     values[4],
		broadNavigationCalls:  values[5],
		sourceReadCalls:       values[6],
		includedSourceRereads: values[7],
	}, nil, nil
}

func (state *runnerState) retainProcessFailure(
	plan *regressionPlan,
	query QueryVariant,
	metrics transcriptMetrics,
	err error,
) error {
	var failure *processFailure
	if !errors.As(err, &failure) {
		return err
	}
	return state.retainInfrastructureFailure(
		plan,
		query,
		failure.stage,
		failure.stdout,
		failure.stderr,
		metrics,
		failure.err,
	)
}

func (state *runnerState) retainInfrastructureFailure(
	plan *regressionPlan,
	query QueryVariant,
	stage string,
	stdout []byte,
	stderr []byte,
	metrics transcriptMetrics,
	cause error,
) error {
	runDirectory := filepath.Join(
		state.config.Output, "runs", plan.benchmarkCase.id, query.ID,
	)
	reviewDirectory := filepath.Join(
		state.config.Output, "reviews", plan.benchmarkCase.id, query.ID,
	)
	if err := os.MkdirAll(runDirectory, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(reviewDirectory, 0o700); err != nil {
		return err
	}
	baseName := fmt.Sprintf("%s-%d-%d", plan.build, plan.run, plan.attempt)
	logPath := filepath.Join(runDirectory, baseName+".jsonl")
	if err := writeFileNew(logPath, stdout, 0o600); err != nil {
		return err
	}
	if err := writeFileNew(
		filepath.Join(runDirectory, baseName+".stderr"),
		stderr,
		0o600,
	); err != nil {
		return err
	}
	if err := writeRunMetrics(
		filepath.Join(runDirectory, baseName+".metrics.tsv"),
		metrics,
	); err != nil {
		return err
	}
	failure := fmt.Errorf(
		"%s %s %s run %d: %w",
		stage,
		plan.benchmarkCase.id,
		plan.build,
		plan.run,
		cause,
	)
	reviewed := state.reviewTemplate(plan, metrics, &InvalidRun{
		InfrastructureFailure: true,
		Reason:                failure.Error(),
		RetainedLog:           logPath,
	})
	if err := writeJSONNew(
		filepath.Join(reviewDirectory, baseName+".json"),
		reviewed,
	); err != nil {
		return err
	}
	return failure
}

func parseNonnegativeInteger(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%q is not a nonnegative integer", value)
	}
	return parsed, nil
}

func writeRunMetrics(path string, metrics transcriptMetrics) error {
	body := fmt.Sprintf(
		"tokens\ttool_calls\tcontext_calls\trepeated_full_packs\tbroad_navigation_calls\tsource_read_calls\tincluded_source_rereads\tcontext_millis\n%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
		metrics.tokens,
		metrics.toolCalls,
		metrics.contextCalls,
		metrics.repeatedFullPacks,
		metrics.broadNavigationCalls,
		metrics.sourceReadCalls,
		metrics.includedSourceRereads,
		metrics.contextMillis,
	)
	return writeFileNew(path, []byte(body), 0o600)
}

func (state *runnerState) reviewTemplate(
	plan *regressionPlan,
	metrics transcriptMetrics,
	invalid *InvalidRun,
) ReviewedRun {
	facets := make(map[string]FacetResult)
	for _, definition := range append(
		append([]FacetDefinition{}, plan.benchmarkCase.contract.Answer.RequiredFacets...),
		plan.benchmarkCase.contract.Answer.ExplicitUnknowns...,
	) {
		facets[definition.ID] = FacetResult{
			Status: "fail", Evidence: "Review required before gating.",
		}
	}
	forbidden := make(map[string]FacetResult)
	for _, definition := range plan.benchmarkCase.contract.Answer.ForbiddenOutcomes {
		forbidden[definition.ID] = FacetResult{
			Status: "fail", Evidence: "Review required before gating.",
		}
	}
	return ReviewedRun{
		Review: RunReview{
			Schema:            1,
			CaseID:            plan.benchmarkCase.id,
			Build:             plan.build,
			Run:               plan.run,
			Attempt:           plan.attempt,
			Facets:            facets,
			ForbiddenOutcomes: forbidden,
		},
		Metrics: RunMetrics{
			Tokens:                metrics.tokens,
			ToolCalls:             metrics.toolCalls,
			SourceReads:           metrics.sourceReadCalls,
			IncludedSourceRereads: metrics.includedSourceRereads,
			RepeatedFullPacks:     metrics.repeatedFullPacks,
			ContextCalls:          metrics.contextCalls,
			ContextMillis:         metrics.contextMillis,
			BroadNavigationCalls:  metrics.broadNavigationCalls,
		},
		Invalid: invalid,
	}
}

func (state *runnerState) appendSummary(
	plan *regressionPlan,
	query QueryVariant,
	metrics transcriptMetrics,
	logPath string,
) error {
	line := fmt.Sprintf(
		"%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
		plan.benchmarkCase.id,
		query.ID,
		plan.build,
		plan.run,
		plan.attempt,
		metrics.tokens,
		metrics.toolCalls,
		metrics.contextCalls,
		metrics.repeatedFullPacks,
		metrics.broadNavigationCalls,
		metrics.sourceReadCalls,
		metrics.includedSourceRereads,
		metrics.contextMillis,
		logPath,
	)
	file, err := os.OpenFile(
		filepath.Join(state.config.Output, "summary.tsv"),
		os.O_WRONLY|os.O_APPEND,
		0o600,
	)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(file, line); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (state *runnerState) prompt(
	benchmarkCase *regressionCase,
	query QueryVariant,
) string {
	return strings.TrimSpace(benchmarkCase.queries[query.ID]) +
		"\n\n" +
		strings.TrimSpace(string(state.instruction)) +
		"\n"
}

func (state *runnerState) hasBothPacks(caseID, queryID string) bool {
	_, golden := state.packs[packKey(caseID, queryID, "golden")]
	_, candidate := state.packs[packKey(caseID, queryID, "candidate")]
	return golden && candidate
}

func (state *runnerState) binary(build string) string {
	return filepath.Join(state.privateBins[build], "goregraph")
}

func (state *runnerState) rememberHash(caseID, build, kind, value string) error {
	key := hashKey(caseID, build, kind)
	if previous, exists := state.hashes[key]; exists && previous != value {
		return fmt.Errorf("%s %s %s hash changed across fresh runs", caseID, build, kind)
	}
	state.hashes[key] = value
	return nil
}

func hashKey(caseID, build, kind string) string {
	return strings.Join([]string{caseID, build, kind}, "\x00")
}

func packKey(caseID, queryID, build string) string {
	return strings.Join([]string{caseID, queryID, build}, "\x00")
}

func endToEndQuery(contract Contract) QueryVariant {
	for _, query := range contract.Queries {
		if query.EndToEnd {
			return query
		}
	}
	return QueryVariant{}
}

func decodeContextPack(body []byte) (agent.ContextPack, error) {
	var pack agent.ContextPack
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pack); err != nil {
		return agent.ContextPack{}, fmt.Errorf("decode Context Pack: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return agent.ContextPack{}, fmt.Errorf("decode Context Pack: trailing JSON value")
		}
		return agent.ContextPack{}, fmt.Errorf("decode Context Pack trailing data: %w", err)
	}
	if pack.Schema != 1 {
		return agent.ContextPack{}, fmt.Errorf(
			"decode Context Pack: schema %d is unsupported; expected 1",
			pack.Schema,
		)
	}
	return pack, nil
}

func environmentWithPath(front string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "PATH=") {
			continue
		}
		environment = append(environment, variable)
	}
	return append(environment, "PATH="+front+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runProcess(
	ctx context.Context,
	path string,
	args []string,
	stdin io.Reader,
	environment []string,
) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	command.Stdin = stdin
	if environment != nil {
		command.Env = environment
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func copyWorkspace(ctx context.Context, source, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	excluded := map[string]bool{
		".git": true, ".goregraph-workspace": true, "goregraph-out": true,
		"node_modules": true, "target": true, "build": true, "dist": true,
	}
	return fs.WalkDir(os.DirFS(source), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		if !safeRelativePath(path) {
			return fmt.Errorf("unsafe workspace path %q", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace symlink is not allowed: %s", path)
		}
		if entry.IsDir() && excluded[entry.Name()] {
			return fs.SkipDir
		}
		target := filepath.Join(destination, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.Mkdir(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace entry must be regular: %s", path)
		}
		return copyFileNewContext(
			ctx,
			filepath.Join(source, filepath.FromSlash(path)),
			target,
			0o600,
		)
	})
}

func hashTree(ctx context.Context, root string) (string, error) {
	hasher := sha256.New()
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("hash tree entry must be regular: %s", path)
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hasher, filepath.ToSlash(path))
		_, _ = hasher.Write([]byte{0})
		copyErr := copyWithContext(ctx, hasher, file)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		_, _ = hasher.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashFile(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if err := copyWithContext(ctx, hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashBytes(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func copyExecutable(ctx context.Context, source, destination string) error {
	return copyFileNewContext(ctx, source, destination, 0o700)
}

func copyFileNewContext(
	ctx context.Context,
	source string,
	destination string,
	mode os.FileMode,
) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	destinationFile, err := openFileNew(destination, mode)
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	copyErr := copyWithContext(ctx, destinationFile, sourceFile)
	return errors.Join(copyErr, sourceFile.Close(), destinationFile.Close())
}

func copyWithContext(
	ctx context.Context,
	destination io.Writer,
	source io.Reader,
) error {
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func writeJSONNew(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileNew(path, append(body, '\n'), 0o600)
}

func writeFileNew(path string, body []byte, mode os.FileMode) error {
	file, err := openFileNew(path, mode)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(body)
	if writeErr == nil && written != len(body) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Close())
}

func appendFile(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(body)
	if writeErr == nil && written != len(body) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Close())
}

func openFileNew(path string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
}
