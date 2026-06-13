package cutover

import "strconv"

type MigrationExecutionResultStatus string

const (
	MigrationExecutionResultPlanned   MigrationExecutionResultStatus = "PLANNED"
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
	Confirm                  bool                    `json:"confirm,omitempty"`
	ResumeFromRun            *MigrationExecutionRun  `json:"resume_from_run,omitempty"`
}

func (r ExecuteMigrationRequest) PlanRequest() *PlanMigrationExecutionRequest {
	return &PlanMigrationExecutionRequest{
		Files:                    r.Files,
		EInvoiceContactMode:      r.EInvoiceContactMode,
		ProviderPreset:           r.ProviderPreset,
		BankTransactionAccountID: r.BankTransactionAccountID,
		OpeningBalanceEntryDate:  r.OpeningBalanceEntryDate,
	}
}

type MigrationExecutionRun struct {
	Summary            MigrationExecutionRunSummary `json:"summary"`
	Plan               *MigrationExecutionPlan      `json:"plan,omitempty"`
	Steps              []MigrationExecutionStepRun  `json:"steps,omitempty"`
	RemediationActions []MigrationRemediationAction `json:"remediation_actions,omitempty"`
}

type MigrationExecutionRunSummary struct {
	Status             string `json:"status"`
	Confirmed          bool   `json:"confirmed"`
	Resumed            bool   `json:"resumed"`
	PlanReady          bool   `json:"plan_ready"`
	ValidationReady    bool   `json:"validation_ready"`
	StepCount          int    `json:"step_count"`
	SucceededStepCount int    `json:"succeeded_step_count"`
	FailedStepCount    int    `json:"failed_step_count"`
	SkippedStepCount   int    `json:"skipped_step_count"`
	PlannedStepCount   int    `json:"planned_step_count"`
	ResumedStepCount   int    `json:"resumed_step_count"`
	NeedsContextCount  int    `json:"needs_context_count"`
	BlockedStepCount   int    `json:"blocked_step_count"`
}

type MigrationExecutionStepRun struct {
	StepNumber int                            `json:"step_number"`
	Kind       FileKind                       `json:"kind"`
	FileName   string                         `json:"file_name"`
	Status     MigrationExecutionResultStatus `json:"status"`
	Message    string                         `json:"message,omitempty"`
	Error      string                         `json:"error,omitempty"`
	APIPath    string                         `json:"api_path,omitempty"`
	CLICommand string                         `json:"cli_command,omitempty"`
	Response   interface{}                    `json:"response,omitempty"`
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
	run.Summary.StepCount = len(plan.Steps)
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
		if step.Status != MigrationExecutionStepReady {
			status = MigrationExecutionResultSkipped
			message = step.Message
			run.Summary.SkippedStepCount++
		} else if confirmed {
			message = "Ready to import."
		} else {
			run.Summary.PlannedStepCount++
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
		if run.Summary.PlannedStepCount > 0 {
			run.Summary.PlannedStepCount--
		}
		run.Summary.SucceededStepCount++
		run.Summary.ResumedStepCount++
	}

	if run.Summary.ResumedStepCount == 0 {
		return
	}
	run.Summary.Resumed = true
	if run.Summary.PlanReady && run.Summary.SucceededStepCount == run.Summary.StepCount && run.Summary.FailedStepCount == 0 {
		run.Summary.Status = "succeeded"
	}
}

func migrationExecutionRunStepKey(stepNumber int, kind FileKind, fileName string) string {
	return string(kind) + "\x00" + fileName + "\x00" + strconv.Itoa(stepNumber)
}
