package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ai-dev-manager/internal/adapter/runtimeadapter"
)

type GSDExecutionSpec struct {
	Steps        []PlanStep `json:"steps"`
	RunVerifiers bool       `json:"run_verifiers"`
}

func NewGSDWorkflowExecutor(builder RuntimeBuilder) (*WorkflowExecutor, error) {
	if builder == nil {
		return nil, errors.New("gsd workflow runtime builder is required")
	}
	return &WorkflowExecutor{
		name:      "gsd",
		builder:   builder,
		planner:   GSDPlanner{},
		executor:  RuntimeStepExecutor{},
		reviewer:  GSDReviewer{},
		completer: GSDAdvancer{},
	}, nil
}

type GSDPlanner struct{}

var (
	statePhasePattern         = regexp.MustCompile(`(?m)^Phase:\s*([0-9]+)\s+—`)
	statePlanPattern          = regexp.MustCompile(`(?m)^Plan:\s*([0-9]{2}-[0-9]{2})\s+—`)
	statePositionBlockPattern = regexp.MustCompile(`(?m)^Phase:\s*([0-9]+)\s+—\s*([^\r\n]+)\r?\nPlan:\s*([0-9]{2}-[0-9]{2})\s+—\s*([^\r\n]+)\r?\nStatus:\s*([^\r\n]+)`)
	planFilePattern           = regexp.MustCompile(`^([0-9]{2}-[0-9]{2})-PLAN\.md$`)
	planHeadingPattern        = regexp.MustCompile(`(?m)^#\s+Phase\s+([0-9]+)\s+Plan\s+([0-9]{2}-[0-9]{2})\s+—\s*([^\r\n]+)`)
	contextHeadingPattern     = regexp.MustCompile(`(?m)^#\s+Phase\s+([0-9]+)\s+Context\s+—\s*([^\r\n]+)`)
)

func (GSDPlanner) Plan(ctx context.Context, _ RunSpec, runtime runtimeadapter.Runtime) (Plan, error) {
	if err := requireCapabilities(runtime, runtimeadapter.OpRead, runtimeadapter.OpTree); err != nil {
		return Plan{}, err
	}
	const (
		projectPath = ".planning/PROJECT.md"
		statePath   = ".planning/STATE.md"
	)
	stateText, err := runtimeReadText(ctx, runtime, statePath)
	if err != nil {
		return Plan{}, fmt.Errorf("read GSD state: %w", err)
	}
	phase, planID, err := parseCurrentGSDPosition(stateText)
	if err != nil {
		return Plan{}, err
	}
	phaseDir, err := resolvePhaseDirectory(ctx, runtime, phase)
	if err != nil {
		return Plan{}, err
	}
	contextPath := filepath.Join(phaseDir, fmt.Sprintf("%02d-CONTEXT.md", phase))
	planPath := filepath.Join(phaseDir, planID+"-PLAN.md")
	if _, err := runtimeReadText(ctx, runtime, projectPath); err != nil {
		return Plan{}, fmt.Errorf("read GSD project: %w", err)
	}
	if _, err := runtimeReadText(ctx, runtime, contextPath); err != nil {
		return Plan{}, fmt.Errorf("read GSD context: %w", err)
	}
	planText, err := runtimeReadText(ctx, runtime, planPath)
	if err != nil {
		return Plan{}, fmt.Errorf("read GSD plan: %w", err)
	}
	spec, err := parseGSDExecutionSpec(planText)
	if err != nil {
		return Plan{}, err
	}
	steps, err := validateGSDExecutionSpec(runtime, spec)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Summary: fmt.Sprintf("Execute GSD phase %d plan %s from audited execution spec.", phase, planID),
		Planning: &PlanningSources{
			Phase:       phase,
			PlanID:      planID,
			ProjectPath: projectPath,
			StatePath:   statePath,
			ContextPath: filepath.Clean(contextPath),
			PlanPath:    filepath.Clean(planPath),
		},
		Steps: steps,
	}, nil
}

func requireCapabilities(runtime runtimeadapter.Runtime, capabilities ...string) error {
	available := make(map[string]struct{})
	for _, capability := range runtime.Capabilities() {
		available[capability] = struct{}{}
	}
	for _, capability := range capabilities {
		if _, ok := available[capability]; !ok {
			return fmt.Errorf("runtime capability %q is required", capability)
		}
	}
	return nil
}

func parseCurrentGSDPosition(state string) (int, string, error) {
	phaseMatch := statePhasePattern.FindStringSubmatch(state)
	planMatch := statePlanPattern.FindStringSubmatch(state)
	if len(phaseMatch) != 2 || len(planMatch) != 2 {
		return 0, "", errors.New("GSD STATE current phase/plan is missing or invalid")
	}
	phase, err := strconv.Atoi(phaseMatch[1])
	if err != nil || phase <= 0 {
		return 0, "", errors.New("GSD STATE phase number is invalid")
	}
	return phase, planMatch[1], nil
}

func resolvePhaseDirectory(ctx context.Context, runtime runtimeadapter.Runtime, phase int) (string, error) {
	output, err := runtime.Invoke(ctx, runtimeadapter.OpTree, map[string]any{
		"path": ".planning/phases", "max_depth": 1, "max_entries": 500,
	})
	if err != nil {
		return "", fmt.Errorf("list GSD phases: %w", err)
	}
	rows, err := decodeObjectRows(output, "GSD phase tree")
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("%02d-", phase)
	matches := make([]string, 0, 1)
	for _, row := range rows {
		path := jsonStringField(row, "path")
		if !jsonBoolField(row, "isdir", "is_dir") || path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if strings.HasPrefix(filepath.Base(clean), prefix) {
			matches = append(matches, clean)
		}
	}
	sort.Strings(matches)
	if len(matches) != 1 {
		return "", fmt.Errorf("GSD phase %02d directory resolution found %d matches", phase, len(matches))
	}
	return matches[0], nil
}

func runtimeReadText(ctx context.Context, runtime runtimeadapter.Runtime, path string) (string, error) {
	output, err := runtime.Invoke(ctx, runtimeadapter.OpRead, map[string]any{"path": path})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return "", err
	}
	for key, raw := range row {
		if strings.EqualFold(key, "content") {
			content, ok := raw.(string)
			if !ok {
				return "", fmt.Errorf("read %s returned non-string content", path)
			}
			return content, nil
		}
	}
	return "", fmt.Errorf("read %s returned no content", path)
}

func parseGSDExecutionSpec(planText string) (GSDExecutionSpec, error) {
	heading := strings.Index(planText, "## Execution Spec")
	if heading < 0 {
		return GSDExecutionSpec{}, errors.New("GSD PLAN is missing ## Execution Spec")
	}
	remainder := planText[heading+len("## Execution Spec"):]
	start := strings.Index(remainder, "```json")
	if start < 0 {
		return GSDExecutionSpec{}, errors.New("GSD Execution Spec is missing json fence")
	}
	remainder = remainder[start+len("```json"):]
	end := strings.Index(remainder, "```")
	if end < 0 {
		return GSDExecutionSpec{}, errors.New("GSD Execution Spec json fence is not closed")
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(remainder[:end])))
	decoder.DisallowUnknownFields()
	var spec GSDExecutionSpec
	if err := decoder.Decode(&spec); err != nil {
		return GSDExecutionSpec{}, fmt.Errorf("decode GSD Execution Spec: %w", err)
	}
	if len(spec.Steps) == 0 && !spec.RunVerifiers {
		return GSDExecutionSpec{}, errors.New("GSD Execution Spec has no steps or verifiers")
	}
	return spec, nil
}

func validateGSDExecutionSpec(runtime runtimeadapter.Runtime, spec GSDExecutionSpec) ([]PlanStep, error) {
	allowed := map[string]string{
		runtimeadapter.OpRead:            runtimeadapter.OpRead,
		runtimeadapter.OpSearch:          runtimeadapter.OpSearch,
		runtimeadapter.OpWrite:           runtimeadapter.OpWrite,
		runtimeadapter.OpEdit:            runtimeadapter.OpEdit,
		runtimeadapter.OpGitStatus:       runtimeadapter.OpGitStatus,
		runtimeadapter.OpGitDiff:         runtimeadapter.OpGitDiff,
		runtimeadapter.OpVerifierRunMany: "verify.run",
	}
	capabilities := make(map[string]struct{})
	for _, capability := range runtime.Capabilities() {
		capabilities[capability] = struct{}{}
	}
	seen := make(map[string]struct{})
	steps := make([]PlanStep, 0, len(spec.Steps)+1)
	hasVerifier := false
	for _, step := range spec.Steps {
		step.ID = strings.TrimSpace(step.ID)
		step.Operation = strings.TrimSpace(step.Operation)
		if step.ID == "" || step.Operation == "" {
			return nil, errors.New("GSD Execution Spec step id and operation are required")
		}
		if _, exists := seen[step.ID]; exists {
			return nil, fmt.Errorf("GSD Execution Spec duplicate step id %q", step.ID)
		}
		seen[step.ID] = struct{}{}
		capability, ok := allowed[step.Operation]
		if !ok {
			return nil, fmt.Errorf("GSD operation %q is not allowed", step.Operation)
		}
		if _, ok := capabilities[capability]; !ok {
			return nil, fmt.Errorf("runtime capability %q is required for GSD operation %q", capability, step.Operation)
		}
		if step.Operation == runtimeadapter.OpVerifierRunMany {
			hasVerifier = true
		}
		step.Input = cloneMap(step.Input)
		steps = append(steps, step)
	}
	if spec.RunVerifiers && !hasVerifier {
		if _, ok := capabilities["verify.run"]; !ok {
			return nil, errors.New(`runtime capability "verify.run" is required for GSD verifiers`)
		}
		steps = append(steps, PlanStep{
			ID:        "gsd-verifiers",
			Operation: runtimeadapter.OpVerifierRunMany,
			Purpose:   "Run all configured verifiers after GSD execution.",
			Input:     map[string]any{"ids": []string{}},
		})
	}
	return steps, nil
}

type GSDReviewer struct{}

func (GSDReviewer) Review(ctx context.Context, _ RunSpec, _ Plan, results []StepResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, err
	}
	for _, result := range results {
		if result.Error != "" {
			return ReviewResult{}, fmt.Errorf("step %s failed before review: %s", result.StepID, result.Error)
		}
	}
	for _, result := range results {
		if result.Operation != runtimeadapter.OpVerifierRunMany {
			continue
		}
		failed, count, err := verifierFailureSummary(result.Output)
		if err != nil {
			return ReviewResult{}, err
		}
		if failed > 0 {
			return ReviewResult{Decision: ReviewFail, Summary: fmt.Sprintf("%d of %d GSD verifiers failed", failed, count)}, nil
		}
		return ReviewResult{Decision: ReviewPass, Summary: fmt.Sprintf("GSD execution completed and all %d verifiers passed or were skipped", count)}, nil
	}
	return ReviewResult{Decision: ReviewPass, Summary: "GSD execution completed without a verifier step"}, nil
}

type GSDAdvancer struct{}

type gsdPosition struct {
	Phase      int
	PhaseTitle string
	PlanID     string
	PlanTitle  string
	Status     string
	Block      string
}

type gsdTarget struct {
	Phase      int
	PhaseTitle string
	PlanID     string
	PlanTitle  string
}

func (GSDAdvancer) Complete(ctx context.Context, _ RunSpec, runtime runtimeadapter.Runtime, plan Plan, review ReviewResult) (AdvanceResult, error) {
	result := AdvanceResult{Status: AdvanceSkipped}
	if plan.Planning != nil {
		result.FromPhase = plan.Planning.Phase
		result.FromPlan = plan.Planning.PlanID
	}
	if review.Decision != ReviewPass {
		result.Reason = "review did not pass"
		return result, nil
	}
	if plan.Planning == nil {
		return AdvanceResult{}, errors.New("GSD advance requires planning provenance")
	}
	stateText, err := runtimeReadText(ctx, runtime, plan.Planning.StatePath)
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("read GSD state for advance: %w", err)
	}
	position, err := parseGSDPositionBlock(stateText)
	if err != nil {
		return AdvanceResult{}, err
	}
	if position.Phase != plan.Planning.Phase || position.PlanID != plan.Planning.PlanID {
		return AdvanceResult{}, fmt.Errorf("GSD STATE moved during run: expected phase %d plan %s, found phase %d plan %s", plan.Planning.Phase, plan.Planning.PlanID, position.Phase, position.PlanID)
	}
	target, reason, err := findNextGSDTarget(ctx, runtime, position, plan.Planning.ContextPath)
	if err != nil {
		return AdvanceResult{}, err
	}
	if target == nil {
		return AdvanceResult{Status: AdvanceBlocked, FromPhase: position.Phase, FromPlan: position.PlanID, Reason: reason}, nil
	}
	if !hasCapability(runtime, runtimeadapter.OpEdit) {
		return AdvanceResult{
			Status: AdvanceBlocked, FromPhase: position.Phase, FromPlan: position.PlanID,
			ToPhase: target.Phase, ToPlan: target.PlanID,
			Reason: `runtime capability "files.edit" is required to advance GSD STATE`,
		}, nil
	}
	newline := "\n"
	if strings.Contains(position.Block, "\r\n") {
		newline = "\r\n"
	}
	newBlock := fmt.Sprintf("Phase: %d — %s%sPlan: %s — %s%sStatus: In Progress", target.Phase, target.PhaseTitle, newline, target.PlanID, target.PlanTitle, newline)
	_, err = runtime.Invoke(ctx, runtimeadapter.OpEdit, map[string]any{
		"path":                  plan.Planning.StatePath,
		"old_text":              position.Block,
		"new_text":              newBlock,
		"expected_replacements": 1,
	})
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("advance GSD STATE to phase %d plan %s: %w", target.Phase, target.PlanID, err)
	}
	return AdvanceResult{
		Status: AdvanceAdvanced, FromPhase: position.Phase, FromPlan: position.PlanID,
		ToPhase: target.Phase, ToPlan: target.PlanID,
		Reason: "advanced to the next pre-existing executable GSD plan after review pass",
	}, nil
}

func parseGSDPositionBlock(state string) (gsdPosition, error) {
	match := statePositionBlockPattern.FindStringSubmatch(state)
	if len(match) != 6 {
		return gsdPosition{}, errors.New("GSD STATE Current Position block is missing or invalid")
	}
	phase, err := strconv.Atoi(match[1])
	if err != nil || phase <= 0 {
		return gsdPosition{}, errors.New("GSD STATE phase number is invalid")
	}
	return gsdPosition{
		Phase: phase, PhaseTitle: strings.TrimSpace(match[2]), PlanID: strings.TrimSpace(match[3]),
		PlanTitle: strings.TrimSpace(match[4]), Status: strings.TrimSpace(match[5]), Block: match[0],
	}, nil
}

func findNextGSDTarget(ctx context.Context, runtime runtimeadapter.Runtime, position gsdPosition, currentContextPath string) (*gsdTarget, string, error) {
	phaseDir := filepath.Dir(filepath.Clean(currentContextPath))
	planIDs, err := listGSDPlanIDs(ctx, runtime, phaseDir, position.Phase)
	if err != nil {
		return nil, "", err
	}
	for _, planID := range planIDs {
		if planID <= position.PlanID {
			continue
		}
		planPath := filepath.Join(phaseDir, planID+"-PLAN.md")
		planText, err := runtimeReadText(ctx, runtime, planPath)
		if err != nil {
			return nil, "", fmt.Errorf("read next GSD plan %s: %w", planID, err)
		}
		planTitle, err := parsePlanHeading(planText, position.Phase, planID)
		if err != nil {
			return nil, "", err
		}
		return &gsdTarget{Phase: position.Phase, PhaseTitle: position.PhaseTitle, PlanID: planID, PlanTitle: planTitle}, "", nil
	}

	nextPhase := position.Phase + 1
	nextDir, reason, err := tryResolvePhaseDirectory(ctx, runtime, nextPhase)
	if err != nil {
		return nil, "", err
	}
	if nextDir == "" {
		return nil, reason, nil
	}
	contextPath := filepath.Join(nextDir, fmt.Sprintf("%02d-CONTEXT.md", nextPhase))
	contextText, err := runtimeReadText(ctx, runtime, contextPath)
	if err != nil {
		return nil, fmt.Sprintf("next phase %02d context is missing or unreadable", nextPhase), nil
	}
	phaseTitle, err := parseContextHeading(contextText, nextPhase)
	if err != nil {
		return nil, err.Error(), nil
	}
	nextPlans, err := listGSDPlanIDs(ctx, runtime, nextDir, nextPhase)
	if err != nil {
		return nil, "", err
	}
	if len(nextPlans) == 0 {
		return nil, fmt.Sprintf("next phase %02d has no executable plan", nextPhase), nil
	}
	planID := nextPlans[0]
	planText, err := runtimeReadText(ctx, runtime, filepath.Join(nextDir, planID+"-PLAN.md"))
	if err != nil {
		return nil, fmt.Sprintf("next phase plan %s is missing or unreadable", planID), nil
	}
	planTitle, err := parsePlanHeading(planText, nextPhase, planID)
	if err != nil {
		return nil, err.Error(), nil
	}
	return &gsdTarget{Phase: nextPhase, PhaseTitle: phaseTitle, PlanID: planID, PlanTitle: planTitle}, "", nil
}

func listGSDPlanIDs(ctx context.Context, runtime runtimeadapter.Runtime, phaseDir string, phase int) ([]string, error) {
	output, err := runtime.Invoke(ctx, runtimeadapter.OpTree, map[string]any{"path": phaseDir, "max_depth": 1, "max_entries": 500})
	if err != nil {
		return nil, fmt.Errorf("list GSD plans in %s: %w", phaseDir, err)
	}
	rows, err := decodeObjectRows(output, "GSD plan tree")
	if err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("%02d-", phase)
	ids := make([]string, 0)
	for _, row := range rows {
		if jsonBoolField(row, "isdir", "is_dir") {
			continue
		}
		path := jsonStringField(row, "path")
		if path == "" {
			continue
		}
		base := filepath.Base(filepath.Clean(path))
		match := planFilePattern.FindStringSubmatch(base)
		if len(match) != 2 || !strings.HasPrefix(match[1], prefix) {
			continue
		}
		ids = append(ids, match[1])
	}
	sort.Strings(ids)
	return ids, nil
}

func tryResolvePhaseDirectory(ctx context.Context, runtime runtimeadapter.Runtime, phase int) (string, string, error) {
	output, err := runtime.Invoke(ctx, runtimeadapter.OpTree, map[string]any{"path": ".planning/phases", "max_depth": 1, "max_entries": 500})
	if err != nil {
		return "", "", fmt.Errorf("list GSD phases: %w", err)
	}
	rows, err := decodeObjectRows(output, "GSD phase tree")
	if err != nil {
		return "", "", err
	}
	prefix := fmt.Sprintf("%02d-", phase)
	matches := make([]string, 0, 1)
	for _, row := range rows {
		if !jsonBoolField(row, "isdir", "is_dir") {
			continue
		}
		path := filepath.Clean(jsonStringField(row, "path"))
		if path != "." && strings.HasPrefix(filepath.Base(path), prefix) {
			matches = append(matches, path)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Sprintf("next phase %02d planning directory does not exist", phase), nil
	case 1:
		return matches[0], "", nil
	default:
		return "", fmt.Sprintf("next phase %02d planning directory is ambiguous (%d matches)", phase, len(matches)), nil
	}
}

func parsePlanHeading(text string, phase int, planID string) (string, error) {
	match := planHeadingPattern.FindStringSubmatch(text)
	if len(match) != 4 {
		return "", fmt.Errorf("GSD plan %s heading is missing or invalid", planID)
	}
	gotPhase, _ := strconv.Atoi(match[1])
	if gotPhase != phase || match[2] != planID {
		return "", fmt.Errorf("GSD plan heading identity mismatch: expected phase %d plan %s", phase, planID)
	}
	return strings.TrimSpace(match[3]), nil
}

func parseContextHeading(text string, phase int) (string, error) {
	match := contextHeadingPattern.FindStringSubmatch(text)
	if len(match) != 3 {
		return "", fmt.Errorf("GSD phase %02d context heading is missing or invalid", phase)
	}
	gotPhase, _ := strconv.Atoi(match[1])
	if gotPhase != phase {
		return "", fmt.Errorf("GSD context heading identity mismatch: expected phase %d", phase)
	}
	return strings.TrimSpace(match[2]), nil
}

func hasCapability(runtime runtimeadapter.Runtime, capability string) bool {
	for _, available := range runtime.Capabilities() {
		if available == capability {
			return true
		}
	}
	return false
}

func decodeObjectRows(value any, label string) ([]map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return rows, nil
}

func jsonStringField(row map[string]any, names ...string) string {
	for key, raw := range row {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				value, _ := raw.(string)
				return value
			}
		}
	}
	return ""
}

func jsonBoolField(row map[string]any, names ...string) bool {
	for key, raw := range row {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				value, _ := raw.(bool)
				return value
			}
		}
	}
	return false
}
