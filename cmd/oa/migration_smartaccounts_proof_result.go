package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/cutover"
)

const (
	smartAccountsProofStatusPassed  = "passed"
	smartAccountsProofStatusBlocked = "blocked"
)

type smartAccountsProofResult struct {
	Provider    cutover.MigrationProviderPreset `json:"provider"`
	GeneratedAt string                          `json:"generated_at,omitempty"`
	PlanPath    string                          `json:"plan_path"`
	Status      string                          `json:"status"`
	Reviewer    string                          `json:"reviewer"`
	ReviewedAt  string                          `json:"reviewed_at"`
	Items       []smartAccountsProofResultItem  `json:"items"`
	Summary     smartAccountsProofResultSummary `json:"summary,omitempty"`
}

type smartAccountsProofResultItem struct {
	Area                       string `json:"area"`
	Status                     string `json:"status"`
	SmartAccountsArtifact      string `json:"smartaccounts_artifact"`
	SmartAccountsSHA256        string `json:"smartaccounts_sha256"`
	OpenAccountingArtifact     string `json:"open_accounting_artifact"`
	OpenAccountingSHA256       string `json:"open_accounting_sha256"`
	Basis                      string `json:"basis,omitempty"`
	Period                     string `json:"period,omitempty"`
	Reviewer                   string `json:"reviewer,omitempty"`
	ReviewedAt                 string `json:"reviewed_at,omitempty"`
	DiscrepancyNote            string `json:"discrepancy_note,omitempty"`
	SmartAccountsArtifactSize  int64  `json:"smartaccounts_artifact_size,omitempty"`
	OpenAccountingArtifactSize int64  `json:"open_accounting_artifact_size,omitempty"`
}

type smartAccountsProofResultSummary struct {
	RequiredAreas int `json:"required_areas,omitempty"`
	PassedAreas   int `json:"passed_areas,omitempty"`
	Blockers      int `json:"blockers,omitempty"`
}

type smartAccountsProofResultValidation struct {
	Provider      cutover.MigrationProviderPreset `json:"provider"`
	PlanPath      string                          `json:"plan_path"`
	ResultPath    string                          `json:"result_path"`
	Ready         bool                            `json:"ready"`
	Status        string                          `json:"status"`
	CheckedAt     string                          `json:"checked_at"`
	RequiredAreas []string                        `json:"required_areas"`
	PassedAreas   []string                        `json:"passed_areas"`
	Blockers      []string                        `json:"blockers,omitempty"`
	NextAction    string                          `json:"next_action"`
}

func (a *cliApp) runMigrationSmartAccountsProofResult(args []string) error {
	fs := flag.NewFlagSet("migration smartaccounts-proof-result", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	planPath := fs.String("plan", "", "Private smartaccounts-proof-plan.json path")
	resultPath := fs.String("result", "", "Private SmartAccounts proof result JSON path")
	requireReady := fs.Bool("require-ready", false, "Return an error when private proof evidence is not ready")
	asJSON := fs.Bool("json", false, "Output proof result validation JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*planPath) == "" || strings.TrimSpace(*resultPath) == "" {
		return errors.New("plan and result are required")
	}

	planAbs, _ := filepath.Abs(strings.TrimSpace(*planPath))
	resultAbs, _ := filepath.Abs(strings.TrimSpace(*resultPath))
	if err := rejectSmartAccountsProofPublicWorktreePathWithLabel(planAbs, "plan"); err != nil {
		return err
	}
	if err := rejectSmartAccountsProofPublicWorktreePathWithLabel(resultAbs, "result"); err != nil {
		return err
	}

	planPayload, err := os.ReadFile(planAbs)
	if err != nil {
		return fmt.Errorf("read SmartAccounts proof plan: %w", err)
	}
	var plan smartAccountsProofPlan
	if err := json.Unmarshal(planPayload, &plan); err != nil {
		return fmt.Errorf("decode SmartAccounts proof plan: %w", err)
	}
	if plan.Provider != cutover.MigrationProviderPresetSmartAccounts {
		return fmt.Errorf("proof plan provider must be %q", cutover.MigrationProviderPresetSmartAccounts)
	}

	resultPayload, err := os.ReadFile(resultAbs)
	if err != nil {
		return fmt.Errorf("read SmartAccounts proof result: %w", err)
	}
	var result smartAccountsProofResult
	if err := json.Unmarshal(resultPayload, &result); err != nil {
		return fmt.Errorf("decode SmartAccounts proof result: %w", err)
	}

	validation := validateSmartAccountsProofResult(&plan, &result, planAbs, resultAbs)
	if *asJSON {
		if err := printJSON(a.stdout, validation); err != nil {
			return err
		}
	} else {
		printSmartAccountsProofResultValidation(a.stdout, validation)
	}
	if *requireReady && !validation.Ready {
		return fmt.Errorf("SmartAccounts proof result is not ready: %d blocker(s)", len(validation.Blockers))
	}
	return nil
}

func validateSmartAccountsProofResult(plan *smartAccountsProofPlan, result *smartAccountsProofResult, planPath, resultPath string) smartAccountsProofResultValidation {
	requiredAreas := smartAccountsProofPlanAreas(plan)
	validation := smartAccountsProofResultValidation{
		Provider:      cutover.MigrationProviderPresetSmartAccounts,
		PlanPath:      planPath,
		ResultPath:    resultPath,
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		RequiredAreas: requiredAreas,
		Status:        smartAccountsProofStatusBlocked,
		NextAction:    "Resolve private proof blockers, regenerate evidence if needed, and rerun this validator before claiming SmartAccounts parity.",
	}

	if len(requiredAreas) == 0 {
		validation.Blockers = append(validation.Blockers, "proof plan does not contain any parity areas")
	}
	if result.Provider != cutover.MigrationProviderPresetSmartAccounts {
		validation.Blockers = append(validation.Blockers, fmt.Sprintf("proof result provider must be %q", cutover.MigrationProviderPresetSmartAccounts))
	}
	if normalizeProofStatus(result.Status) != smartAccountsProofStatusPassed {
		validation.Blockers = append(validation.Blockers, "proof result status must be passed")
	}
	if strings.TrimSpace(result.PlanPath) == "" {
		validation.Blockers = append(validation.Blockers, "proof result plan_path is required")
	} else if filepath.IsAbs(result.PlanPath) {
		resultPlanAbs, _ := filepath.Abs(result.PlanPath)
		if filepath.Clean(resultPlanAbs) != filepath.Clean(planPath) {
			validation.Blockers = append(validation.Blockers, "proof result plan_path does not match --plan")
		}
	}

	resultReviewer := strings.TrimSpace(result.Reviewer)
	resultReviewedAt := strings.TrimSpace(result.ReviewedAt)
	if resultReviewer == "" {
		validation.Blockers = append(validation.Blockers, "proof result reviewer is required")
	}
	if resultReviewedAt == "" {
		validation.Blockers = append(validation.Blockers, "proof result reviewed_at is required")
	} else if !smartAccountsProofReviewedAtLooksValid(resultReviewedAt) {
		validation.Blockers = append(validation.Blockers, "proof result reviewed_at must be RFC3339 or YYYY-MM-DD")
	}

	byArea := map[string]smartAccountsProofResultItem{}
	seenAreas := map[string]bool{}
	for _, item := range result.Items {
		area := strings.TrimSpace(item.Area)
		if area == "" {
			validation.Blockers = append(validation.Blockers, "proof result item area is required")
			continue
		}
		if seenAreas[area] {
			validation.Blockers = append(validation.Blockers, "duplicate proof result item for area "+area)
			continue
		}
		seenAreas[area] = true
		byArea[area] = item
	}

	requiredSet := map[string]bool{}
	for _, area := range requiredAreas {
		requiredSet[area] = true
	}
	for area := range byArea {
		if !requiredSet[area] {
			validation.Blockers = append(validation.Blockers, "proof result contains unplanned area "+area)
		}
	}

	resultDir := filepath.Dir(resultPath)
	for _, area := range requiredAreas {
		item, ok := byArea[area]
		if !ok {
			validation.Blockers = append(validation.Blockers, "missing proof result item for area "+area)
			continue
		}
		itemBlockers := smartAccountsProofResultItemBlockers(area, item, resultReviewer, resultReviewedAt, resultDir)
		if len(itemBlockers) == 0 {
			validation.PassedAreas = append(validation.PassedAreas, area)
			continue
		}
		validation.Blockers = append(validation.Blockers, itemBlockers...)
	}
	sort.Strings(validation.Blockers)

	validation.Ready = len(validation.Blockers) == 0
	if validation.Ready {
		validation.Status = smartAccountsProofStatusPassed
		validation.NextAction = "Private proof evidence covers every planned SmartAccounts parity area; continue with accountant signoff and the cutover closeout gate."
	}
	return validation
}

func smartAccountsProofPlanAreas(plan *smartAccountsProofPlan) []string {
	areas := make([]string, 0, len(plan.Items))
	seen := map[string]bool{}
	for _, item := range plan.Items {
		area := strings.TrimSpace(item.Area)
		if area == "" || seen[area] {
			continue
		}
		seen[area] = true
		areas = append(areas, area)
	}
	return areas
}

func smartAccountsProofResultItemBlockers(area string, item smartAccountsProofResultItem, resultReviewer, resultReviewedAt, resultDir string) []string {
	var blockers []string
	if normalizeProofStatus(item.Status) != smartAccountsProofStatusPassed {
		blockers = append(blockers, fmt.Sprintf("%s status must be passed", area))
	}
	reviewer := smartAccountsProofFirstNonEmpty(item.Reviewer, resultReviewer)
	if reviewer == "" {
		blockers = append(blockers, fmt.Sprintf("%s reviewer is required", area))
	}
	reviewedAt := smartAccountsProofFirstNonEmpty(item.ReviewedAt, resultReviewedAt)
	if reviewedAt == "" {
		blockers = append(blockers, fmt.Sprintf("%s reviewed_at is required", area))
	} else if !smartAccountsProofReviewedAtLooksValid(reviewedAt) {
		blockers = append(blockers, fmt.Sprintf("%s reviewed_at must be RFC3339 or YYYY-MM-DD", area))
	}
	blockers = append(blockers, smartAccountsProofArtifactBlockers(area, "SmartAccounts", item.SmartAccountsArtifact, item.SmartAccountsSHA256, resultDir)...)
	blockers = append(blockers, smartAccountsProofArtifactBlockers(area, "Open Accounting", item.OpenAccountingArtifact, item.OpenAccountingSHA256, resultDir)...)
	return blockers
}

func smartAccountsProofArtifactBlockers(area, label, artifactPath, expectedSHA, resultDir string) []string {
	var blockers []string
	trimmedPath := strings.TrimSpace(artifactPath)
	trimmedSHA := strings.ToLower(strings.TrimSpace(expectedSHA))
	if trimmedPath == "" {
		blockers = append(blockers, fmt.Sprintf("%s %s artifact path is required", area, label))
	}
	if trimmedSHA == "" {
		blockers = append(blockers, fmt.Sprintf("%s %s artifact SHA-256 is required", area, label))
	} else if !smartAccountsProofLooksSHA256(trimmedSHA) {
		blockers = append(blockers, fmt.Sprintf("%s %s artifact SHA-256 must be 64 hex characters", area, label))
	}
	if trimmedPath == "" {
		return blockers
	}
	resolvedPath := smartAccountsProofResolveArtifactPath(resultDir, trimmedPath)
	candidatePaths := []string{resolvedPath}
	if realPath, err := filepath.EvalSymlinks(resolvedPath); err == nil {
		candidatePaths = append(candidatePaths, realPath)
	}
	for _, candidatePath := range uniqueNonEmptyStrings(candidatePaths) {
		if root, ok := smartAccountsProofOpenAccountingRootForPath(candidatePath); ok && pathWithin(candidatePath, root) {
			blockers = append(blockers, fmt.Sprintf("%s %s artifact must not be inside public Open Accounting Git worktree %s", area, label, root))
			break
		}
	}
	if trimmedSHA == "" || !smartAccountsProofLooksSHA256(trimmedSHA) {
		return blockers
	}
	actualSHA, err := smartAccountsProofFileSHA256(resolvedPath)
	if err != nil {
		blockers = append(blockers, fmt.Sprintf("%s %s artifact hash check failed: %v", area, label, err))
		return blockers
	}
	if actualSHA != trimmedSHA {
		blockers = append(blockers, fmt.Sprintf("%s %s artifact SHA-256 mismatch", area, label))
	}
	return blockers
}

func smartAccountsProofResolveArtifactPath(baseDir, artifactPath string) string {
	if filepath.IsAbs(artifactPath) {
		return filepath.Clean(artifactPath)
	}
	absPath, _ := filepath.Abs(filepath.Join(baseDir, artifactPath))
	return absPath
}

func smartAccountsProofFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func smartAccountsProofLooksSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func smartAccountsProofReviewedAtLooksValid(value string) bool {
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	return false
}

func normalizeProofStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func printSmartAccountsProofResultValidation(w io.Writer, validation smartAccountsProofResultValidation) {
	_, _ = fmt.Fprintln(w, "SmartAccounts proof result validation")
	_, _ = fmt.Fprintf(w, "Plan: %s\n", validation.PlanPath)
	_, _ = fmt.Fprintf(w, "Result: %s\n", validation.ResultPath)
	_, _ = fmt.Fprintf(w, "Ready: %t\n", validation.Ready)
	_, _ = fmt.Fprintf(w, "Status: %s\n", validation.Status)
	_, _ = fmt.Fprintf(w, "Areas: passed=%d required=%d\n", len(validation.PassedAreas), len(validation.RequiredAreas))
	if len(validation.Blockers) > 0 {
		_, _ = fmt.Fprintf(w, "Blockers: %d\n", len(validation.Blockers))
		for _, blocker := range validation.Blockers {
			_, _ = fmt.Fprintf(w, "- %s\n", blocker)
		}
	}
	_, _ = fmt.Fprintf(w, "Next: %s\n", validation.NextAction)
}
