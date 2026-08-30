package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

type executorHandlerStore struct {
	preview *smartaccountsexecutor.Preview
	posting *smartaccountsexecutor.PostedIdentity
}

func (s *executorHandlerStore) SavePreview(context.Context, string, *smartaccountsexecutor.Preview, string) error {
	return nil
}
func (s *executorHandlerStore) GetPreview(context.Context, string, string, string) (*smartaccountsexecutor.Preview, error) {
	if s.preview == nil {
		return nil, smartaccountsexecutor.ErrPreviewNotFound
	}
	return s.preview, nil
}
func (s *executorHandlerStore) GetPostedIdentity(context.Context, string, string, string, string, string, string) (*smartaccountsexecutor.PostedIdentity, error) {
	return s.posting, nil
}
func (s *executorHandlerStore) ReservePosting(_ context.Context, _ string, _ string, _ string, _ string, _ string, external string, revision string, packageID, previewID, reservedBy string) (*smartaccountsexecutor.PostedIdentity, bool, error) {
	if s.posting != nil {
		return s.posting, false, nil
	}
	s.posting = &smartaccountsexecutor.PostedIdentity{ExternalID: external, Revision: revision, ReservationID: "11111111-1111-1111-1111-111111111111", Status: "RESERVED", PackageID: packageID, PreviewID: previewID, ReservedBy: reservedBy}
	return s.posting, true, nil
}
func (s *executorHandlerStore) MarkPostingApplied(_ context.Context, _, _, _, journal, actor string) error {
	s.posting.Status = smartaccountsexecutor.PlanStatusApplied
	s.posting.JournalID = journal
	s.posting.AppliedBy = actor
	return nil
}
func (s *executorHandlerStore) MarkPreviewApplied(context.Context, string, string, string) error {
	s.preview.Status = smartaccountsexecutor.PlanStatusApplied
	return nil
}
func (s *executorHandlerStore) SaveMapping(context.Context, string, string, string, string, string, smartaccountsexecutor.AccountImport) error {
	return nil
}
func (s *executorHandlerStore) ListAppliedPostings(_ context.Context, _, _, _, _, _ string) ([]smartaccountsexecutor.AppliedIdentity, error) {
	if s.posting == nil || s.posting.Status != smartaccountsexecutor.PlanStatusApplied {
		return nil, nil
	}
	return []smartaccountsexecutor.AppliedIdentity{{ExternalID: s.posting.ExternalID, Revision: s.posting.Revision, ReservationID: s.posting.ReservationID, JournalID: s.posting.JournalID, AppliedBy: s.posting.AppliedBy}}, nil
}

type executorHandlerWriter struct{ creates, posts int }

type executorTolerancePolicyService struct{}

func (executorTolerancePolicyService) Candidate(context.Context, string, string, string, smartaccountsreconciliation.TolerancePolicyCandidateRequest) (*smartaccountsreconciliation.TolerancePolicyCandidate, error) {
	return nil, smartaccountsreconciliation.ErrNotFound
}
func (executorTolerancePolicyService) Approve(context.Context, string, string, string, string, smartaccountsreconciliation.TolerancePolicyApprovalRequest) (*smartaccountsreconciliation.TolerancePolicy, bool, error) {
	return nil, false, smartaccountsreconciliation.ErrNotFound
}
func (executorTolerancePolicyService) Resolve(context.Context, string, string, smartaccountsreconciliation.TolerancePolicyCandidateRequest) (*smartaccountsreconciliation.ResolvedTolerancePolicy, error) {
	return &smartaccountsreconciliation.ResolvedTolerancePolicy{PolicyID: "11111111-1111-1111-1111-111111111111", AlgorithmVersion: smartaccountsreconciliation.ExactMatchTolerancePolicyVersion, Label: smartaccountsreconciliation.ExactMatchTolerancePolicyLabel, TolerancePolicySHA256: strings.Repeat("a", 64), ApprovedAt: time.Now().UTC()}, nil
}

type executorHandlerReceiptRecorder struct{ first, replay int }

func (r *executorHandlerReceiptRecorder) RecordFirstGLApply(context.Context, smartaccountsexecutor.ApplyReceiptInput) error {
	r.first++
	return nil
}
func (r *executorHandlerReceiptRecorder) RecordExactGLReplay(context.Context, string, string, string, string, string) error {
	r.replay++
	return nil
}

type executorHandlerPolicyVerifier struct {
	approver string
	calls    int
}

func (v *executorHandlerPolicyVerifier) VerifyTolerancePolicy(_ context.Context, binding smartaccountsexecutor.TolerancePolicyBinding) error {
	v.calls++
	if binding.ActorID == v.approver || binding.TolerancePolicySHA256 != strings.Repeat("a", 64) {
		return context.Canceled
	}
	return nil
}

func (w *executorHandlerWriter) CreateAccount(context.Context, string, string, *accounting.CreateAccountRequest) (*accounting.Account, error) {
	return &accounting.Account{}, nil
}
func (w *executorHandlerWriter) GetJournalEntryBySource(context.Context, string, string, string, string) (*accounting.JournalEntry, error) {
	return nil, nil
}
func (w *executorHandlerWriter) CreateJournalEntry(_ context.Context, _ string, _ string, request *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	w.creates++
	return &accounting.JournalEntry{ID: "22222222-2222-2222-2222-222222222222", Status: accounting.StatusDraft, SourceType: request.SourceType, SourceID: request.SourceID}, nil
}
func (w *executorHandlerWriter) PostJournalEntry(context.Context, string, string, string, string, string) error {
	w.posts++
	return nil
}
func executorPreview() *smartaccountsexecutor.Preview {
	return &smartaccountsexecutor.Preview{ID: "preview", TenantID: "tenant", SourceCompanyID: "source", PackageID: "package", Status: smartaccountsexecutor.PlanStatusPreviewReady, PreviewSHA256: "digest", Journals: []smartaccountsexecutor.PlannedJournal{{Journal: smartaccountsexecutor.Journal{ExternalID: "j", Revision: "r", PostingDate: time.Now().UTC().Format("2006-01-02"), Currency: "EUR", Lines: []smartaccountsexecutor.JournalLine{{SourceAccountExternalID: "a", Debit: decimal.NewFromInt(1)}, {SourceAccountExternalID: "b", Credit: decimal.NewFromInt(1)}}}, MappedLines: []smartaccountsexecutor.MappedLine{{SourceAccountExternalID: "a", TargetAccountID: "a1"}, {SourceAccountExternalID: "b", TargetAccountID: "b1"}}, Action: "CREATE_AND_POST"}}}
}
func TestApplySmartAccountsPackageRequiresConfirmationAndDoesNotExposeRawSource(t *testing.T) {
	store := &executorHandlerStore{preview: executorPreview()}
	writer := &executorHandlerWriter{}
	receipts, policies := &executorHandlerReceiptRecorder{}, &executorHandlerPolicyVerifier{approver: "accountant"}
	executor := smartaccountsexecutor.NewService(nil, store, writer)
	executor.SetApplyReceiptRecorder(receipts)
	executor.SetTolerancePolicyVerifier(policies)
	h := &Handlers{smartAccountsExecutor: executor, smartAccountsTolerancePolicyService: executorTolerancePolicyService{}}
	call := func(actor, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Body = io.NopCloser(strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, &auth.Claims{UserID: actor})
		rc := chi.NewRouteContext()
		rc.URLParams.Add("tenantID", "tenant")
		r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rc))
		rr := httptest.NewRecorder()
		h.ApplySmartAccountsPackage(rr, r)
		return rr
	}
	if rr := call("owner", `{"preview_id":"preview","preview_sha256":"digest"}`); rr.Code != http.StatusConflict || writer.creates != 0 {
		t.Fatalf("unconfirmed=%d %s", rr.Code, rr.Body.String())
	}
	rr := call("owner", `{"confirm":true,"preview_id":"preview","preview_sha256":"digest","tolerance_policy_id":"11111111-1111-1111-1111-111111111111"}`)
	if rr.Code != http.StatusOK || writer.creates != 1 || writer.posts != 1 {
		t.Fatalf("confirmed=%d %s", rr.Code, rr.Body.String())
	}
	if receipts.first != 1 || policies.calls != 1 {
		t.Fatalf("separate owner must resolve and use the approved policy once: receipts=%#v policies=%#v", receipts, policies)
	}
	if strings.Contains(rr.Body.String(), "payload") {
		t.Fatal("raw payload leaked")
	}
	if rr = call("owner", `{"confirm":true,"preview_id":"preview","preview_sha256":"digest","tolerance_policy_sha256":"`+strings.Repeat("a", 64)+`"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("manual tolerance digest must be rejected = %d %s", rr.Code, rr.Body.String())
	}
	if rr = call("accountant", `{"confirm":true,"preview_id":"preview","preview_sha256":"digest","tolerance_policy_id":"11111111-1111-1111-1111-111111111111"}`); rr.Code != http.StatusConflict || receipts.replay != 0 {
		t.Fatalf("policy approver must not apply or attest replay = %d %s", rr.Code, rr.Body.String())
	}
}
