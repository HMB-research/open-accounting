package cutover

import (
	"strconv"
	"time"
)

type MigrationExecutionResultStatus string

const (
	MigrationExecutionResultPlanned   MigrationExecutionResultStatus = "PLANNED"
	MigrationExecutionResultRunning   MigrationExecutionResultStatus = "RUNNING"
	MigrationExecutionResultSkipped   MigrationExecutionResultStatus = "SKIPPED"
	MigrationExecutionResultSucceeded MigrationExecutionResultStatus = "SUCCEEDED"
	MigrationExecutionResultFailed    MigrationExecutionResultStatus = "FAILED"
)

type ExecuteMigrationRequest struct {
	Files                    []BundleFile            `json:"files"`
	EInvoiceContactMode      EInvoiceContactMode     `json:"e_invoice_contact_mode,omitempty"`
	EInvoiceInvoiceType      string                  `json:"e_invoice_invoice_type,omitempty"`
	ProviderPreset           MigrationProviderPreset `json:"provider_preset,omitempty"`
	BankTransactionAccountID string                  `json:"bank_transaction_account_id,omitempty"`
	BankTransactionFormat    string                  `json:"bank_transaction_format,omitempty"`
	OpeningBalanceEntryDate  string                  `json:"opening_balance_entry_date,omitempty"`
	PostJournalEntries       bool                    `json:"post_journal_entries,omitempty"`
	Confirm                  bool                    `json:"confirm,omitempty"`
	ResumeFromRun            *MigrationExecutionRun  `json:"resume_from_run,omitempty"`
	ResumeFromRunID          string                  `json:"resume_from_run_id,omitempty"`
}

// NewStoredMigrationExecutionRequest returns the replay-safe subset of an execution request.
func NewStoredMigrationExecutionRequest(req *ExecuteMigrationRequest) *ExecuteMigrationRequest {
	if req == nil {
		return nil
	}
	return &ExecuteMigrationRequest{
		Files:                    cloneMigrationBundleFiles(req.Files),
		EInvoiceContactMode:      req.EInvoiceContactMode,
		EInvoiceInvoiceType:      req.EInvoiceInvoiceType,
		ProviderPreset:           req.ProviderPreset,
		BankTransactionAccountID: req.BankTransactionAccountID,
		BankTransactionFormat:    req.BankTransactionFormat,
		OpeningBalanceEntryDate:  req.OpeningBalanceEntryDate,
		PostJournalEntries:       req.PostJournalEntries,
	}
}

// MergeSavedMigrationExecutionRequest fills a resume request from the saved run payload when fields are omitted.
func MergeSavedMigrationExecutionRequest(req *ExecuteMigrationRequest, saved *ExecuteMigrationRequest) {
	if req == nil || saved == nil {
		return
	}
	hasRequestFiles := len(req.Files) > 0
	if len(req.Files) == 0 && len(saved.Files) > 0 {
		req.Files = cloneMigrationBundleFiles(saved.Files)
	}
	if req.EInvoiceContactMode == "" {
		req.EInvoiceContactMode = saved.EInvoiceContactMode
	}
	if req.EInvoiceInvoiceType == "" {
		req.EInvoiceInvoiceType = saved.EInvoiceInvoiceType
	}
	if req.ProviderPreset == "" {
		req.ProviderPreset = saved.ProviderPreset
	}
	if req.BankTransactionAccountID == "" {
		req.BankTransactionAccountID = saved.BankTransactionAccountID
	}
	if req.BankTransactionFormat == "" {
		req.BankTransactionFormat = saved.BankTransactionFormat
	}
	if req.OpeningBalanceEntryDate == "" {
		req.OpeningBalanceEntryDate = saved.OpeningBalanceEntryDate
	}
	if !hasRequestFiles && !req.PostJournalEntries {
		req.PostJournalEntries = saved.PostJournalEntries
	}
}

func cloneMigrationBundleFiles(files []BundleFile) []BundleFile {
	if len(files) == 0 {
		return nil
	}
	cloned := make([]BundleFile, len(files))
	copy(cloned, files)
	return cloned
}

func (r ExecuteMigrationRequest) PlanRequest() *PlanMigrationExecutionRequest {
	return &PlanMigrationExecutionRequest{
		Files:                    r.Files,
		EInvoiceContactMode:      r.EInvoiceContactMode,
		EInvoiceInvoiceType:      r.EInvoiceInvoiceType,
		ProviderPreset:           r.ProviderPreset,
		BankTransactionAccountID: r.BankTransactionAccountID,
		BankTransactionFormat:    r.BankTransactionFormat,
		OpeningBalanceEntryDate:  r.OpeningBalanceEntryDate,
		PostJournalEntries:       r.PostJournalEntries,
	}
}

type MigrationExecutionRun struct {
	ID                 string                       `json:"id,omitempty"`
	TenantID           string                       `json:"tenant_id,omitempty"`
	CreatedBy          string                       `json:"created_by,omitempty"`
	CreatedAt          *time.Time                   `json:"created_at,omitempty"`
	UpdatedAt          *time.Time                   `json:"updated_at,omitempty"`
	Summary            MigrationExecutionRunSummary `json:"summary"`
	Plan               *MigrationExecutionPlan      `json:"plan,omitempty"`
	Steps              []MigrationExecutionStepRun  `json:"steps,omitempty"`
	RemediationActions []MigrationRemediationAction `json:"remediation_actions,omitempty"`
	ExecutionRequest   *ExecuteMigrationRequest     `json:"-"`
}

type MigrationExecutionRunEvent struct {
	Type     string                 `json:"type"`
	Sequence int                    `json:"sequence"`
	Run      *MigrationExecutionRun `json:"run,omitempty"`
}

type MigrationExecutionRunSummary struct {
	Status                string                         `json:"status"`
	Confirmed             bool                           `json:"confirmed"`
	Resumed               bool                           `json:"resumed"`
	PlanReady             bool                           `json:"plan_ready"`
	ValidationReady       bool                           `json:"validation_ready"`
	StepCount             int                            `json:"step_count"`
	RunningStepCount      int                            `json:"running_step_count"`
	SucceededStepCount    int                            `json:"succeeded_step_count"`
	FailedStepCount       int                            `json:"failed_step_count"`
	SkippedStepCount      int                            `json:"skipped_step_count"`
	PlannedStepCount      int                            `json:"planned_step_count"`
	ResumedStepCount      int                            `json:"resumed_step_count"`
	CompletedStepCount    int                            `json:"completed_step_count"`
	RemainingStepCount    int                            `json:"remaining_step_count"`
	ProgressPercent       int                            `json:"progress_percent"`
	DurationMS            int64                          `json:"duration_ms,omitempty"`
	NeedsContextCount     int                            `json:"needs_context_count"`
	BlockedStepCount      int                            `json:"blocked_step_count"`
	ActiveStepNumber      int                            `json:"active_step_number,omitempty"`
	ActiveStepKind        FileKind                       `json:"active_step_kind,omitempty"`
	ActiveStepFileName    string                         `json:"active_step_file_name,omitempty"`
	ActiveStepStatus      MigrationExecutionResultStatus `json:"active_step_status,omitempty"`
	ActiveStepStartedAt   *time.Time                     `json:"active_step_started_at,omitempty"`
	ActiveStepCompletedAt *time.Time                     `json:"active_step_completed_at,omitempty"`
	ActiveStepDurationMS  int64                          `json:"active_step_duration_ms,omitempty"`
}

type MigrationExecutionStepRun struct {
	StepNumber  int                            `json:"step_number"`
	Kind        FileKind                       `json:"kind"`
	FileName    string                         `json:"file_name"`
	Status      MigrationExecutionResultStatus `json:"status"`
	Message     string                         `json:"message,omitempty"`
	Error       string                         `json:"error,omitempty"`
	APIPath     string                         `json:"api_path,omitempty"`
	CLICommand  string                         `json:"cli_command,omitempty"`
	Response    interface{}                    `json:"response,omitempty"`
	StartedAt   *time.Time                     `json:"started_at,omitempty"`
	CompletedAt *time.Time                     `json:"completed_at,omitempty"`
	DurationMS  int64                          `json:"duration_ms,omitempty"`
}

func NewMigrationExecutionRun(plan *MigrationExecutionPlan, confirmed bool) *MigrationExecutionRun {
	run := &MigrationExecutionRun{
		Summary: MigrationExecutionRunSummary{
			Status:    "blocked",
			Confirmed: confirmed,
		},
		Plan: plan,
	}
	if plan == nil {
		return run
	}
	run.RemediationActions = plan.RemediationActions
	run.Summary.PlanReady = plan.Summary.Ready
	run.Summary.ValidationReady = plan.Summary.ValidationReady
	run.Summary.NeedsContextCount = plan.Summary.NeedsContextCount
	run.Summary.BlockedStepCount = plan.Summary.BlockedStepCount
	if plan.Summary.Ready && confirmed {
		run.Summary.Status = "running"
	} else if plan.Summary.Ready {
		run.Summary.Status = "needs_confirmation"
	}

	run.Steps = make([]MigrationExecutionStepRun, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		status := MigrationExecutionResultPlanned
		message := "Pass confirm=true to run this import."
		if confirmed {
			message = "Ready to import."
		}
		if step.Status != MigrationExecutionStepReady {
			status = MigrationExecutionResultSkipped
			message = step.Message
		}
		run.Steps = append(run.Steps, MigrationExecutionStepRun{
			StepNumber: step.StepNumber,
			Kind:       step.Kind,
			FileName:   step.FileName,
			Status:     status,
			Message:    message,
			APIPath:    step.APIPath,
			CLICommand: step.CLICommand,
		})
	}
	RefreshMigrationExecutionRunProgress(run)
	return run
}

func NewResumableMigrationExecutionRun(plan *MigrationExecutionPlan, confirmed bool, resumeFrom *MigrationExecutionRun) *MigrationExecutionRun {
	run := NewMigrationExecutionRun(plan, confirmed)
	ApplyMigrationExecutionResume(run, resumeFrom)
	return run
}

func ApplyMigrationExecutionResume(run *MigrationExecutionRun, resumeFrom *MigrationExecutionRun) {
	if run == nil || resumeFrom == nil {
		return
	}

	succeededSteps := make(map[string]MigrationExecutionStepRun, len(resumeFrom.Steps))
	for _, step := range resumeFrom.Steps {
		if step.Status != MigrationExecutionResultSucceeded {
			continue
		}
		succeededSteps[migrationExecutionRunStepKey(step.StepNumber, step.Kind, step.FileName)] = step
	}
	if len(succeededSteps) == 0 {
		return
	}

	resumedCount := 0
	for index := range run.Steps {
		current := &run.Steps[index]
		if current.Status != MigrationExecutionResultPlanned {
			continue
		}
		previous, ok := succeededSteps[migrationExecutionRunStepKey(current.StepNumber, current.Kind, current.FileName)]
		if !ok {
			continue
		}
		current.Status = MigrationExecutionResultSucceeded
		current.Message = "Step already succeeded in the previous run; skipping on resume."
		current.Response = previous.Response
		current.StartedAt = previous.StartedAt
		current.CompletedAt = previous.CompletedAt
		current.DurationMS = previous.DurationMS
		resumedCount++
	}

	RefreshMigrationExecutionRunProgress(run)
	run.Summary.ResumedStepCount = resumedCount
	if run.Summary.ResumedStepCount == 0 {
		return
	}
	run.Summary.Resumed = true
	RefreshMigrationExecutionRunProgress(run)
}

func RefreshMigrationExecutionRunProgress(run *MigrationExecutionRun) {
	if run == nil {
		return
	}

	summary := &run.Summary
	if len(run.Steps) > 0 {
		summary.StepCount = len(run.Steps)
		summary.RunningStepCount = 0
		summary.SucceededStepCount = 0
		summary.FailedStepCount = 0
		summary.SkippedStepCount = 0
		summary.PlannedStepCount = 0
		summary.ActiveStepNumber = 0
		summary.ActiveStepKind = ""
		summary.ActiveStepFileName = ""
		summary.ActiveStepStatus = ""
		summary.ActiveStepStartedAt = nil
		summary.ActiveStepCompletedAt = nil
		summary.ActiveStepDurationMS = 0
		summary.DurationMS = 0

		activePriority := 0
		for index := range run.Steps {
			step := &run.Steps[index]
			if step.DurationMS == 0 {
				step.DurationMS = migrationExecutionStepDurationMS(step.StartedAt, step.CompletedAt)
			}
			if step.DurationMS > 0 {
				summary.DurationMS += step.DurationMS
			}
			switch step.Status {
			case MigrationExecutionResultRunning:
				summary.RunningStepCount++
				activePriority = setMigrationExecutionActiveStep(summary, step, activePriority, 3)
			case MigrationExecutionResultFailed:
				summary.FailedStepCount++
				activePriority = setMigrationExecutionActiveStep(summary, step, activePriority, 2)
			case MigrationExecutionResultPlanned:
				summary.PlannedStepCount++
				activePriority = setMigrationExecutionActiveStep(summary, step, activePriority, 1)
			case MigrationExecutionResultSkipped:
				summary.SkippedStepCount++
			case MigrationExecutionResultSucceeded:
				summary.SucceededStepCount++
			}
		}
	}

	summary.CompletedStepCount = summary.SucceededStepCount + summary.FailedStepCount
	summary.RemainingStepCount = summary.StepCount - summary.CompletedStepCount
	if summary.RemainingStepCount < 0 {
		summary.RemainingStepCount = 0
	}
	if summary.StepCount > 0 {
		summary.ProgressPercent = summary.CompletedStepCount * 100 / summary.StepCount
	} else {
		summary.ProgressPercent = 0
	}

	switch {
	case summary.FailedStepCount > 0:
		summary.Status = "failed"
	case summary.RunningStepCount > 0:
		summary.Status = "running"
	case summary.PlanReady && summary.StepCount > 0 && summary.SucceededStepCount == summary.StepCount:
		summary.Status = "succeeded"
	case summary.PlanReady && !summary.Confirmed:
		summary.Status = "needs_confirmation"
	case summary.PlanReady && summary.Confirmed && summary.PlannedStepCount > 0:
		summary.Status = "running"
	case !summary.ValidationReady || !summary.PlanReady || summary.NeedsContextCount > 0 || summary.BlockedStepCount > 0 || summary.SkippedStepCount > 0:
		summary.Status = "blocked"
	}
}

func setMigrationExecutionActiveStep(summary *MigrationExecutionRunSummary, step *MigrationExecutionStepRun, currentPriority, priority int) int {
	if priority <= currentPriority {
		return currentPriority
	}
	summary.ActiveStepNumber = step.StepNumber
	summary.ActiveStepKind = step.Kind
	summary.ActiveStepFileName = step.FileName
	summary.ActiveStepStatus = step.Status
	summary.ActiveStepStartedAt = step.StartedAt
	summary.ActiveStepCompletedAt = step.CompletedAt
	summary.ActiveStepDurationMS = step.DurationMS
	return priority
}

func MarkMigrationExecutionStepRunning(run *MigrationExecutionRun, index int, now time.Time) {
	step := migrationExecutionStepRunAt(run, index)
	if step == nil {
		return
	}
	startedAt := normalizeMigrationExecutionTime(now)
	if step.StartedAt == nil {
		step.StartedAt = &startedAt
	}
	step.CompletedAt = nil
	step.DurationMS = 0
	step.Status = MigrationExecutionResultRunning
	step.Message = "Import running."
	step.Error = ""
	RefreshMigrationExecutionRunProgress(run)
}

func CompleteMigrationExecutionStep(run *MigrationExecutionRun, index int, status MigrationExecutionResultStatus, message, errorText string, response interface{}, now time.Time) {
	step := migrationExecutionStepRunAt(run, index)
	if step == nil {
		return
	}
	completedAt := normalizeMigrationExecutionTime(now)
	if step.StartedAt == nil {
		startedAt := completedAt
		step.StartedAt = &startedAt
	}
	step.CompletedAt = &completedAt
	step.DurationMS = migrationExecutionStepDurationMS(step.StartedAt, step.CompletedAt)
	step.Status = status
	step.Message = message
	step.Error = errorText
	step.Response = response
	RefreshMigrationExecutionRunProgress(run)
}

func migrationExecutionStepRunAt(run *MigrationExecutionRun, index int) *MigrationExecutionStepRun {
	if run == nil || index < 0 || index >= len(run.Steps) {
		return nil
	}
	return &run.Steps[index]
}

func normalizeMigrationExecutionTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func migrationExecutionStepDurationMS(startedAt, completedAt *time.Time) int64 {
	if startedAt == nil || completedAt == nil {
		return 0
	}
	duration := completedAt.Sub(*startedAt)
	if duration < 0 {
		return 0
	}
	ms := duration.Milliseconds()
	if ms == 0 && duration > 0 {
		return 1
	}
	return ms
}

func migrationExecutionRunStepKey(stepNumber int, kind FileKind, fileName string) string {
	return string(kind) + "\x00" + fileName + "\x00" + strconv.Itoa(stepNumber)
}
