package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/go-chi/chi/v5"
)

type reconciliationHandlerService struct {
	evaluation  *smartaccountsreconciliation.Evaluation
	approveArgs []string
}

func (s *reconciliationHandlerService) Evaluate(context.Context, string, string, string) (*smartaccountsreconciliation.Evaluation, bool, error) {
	return s.evaluation, true, nil
}
func (s *reconciliationHandlerService) GetForOwner(context.Context, string, string, string) (*smartaccountsreconciliation.Evaluation, error) {
	return s.evaluation, nil
}
func (s *reconciliationHandlerService) GetForTenant(_ context.Context, tenantID, batchID, sourceID string) (*smartaccountsreconciliation.Evaluation, error) {
	if s.evaluation == nil || s.evaluation.TenantID != tenantID || s.evaluation.BatchID != batchID || s.evaluation.SourceCompanyID != sourceID {
		return nil, smartaccountsreconciliation.ErrNotFound
	}
	return s.evaluation, nil
}
func (s *reconciliationHandlerService) Rollup(context.Context, string, string) (*smartaccountsreconciliation.Rollup, error) {
	return &smartaccountsreconciliation.Rollup{BatchID: "batch", Status: smartaccountsreconciliation.RollupInProgress, SelectedCount: 1}, nil
}
func (s *reconciliationHandlerService) FullClaimStatus(context.Context, string, string) (*smartaccountsreconciliation.FullClaimStatus, error) {
	return &smartaccountsreconciliation.FullClaimStatus{
		Status:                       smartaccountsreconciliation.FullClaimStatusNotEligible,
		FullClaimEligible:            false,
		SelectedCount:                1,
		CurrentPassGapCount:          1,
		MatrixFilterContractGapCount: 1,
		BlockingCodes:                []string{"matrix_filter_contract_gap", "selected_sources_not_current_pass"},
	}, nil
}
func (s *reconciliationHandlerService) Get(context.Context, string, string, string) (*smartaccountsreconciliation.Evaluation, error) {
	return s.evaluation, nil
}
func (s *reconciliationHandlerService) Approve(_ context.Context, actor, role, tenantID, evaluationID string, _ smartaccountsreconciliation.ApprovalRequest) (*smartaccountsreconciliation.Evaluation, bool, error) {
	s.approveArgs = []string{actor, role, tenantID, evaluationID}
	return s.evaluation, false, nil
}

type toleranceHandlerService struct{ calls int }

func (s *toleranceHandlerService) Candidate(context.Context, string, string, string, smartaccountsreconciliation.TolerancePolicyCandidateRequest) (*smartaccountsreconciliation.TolerancePolicyCandidate, error) {
	return &smartaccountsreconciliation.TolerancePolicyCandidate{AlgorithmVersion: smartaccountsreconciliation.ExactMatchTolerancePolicyVersion, Label: smartaccountsreconciliation.ExactMatchTolerancePolicyLabel, CandidateSHA256: strings.Repeat("c", 64)}, nil
}
func (s *toleranceHandlerService) Approve(context.Context, string, string, string, string, smartaccountsreconciliation.TolerancePolicyApprovalRequest) (*smartaccountsreconciliation.TolerancePolicy, bool, error) {
	s.calls++
	return &smartaccountsreconciliation.TolerancePolicy{ID: "11111111-1111-1111-1111-111111111111"}, true, nil
}
func (s *toleranceHandlerService) Resolve(context.Context, string, string, smartaccountsreconciliation.TolerancePolicyCandidateRequest) (*smartaccountsreconciliation.ResolvedTolerancePolicy, error) {
	return &smartaccountsreconciliation.ResolvedTolerancePolicy{PolicyID: "11111111-1111-1111-1111-111111111111", AlgorithmVersion: smartaccountsreconciliation.ExactMatchTolerancePolicyVersion, Label: smartaccountsreconciliation.ExactMatchTolerancePolicyLabel, TolerancePolicySHA256: strings.Repeat("c", 64), ApprovedAt: time.Now().UTC()}, nil
}

func reconciliationRequest(method, body string, claims *auth.Claims, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(context.WithValue(r.Context(), auth.ClaimsContextKey, claims))
	rc := chi.NewRouteContext()
	for key, value := range params {
		rc.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func TestReconciliationOwnerAndAccountantHandlersKeepOnlySafeBindings(t *testing.T) {
	evaluation := &smartaccountsreconciliation.Evaluation{ID: "11111111-1111-1111-1111-111111111111", Status: smartaccountsreconciliation.StatusEvidencePending, CreatedAt: time.Now().UTC()}
	reconciliation := &reconciliationHandlerService{evaluation: evaluation}
	policies := &toleranceHandlerService{}
	h := &Handlers{smartAccountsReconciliationService: reconciliation, smartAccountsTolerancePolicyService: policies}
	owner := &auth.Claims{UserID: "owner", TokenKind: auth.TokenKindAccessToken}
	rr := httptest.NewRecorder()
	h.EvaluateSmartAccountsReconciliation(rr, reconciliationRequest(http.MethodPost, `{}`, owner, map[string]string{"batchID": "batch", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusCreated || strings.Contains(rr.Body.String(), "proof") {
		t.Fatalf("owner evaluation = %d %s", rr.Code, rr.Body.String())
	}
	accountant := &auth.Claims{UserID: "accountant", Role: tenant.RoleAccountant, TokenKind: auth.TokenKindAccessToken}
	rr = httptest.NewRecorder()
	h.ApproveSmartAccountsReconciliation(rr, reconciliationRequest(http.MethodPost, `{"confirmed":true,"evidence_sha256":"`+strings.Repeat("a", 64)+`","tolerance_sha256":"`+strings.Repeat("b", 64)+`"}`, accountant, map[string]string{"tenantID": "tenant-1", "evaluationID": evaluation.ID}))
	if rr.Code != http.StatusOK || len(reconciliation.approveArgs) != 4 || reconciliation.approveArgs[2] != "tenant-1" {
		t.Fatalf("accountant approval = %d %#v", rr.Code, reconciliation.approveArgs)
	}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsTolerancePolicyCandidate(rr, reconciliationRequest(http.MethodPost, `{"package_id":"package","preview_id":"11111111-1111-1111-1111-111111111111"}`, accountant, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "candidate_sha256") {
		t.Fatalf("policy candidate = %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ApproveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{"confirmed":true,"package_id":"package","preview_id":"11111111-1111-1111-1111-111111111111","expected_candidate_sha256":"`+strings.Repeat("c", 64)+`"}`, accountant, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusCreated || policies.calls != 1 {
		t.Fatalf("policy approval = %d / %d", rr.Code, policies.calls)
	}
	owner = &auth.Claims{UserID: "owner", Role: tenant.RoleOwner, TokenKind: auth.TokenKindAccessToken}
	rr = httptest.NewRecorder()
	h.ResolveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{"package_id":"package","preview_id":"11111111-1111-1111-1111-111111111111"}`, owner, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "policy_id") || strings.Contains(rr.Body.String(), "approved_by") {
		t.Fatalf("owner policy resolution = %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ApproveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{"confirmed":true,"package_id":"package","preview_id":"11111111-1111-1111-1111-111111111111","tolerance_policy_sha256":"`+strings.Repeat("c", 64)+`"}`, accountant, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusBadRequest || policies.calls != 1 {
		t.Fatalf("manual tolerance digest must be rejected = %d / %d", rr.Code, policies.calls)
	}
}

func TestReconciliationAccountantActionsRejectAPITokens(t *testing.T) {
	h := &Handlers{smartAccountsReconciliationService: &reconciliationHandlerService{evaluation: &smartaccountsreconciliation.Evaluation{}}, smartAccountsTolerancePolicyService: &toleranceHandlerService{}}
	claims := &auth.Claims{UserID: "accountant", Role: tenant.RoleAccountant, TokenKind: auth.TokenKindAPIToken}
	rr := httptest.NewRecorder()
	h.ApproveSmartAccountsReconciliation(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "evaluationID": "11111111-1111-1111-1111-111111111111"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("API-token accountant approval = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsTolerancePolicyCandidate(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("API-token tolerance candidate = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ResolveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("API-token tolerance resolution = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ApproveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("API-token tolerance approval = %d", rr.Code)
	}
}

func TestReconciliationAccountantActionsRequireAnAccessSession(t *testing.T) {
	h := &Handlers{smartAccountsReconciliationService: &reconciliationHandlerService{evaluation: &smartaccountsreconciliation.Evaluation{}}, smartAccountsTolerancePolicyService: &toleranceHandlerService{}}
	claims := &auth.Claims{UserID: "accountant", Role: tenant.RoleAccountant, TokenKind: ""}
	rr := httptest.NewRecorder()
	h.ApproveSmartAccountsReconciliation(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "evaluationID": "11111111-1111-1111-1111-111111111111"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-access accountant approval = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsTolerancePolicyCandidate(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-access tolerance candidate = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ApproveSmartAccountsTolerancePolicy(rr, reconciliationRequest(http.MethodPost, `{}`, claims, map[string]string{"tenantID": "tenant-1", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-access tolerance approval = %d", rr.Code)
	}
}

func TestGetSmartAccountsTenantReconciliationIsAccountantOnlyAndTenantBound(t *testing.T) {
	evaluation := &smartaccountsreconciliation.Evaluation{
		ID:                  "11111111-1111-1111-1111-111111111111",
		TenantID:            "tenant-1",
		BatchID:             "batch-1",
		SourceCompanyID:     "sa-browser-v1-1234",
		Status:              smartaccountsreconciliation.StatusReadyForAccountant,
		Blockers:            []string{"unresolved_gl_revisions"},
		EvidenceSubmittedBy: "owner-must-not-serialize",
		GLFirstAppliedBy:    "writer-must-not-serialize",
		GLExactReplayBy:     "replay-must-not-serialize",
		CreatedAt:           time.Now().UTC(),
	}
	h := &Handlers{smartAccountsReconciliationService: &reconciliationHandlerService{evaluation: evaluation}}
	accountant := &auth.Claims{UserID: "accountant", Role: tenant.RoleAccountant, TokenKind: auth.TokenKindAccessToken}
	rr := httptest.NewRecorder()
	h.GetSmartAccountsTenantReconciliation(rr, reconciliationRequest(http.MethodGet, "", accountant, map[string]string{"tenantID": "tenant-1", "batchID": "batch-1", "sourceCompanyID": "sa-browser-v1-1234"}))
	if rr.Code != http.StatusOK || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("accountant handoff = %d %#v", rr.Code, rr.Header())
	}
	for _, forbidden := range []string{"owner-must-not-serialize", "writer-must-not-serialize", "replay-must-not-serialize", "debit", "credit"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("unsafe accountant handoff contains %q: %s", forbidden, rr.Body.String())
		}
	}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsTenantReconciliation(rr, reconciliationRequest(http.MethodGet, "", accountant, map[string]string{"tenantID": "other-tenant", "batchID": "batch-1", "sourceCompanyID": "sa-browser-v1-1234"}))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant accountant handoff = %d", rr.Code)
	}
	apiToken := &auth.Claims{UserID: "accountant", Role: tenant.RoleAccountant, TokenKind: auth.TokenKindAPIToken}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsTenantReconciliation(rr, reconciliationRequest(http.MethodGet, "", apiToken, map[string]string{"tenantID": "tenant-1", "batchID": "batch-1", "sourceCompanyID": "sa-browser-v1-1234"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("API-token accountant handoff = %d", rr.Code)
	}
}

func TestReconciliationStrictDecodeRejectsUnknownProofData(t *testing.T) {
	h := &Handlers{smartAccountsReconciliationService: &reconciliationHandlerService{evaluation: &smartaccountsreconciliation.Evaluation{}}}
	rr := httptest.NewRecorder()
	h.EvaluateSmartAccountsReconciliation(rr, reconciliationRequest(http.MethodPost, `{"proof":"raw rows"}`, &auth.Claims{UserID: "owner", TokenKind: auth.TokenKindAccessToken}, map[string]string{"batchID": "batch", "sourceCompanyID": "sa-browser-v1-1"}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("raw proof request = %d", rr.Code)
	}
}

func TestGetSmartAccountsFullClaimEligibilityIsOwnerOnlyAndCountOnly(t *testing.T) {
	h := &Handlers{smartAccountsReconciliationService: &reconciliationHandlerService{}}
	owner := &auth.Claims{UserID: "owner", TokenKind: auth.TokenKindAccessToken}
	rr := httptest.NewRecorder()
	h.GetSmartAccountsFullClaimEligibility(rr, reconciliationRequest(http.MethodGet, "", owner, map[string]string{"batchID": "batch-1"}))
	if rr.Code != http.StatusOK || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("full-claim owner status = %d %#v", rr.Code, rr.Header())
	}
	for _, forbidden := range []string{"sa-browser-v1-", "package", "sha256", "proof", "debit", "credit", "Hold My Beer"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("full-claim status leaks %q: %s", forbidden, rr.Body.String())
		}
	}
	if !strings.Contains(rr.Body.String(), "matrix_filter_contract_gap") || strings.Contains(rr.Body.String(), "full_claim_eligible\":true") {
		t.Fatalf("full-claim fixed status = %s", rr.Body.String())
	}

	apiToken := &auth.Claims{UserID: "owner", TokenKind: auth.TokenKindAPIToken}
	rr = httptest.NewRecorder()
	h.GetSmartAccountsFullClaimEligibility(rr, reconciliationRequest(http.MethodGet, "", apiToken, map[string]string{"batchID": "batch-1"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("full-claim API token = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	request := reconciliationRequest(http.MethodGet, "", owner, map[string]string{"batchID": "batch-1"})
	request.URL.RawQuery = "source_company_id=sa-browser-v1-1"
	h.GetSmartAccountsFullClaimEligibility(rr, request)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("full-claim query = %d", rr.Code)
	}
}
