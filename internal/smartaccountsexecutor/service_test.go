package smartaccountsexecutor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
)

type memoryStore struct {
	previews map[string]*Preview
	postings map[string]*PostedIdentity
	reserves int
	mappings int
}

func (m *memoryStore) SavePreview(_ context.Context, _ string, p *Preview, _ string) error {
	if m.previews == nil {
		m.previews = map[string]*Preview{}
	}
	m.previews[p.ID] = p
	return nil
}
func (m *memoryStore) GetPreview(_ context.Context, _, _, id string) (*Preview, error) {
	if p := m.previews[id]; p != nil {
		return p, nil
	}
	return nil, ErrPreviewNotFound
}
func (m *memoryStore) GetPostedIdentity(_ context.Context, _, _, _, _, _, external string) (*PostedIdentity, error) {
	return m.postings[external], nil
}
func (m *memoryStore) ReservePosting(_ context.Context, _, _, _, _, _, external, revision, packageID, previewID, reservedBy string) (*PostedIdentity, bool, error) {
	if m.postings == nil {
		m.postings = map[string]*PostedIdentity{}
	}
	if p := m.postings[external]; p != nil {
		return p, false, nil
	}
	p := &PostedIdentity{ExternalID: external, Revision: revision, ReservationID: "11111111-1111-1111-1111-111111111111", Status: "RESERVED", PackageID: packageID, PreviewID: previewID, ReservedBy: reservedBy}
	m.postings[external] = p
	m.reserves++
	return p, true, nil
}
func (m *memoryStore) MarkPostingApplied(_ context.Context, _, _, id, journal, actor string) error {
	for _, p := range m.postings {
		if p.ReservationID == id {
			p.Status = PlanStatusApplied
			p.JournalID = journal
			p.AppliedBy = actor
			return nil
		}
	}
	return errors.New("missing")
}
func (m *memoryStore) MarkPreviewApplied(_ context.Context, _, _, id string) error {
	m.previews[id].Status = PlanStatusApplied
	return nil
}
func (m *memoryStore) SaveMapping(context.Context, string, string, string, string, string, AccountImport) error {
	m.mappings++
	return nil
}
func (m *memoryStore) ListAppliedPostings(_ context.Context, _, _, _, _, _ string) ([]AppliedIdentity, error) {
	result := make([]AppliedIdentity, 0, len(m.postings))
	for _, posting := range m.postings {
		if posting.Status == PlanStatusApplied {
			result = append(result, AppliedIdentity{ExternalID: posting.ExternalID, Revision: posting.Revision, ReservationID: posting.ReservationID, JournalID: posting.JournalID, AppliedBy: posting.AppliedBy})
		}
	}
	return result, nil
}

type fakeWriter struct {
	accounts, creates, posts, failCreateAt, failPostAt, failAfterPostAt int
	raceCreate                                                          bool
	entries                                                             map[string]*accounting.JournalEntry
}

type fakeApplyReceiptRecorder struct {
	first     []ApplyReceiptInput
	replay    [][5]string
	failFirst bool
}

func (r *fakeApplyReceiptRecorder) RecordFirstGLApply(_ context.Context, input ApplyReceiptInput) error {
	if r.failFirst {
		r.failFirst = false
		return errors.New("receipt unavailable")
	}
	r.first = append(r.first, input)
	return nil
}

type fakeTolerancePolicyVerifier struct {
	bindings           []TolerancePolicyBinding
	err                error
	expectedPreviewSHA string
}

func (v *fakeTolerancePolicyVerifier) VerifyTolerancePolicy(_ context.Context, binding TolerancePolicyBinding) error {
	v.bindings = append(v.bindings, binding)
	if v.expectedPreviewSHA != "" && binding.PreviewSHA256 != v.expectedPreviewSHA {
		return errors.New("policy preview binding mismatch")
	}
	return v.err
}
func (r *fakeApplyReceiptRecorder) RecordExactGLReplay(_ context.Context, tenant, source, packageID, previewSHA, actor string) error {
	r.replay = append(r.replay, [5]string{tenant, source, packageID, previewSHA, actor})
	return nil
}

func (f *fakeWriter) CreateAccount(_ context.Context, _, _ string, _ *accounting.CreateAccountRequest) (*accounting.Account, error) {
	f.accounts++
	return &accounting.Account{}, nil
}
func (f *fakeWriter) GetJournalEntryBySource(_ context.Context, _, _ string, sourceType, sourceID string) (*accounting.JournalEntry, error) {
	if f.entries == nil {
		return nil, nil
	}
	entry := f.entries[sourceType+"\x00"+sourceID]
	return entry, nil
}
func (f *fakeWriter) CreateJournalEntry(_ context.Context, _, _ string, request *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	f.creates++
	if f.failCreateAt > 0 && f.creates == f.failCreateAt {
		return nil, errors.New("journal create unavailable")
	}
	if f.entries == nil {
		f.entries = map[string]*accounting.JournalEntry{}
	}
	if request.SourceID == nil {
		return nil, errors.New("source id required")
	}
	key := request.SourceType + "\x00" + *request.SourceID
	if _, exists := f.entries[key]; exists {
		return nil, errors.New("duplicate key source identity")
	}
	entry := f.newEntry(request)
	if f.raceCreate {
		f.entries[key] = entry
		return nil, errors.New("duplicate key source identity")
	}
	f.entries[key] = entry
	return entry, nil
}
func (f *fakeWriter) newEntry(request *accounting.CreateJournalEntryRequest) *accounting.JournalEntry {
	lines := make([]accounting.JournalEntryLine, 0, len(request.Lines))
	for _, line := range request.Lines {
		lines = append(lines, accounting.JournalEntryLine{AccountID: line.AccountID, Description: line.Description, DebitAmount: line.DebitAmount, CreditAmount: line.CreditAmount, Currency: line.Currency, ExchangeRate: line.ExchangeRate, BaseDebit: line.DebitAmount.Mul(line.ExchangeRate), BaseCredit: line.CreditAmount.Mul(line.ExchangeRate), VATRate: line.VATRate, IsVATInclusive: line.IsVATInclusive})
	}
	return &accounting.JournalEntry{ID: fmt.Sprintf("00000000-0000-0000-0000-%012d", f.creates), EntryDate: request.EntryDate, Description: request.Description, Reference: request.Reference, SourceType: request.SourceType, SourceID: request.SourceID, Status: accounting.StatusDraft, Lines: lines}
}
func (f *fakeWriter) PostJournalEntry(_ context.Context, _, _ string, journalID, _, _ string) error {
	f.posts++
	if f.failPostAt > 0 && f.posts == f.failPostAt {
		return errors.New("journal post unavailable")
	}
	for _, entry := range f.entries {
		if entry.ID == journalID {
			entry.Status = accounting.StatusPosted
			if f.failAfterPostAt > 0 && f.posts == f.failAfterPostAt {
				return errors.New("journal post response lost")
			}
			return nil
		}
	}
	return errors.New("journal missing")
}

func readyPreview() *Preview {
	return &Preview{ID: "preview", TenantID: "tenant", PackageID: "package", SourceCompanyID: "source", ScopeSHA256: strings.Repeat("c", 64), Status: PlanStatusPreviewReady, PreviewSHA256: "digest", Journals: []PlannedJournal{{Journal: Journal{ExternalID: "journal", Revision: "revision", PostingDate: time.Now().UTC().Format("2006-01-02"), Currency: "EUR", Lines: []JournalLine{{SourceAccountExternalID: "source-account", Debit: decimal.NewFromInt(5)}, {SourceAccountExternalID: "credit", Credit: decimal.NewFromInt(5)}}}, MappedLines: []MappedLine{{SourceAccountExternalID: "source-account", TargetAccountID: "account-1"}, {SourceAccountExternalID: "credit", TargetAccountID: "account-2"}}, Action: "CREATE_AND_POST"}}}
}
func TestApplyRequiresExactConfirmationAndReplaysSafely(t *testing.T) {
	store := &memoryStore{previews: map[string]*Preview{"preview": readyPreview()}}
	writer := &fakeWriter{}
	svc := NewService(nil, store, writer)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{PreviewID: "preview", PreviewSHA256: "digest"}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("missing confirmation: %v", err)
	}
	p, err := svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: "digest"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != PlanStatusApplied || writer.creates != 1 || writer.posts != 1 || store.reserves != 1 {
		t.Fatalf("apply = %#v / writer=%#v", p, writer)
	}
	if _, err = svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: "digest"}); err != nil {
		t.Fatal(err)
	}
	if writer.creates != 1 || writer.posts != 1 {
		t.Fatalf("replay wrote again: %#v", writer)
	}
}
func TestApplyDoesNotPostExistingDifferentRevision(t *testing.T) {
	p := readyPreview()
	store := &memoryStore{previews: map[string]*Preview{"preview": p}, postings: map[string]*PostedIdentity{"journal": {ExternalID: "journal", Revision: "other", Status: PlanStatusApplied}}}
	writer := &fakeWriter{}
	_, err := NewService(nil, store, writer).Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: "digest"})
	if err == nil || writer.creates != 0 {
		t.Fatalf("revision conflict must not post: %v / %#v", err, writer)
	}
}

func TestApplyRecordsIDOnlyMappingIdentitySnapshotAndExactReplay(t *testing.T) {
	p := readyPreview()
	p.PreviewSHA256 = strings.Repeat("a", 64)
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	receipts := &fakeApplyReceiptRecorder{}
	policies := &fakeTolerancePolicyVerifier{}
	svc := NewService(nil, store, &fakeWriter{})
	svc.SetApplyReceiptRecorder(receipts)
	svc.SetTolerancePolicyVerifier(policies)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: p.PreviewSHA256}); !errors.Is(err, ErrApplyReceiptRequired) {
		t.Fatalf("missing tolerance policy = %v", err)
	}
	policy := strings.Repeat("b", 64)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err != nil {
		t.Fatal(err)
	}
	if len(receipts.first) != 1 || len(receipts.first[0].Mappings) != 2 || len(receipts.first[0].Identities) != 1 || receipts.first[0].TolerancePolicySHA256 != policy {
		t.Fatalf("first receipt = %#v", receipts.first)
	}
	if len(policies.bindings) == 0 || policies.bindings[0].ScopeSHA256 != p.ScopeSHA256 {
		t.Fatalf("policy verification did not receive exact scope binding: %#v", policies.bindings)
	}
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: "preview", PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err != nil {
		t.Fatal(err)
	}
	if len(receipts.replay) != 1 || receipts.replay[0][3] != p.PreviewSHA256 {
		t.Fatalf("replay receipt = %#v", receipts.replay)
	}
}

func TestApplyReplayRechecksEveryPostedJournalBeforeReceipt(t *testing.T) {
	for name, mutate := range map[string]func(*accounting.JournalEntry){
		"voided":       func(entry *accounting.JournalEntry) { entry.Status = accounting.StatusVoided },
		"changed_line": func(entry *accounting.JournalEntry) { entry.Lines[0].DebitAmount = decimal.NewFromInt(99) },
	} {
		t.Run(name, func(t *testing.T) {
			p := readyPreview()
			p.PreviewSHA256 = strings.Repeat("a", 64)
			store := &memoryStore{previews: map[string]*Preview{"preview": p}}
			writer := &fakeWriter{}
			receipts := &fakeApplyReceiptRecorder{}
			svc := NewService(nil, store, writer)
			svc.SetApplyReceiptRecorder(receipts)
			svc.SetTolerancePolicyVerifier(&fakeTolerancePolicyVerifier{})
			policy := strings.Repeat("b", 64)
			if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err != nil {
				t.Fatal(err)
			}
			for _, entry := range writer.entries {
				mutate(entry)
			}
			if _, err := svc.Apply(context.Background(), "schema", "tenant", "other-qualified-operator", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err == nil {
				t.Fatal("changed posted target must require review before exact replay")
			}
			if len(receipts.replay) != 0 || writer.creates != 1 || writer.posts != 1 {
				t.Fatalf("failed replay must not attest or write: receipts=%#v writer=%#v", receipts.replay, writer)
			}
		})
	}
}

func TestApplyRetriesReceiptBeforePreviewIsMarkedApplied(t *testing.T) {
	p := readyPreview()
	p.PreviewSHA256 = strings.Repeat("a", 64)
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	receipts := &fakeApplyReceiptRecorder{failFirst: true}
	svc := NewService(nil, store, &fakeWriter{})
	svc.SetApplyReceiptRecorder(receipts)
	svc.SetTolerancePolicyVerifier(&fakeTolerancePolicyVerifier{})
	policy := strings.Repeat("b", 64)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err == nil {
		t.Fatal("first receipt failure must be returned")
	}
	if p.Status != PlanStatusPreviewReady {
		t.Fatalf("receipt failure stranded preview as %s", p.Status)
	}
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator-2", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err != nil {
		t.Fatalf("retry must backfill first receipt without new write: %v", err)
	}
	if p.Status != PlanStatusApplied || len(receipts.first) != 1 || receipts.first[0].ActorID != "operator" {
		t.Fatalf("retry = status %s receipts %#v", p.Status, receipts.first)
	}
}

func TestInterruptedPostingCannotSwitchFinancialActor(t *testing.T) {
	p := readyPreview()
	p.PreviewSHA256 = strings.Repeat("a", 64)
	p.Journals = append(p.Journals, PlannedJournal{Journal: Journal{ExternalID: "journal-2", Revision: "revision-2", PostingDate: p.Journals[0].PostingDate, Currency: "EUR", Lines: p.Journals[0].Lines}, MappedLines: p.Journals[0].MappedLines, Action: "CREATE_AND_POST"})
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	writer := &fakeWriter{failCreateAt: 2}
	svc := NewService(nil, store, writer)
	svc.SetApplyReceiptRecorder(&fakeApplyReceiptRecorder{})
	svc.SetTolerancePolicyVerifier(&fakeTolerancePolicyVerifier{})
	policy := strings.Repeat("b", 64)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); err == nil {
		t.Fatal("partial posting error expected")
	}
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-b", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: policy}); !errors.Is(err, ErrApplyReceiptRequired) {
		t.Fatalf("cross-actor partial continuation = %v", err)
	}
	if writer.posts != 1 || p.Status != PlanStatusPreviewReady {
		t.Fatalf("cross-actor attempt must not alter the partial posting state: posts=%d status=%s", writer.posts, p.Status)
	}
}

func TestApplyResumesReservedJournalAfterCreateBeforePost(t *testing.T) {
	p := readyPreview()
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	writer := &fakeWriter{failPostAt: 1}
	svc := NewService(nil, store, writer)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err == nil {
		t.Fatal("interrupted post expected")
	}
	if writer.creates != 1 || writer.posts != 1 || store.postings["journal"].Status != "RESERVED" {
		t.Fatalf("first attempt did not leave one durable reservation: writer=%#v posting=%#v", writer, store.postings["journal"])
	}
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err != nil {
		t.Fatalf("same actor resume: %v", err)
	}
	if writer.creates != 1 || writer.posts != 2 || store.postings["journal"].Status != PlanStatusApplied {
		t.Fatalf("resume duplicated a journal or failed to post: writer=%#v posting=%#v", writer, store.postings["journal"])
	}
}

func TestApplyResumesReservedJournalAfterPostResponseLoss(t *testing.T) {
	p := readyPreview()
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	writer := &fakeWriter{failAfterPostAt: 1}
	svc := NewService(nil, store, writer)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err == nil {
		t.Fatal("lost post response expected")
	}
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err != nil {
		t.Fatalf("posted journal resume: %v", err)
	}
	if writer.creates != 1 || writer.posts != 1 || store.postings["journal"].Status != PlanStatusApplied {
		t.Fatalf("post-loss resume wrote again: writer=%#v posting=%#v", writer, store.postings["journal"])
	}
}

func TestApplyReservedJournalMismatchRequiresReviewWithoutWriting(t *testing.T) {
	p := readyPreview()
	reservationID := "11111111-1111-1111-1111-111111111111"
	store := &memoryStore{previews: map[string]*Preview{"preview": p}, postings: map[string]*PostedIdentity{"journal": {ExternalID: "journal", Revision: "revision", ReservationID: reservationID, Status: "RESERVED", PackageID: p.PackageID, PreviewID: p.ID, ReservedBy: "actor-a"}}}
	request, err := journalCreateRequest(p, p.Journals[0], reservationID, "actor-a")
	if err != nil {
		t.Fatal(err)
	}
	writer := &fakeWriter{entries: map[string]*accounting.JournalEntry{}}
	entry := writer.newEntry(request)
	entry.Reference = "different-source-reference"
	writer.entries[SmartAccountsGLSourceType+"\x00"+reservationID] = entry
	if _, err := NewService(nil, store, writer).Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err == nil {
		t.Fatal("mismatched reserved target must require review")
	}
	if writer.creates != 0 || writer.posts != 0 || store.postings["journal"].Status != "RESERVED" {
		t.Fatalf("mismatch must not write or mark applied: writer=%#v posting=%#v", writer, store.postings["journal"])
	}
}

func TestApplyRecoversUniqueSourceCreateRaceByExactRead(t *testing.T) {
	p := readyPreview()
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	writer := &fakeWriter{raceCreate: true}
	if _, err := NewService(nil, store, writer).Apply(context.Background(), "schema", "tenant", "actor-a", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256}); err != nil {
		t.Fatalf("exact source race must be reread and resumed: %v", err)
	}
	if writer.creates != 1 || writer.posts != 1 || store.postings["journal"].Status != PlanStatusApplied {
		t.Fatalf("source race result = writer=%#v posting=%#v", writer, store.postings["journal"])
	}
}

func TestApplyRejectsPolicyApprovedForDifferentPreviewMapping(t *testing.T) {
	p := readyPreview()
	p.PreviewSHA256 = strings.Repeat("a", 64)
	store := &memoryStore{previews: map[string]*Preview{"preview": p}}
	policies := &fakeTolerancePolicyVerifier{expectedPreviewSHA: strings.Repeat("b", 64)}
	svc := NewService(nil, store, &fakeWriter{})
	svc.SetApplyReceiptRecorder(&fakeApplyReceiptRecorder{})
	svc.SetTolerancePolicyVerifier(policies)
	if _, err := svc.Apply(context.Background(), "schema", "tenant", "operator", ConfirmRequest{Confirm: true, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: strings.Repeat("c", 64)}); !errors.Is(err, ErrApplyReceiptRequired) {
		t.Fatalf("different preview policy must be rejected: %v", err)
	}
	if len(policies.bindings) != 1 || policies.bindings[0].PreviewSHA256 != p.PreviewSHA256 {
		t.Fatalf("policy preview binding = %#v", policies.bindings)
	}
}
