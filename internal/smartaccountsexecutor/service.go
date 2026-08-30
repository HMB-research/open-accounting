package smartaccountsexecutor

import (
	"context"
	"errors"
	"fmt"
	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"sort"
	"strings"
	"time"
)

var ErrPreviewNotFound = errors.New("SmartAccounts executor preview not found")
var ErrConfirmationRequired = errors.New("explicit matching SmartAccounts preview confirmation is required")
var ErrApplyReceiptRequired = errors.New("SmartAccounts GL apply receipt or approved tolerance policy is required")

type FinancialWriter interface {
	CreateAccount(context.Context, string, string, *accounting.CreateAccountRequest) (*accounting.Account, error)
	GetJournalEntryBySource(context.Context, string, string, string, string) (*accounting.JournalEntry, error)
	CreateJournalEntry(context.Context, string, string, *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error)
	PostJournalEntry(context.Context, string, string, string, string, string) error
}
type Service struct {
	planner  *Planner
	store    Store
	writer   FinancialWriter
	receipts ApplyReceiptRecorder
	policies TolerancePolicyVerifier
}

func NewService(p *Planner, s Store, w FinancialWriter) *Service {
	return &Service{planner: p, store: s, writer: w}
}

func (s *Service) SetApplyReceiptRecorder(recorder ApplyReceiptRecorder) {
	if s != nil {
		s.receipts = recorder
	}
}

func (s *Service) SetTolerancePolicyVerifier(verifier TolerancePolicyVerifier) {
	if s != nil {
		s.policies = verifier
	}
}
func (s *Service) Preview(ctx context.Context, schema, tenant, packageID, user string, req PreviewRequest) (*Preview, error) {
	if s == nil || s.planner == nil || s.store == nil {
		return nil, errors.New("SmartAccounts executor is not configured")
	}
	p, err := s.planner.Preview(ctx, schema, tenant, packageID, req)
	if p != nil {
		if saveErr := s.store.SavePreview(ctx, schema, p, user); saveErr != nil {
			return nil, saveErr
		}
	}
	return p, err
}

// GetPreview is a server-only status seam for reconciliation adapters. It
// returns no archive records and does not create or apply financial entries.
func (s *Service) GetPreview(ctx context.Context, schema, tenant, previewID string) (*Preview, error) {
	if s == nil || s.store == nil {
		return nil, ErrPreviewNotFound
	}
	return s.store.GetPreview(ctx, schema, tenant, previewID)
}
func (s *Service) Apply(ctx context.Context, schema, tenant, user string, req ConfirmRequest) (*Preview, error) {
	if s == nil || s.store == nil || s.writer == nil {
		return nil, errors.New("SmartAccounts executor is not configured")
	}
	if !req.Confirm || strings.TrimSpace(req.PreviewID) == "" {
		return nil, ErrConfirmationRequired
	}
	p, err := s.store.GetPreview(ctx, schema, tenant, req.PreviewID)
	if err != nil {
		return nil, err
	}
	if p.PreviewSHA256 != req.PreviewSHA256 {
		return p, ErrConfirmationRequired
	}
	if s.receipts != nil {
		if !validApplyReceiptDigest(req.TolerancePolicySHA256) || s.policies == nil {
			return p, ErrApplyReceiptRequired
		}
		if err := s.policies.VerifyTolerancePolicy(ctx, TolerancePolicyBinding{TenantID: p.TenantID, SourceCompanyID: p.SourceCompanyID, PackageID: p.PackageID, ScopeSHA256: p.ScopeSHA256, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: req.TolerancePolicySHA256, ActorID: user}); err != nil {
			return p, ErrApplyReceiptRequired
		}
	}
	if p.Status == PlanStatusApplied {
		// A persisted preview status is not itself evidence that the financial
		// side effects still exist. Before treating a confirm as an exact replay,
		// re-read every receipt-bound reservation source and compare it to the
		// immutable preview. A voided, missing, or altered journal is a manual
		// review condition, never a basis for an exact-replay receipt.
		if err := s.verifyAppliedPreview(ctx, schema, tenant, p, user); err != nil {
			return p, err
		}
		if s.receipts != nil {
			if err := s.receipts.RecordExactGLReplay(ctx, p.TenantID, p.SourceCompanyID, p.PackageID, p.PreviewSHA256, user); err != nil {
				return p, err
			}
		}
		return p, nil
	}
	if p.Status != PlanStatusPreviewReady {
		return p, ErrConfirmationRequired
	}
	if s.receipts != nil {
		// If an interrupted apply has already posted only a subset of this
		// preview, only its original financial actor may continue posting. A
		// different actor may safely retry after *all* postings exist solely to
		// backfill a failed receipt; the receipt will retain the original actor.
		existing, listErr := s.store.ListAppliedPostings(ctx, schema, tenant, p.SourceCompanyID, p.PackageID, p.ID)
		if listErr != nil {
			return p, listErr
		}
		if len(existing) > 0 {
			original, actorErr := firstAppliedActor(existing)
			if actorErr != nil {
				return p, actorErr
			}
			if len(existing) < plannedPostingCount(p) && original != strings.TrimSpace(user) {
				return p, ErrApplyReceiptRequired
			}
		}
	}
	for _, imp := range p.AccountImports {
		target := deterministicAccountID(p.TenantID, p.SourceCompanyID, imp.SourceAccountExternalID)
		if err := s.store.SaveMapping(ctx, schema, tenant, p.SourceCompanyID, imp.SourceAccountExternalID, target, imp); err != nil {
			return p, err
		}
		if _, err := s.writer.CreateAccount(ctx, schema, tenant, &accounting.CreateAccountRequest{ID: target, Code: imp.Code, Name: imp.Name, AccountType: accounting.AccountType(imp.AccountType)}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return p, err
		}
	}
	for _, planned := range p.Journals {
		if planned.Action != "CREATE_AND_POST" {
			continue
		}
		identity, created, err := s.store.ReservePosting(ctx, schema, tenant, Provider, p.SourceCompanyID, ResourceGeneralLedger, planned.ExternalID, planned.Revision, p.PackageID, p.ID, user)
		if err != nil {
			return p, err
		}
		if identity == nil || identity.Revision != planned.Revision || identity.PackageID != p.PackageID || identity.PreviewID != p.ID || strings.TrimSpace(identity.ReservationID) == "" {
			return p, fmt.Errorf("journal %s requires review because its posting identity binding differs", planned.ExternalID)
		}
		if !created && identity.Status == PlanStatusApplied {
			if err := s.verifyAppliedJournal(ctx, schema, tenant, p, planned, identity, user); err != nil {
				return p, err
			}
			continue
		}
		if !created && (identity.Status != "RESERVED" || strings.TrimSpace(identity.ReservedBy) != strings.TrimSpace(user)) {
			return p, fmt.Errorf("journal %s requires review because its reserved posting identity cannot be resumed by this actor", planned.ExternalID)
		}
		request, err := journalCreateRequest(p, planned, identity.ReservationID, user)
		if err != nil {
			return p, err
		}
		entry, err := s.findOrCreateReservedJournal(ctx, schema, tenant, request)
		if err != nil {
			return p, err
		}
		if entry.Status == accounting.StatusDraft {
			if err = s.writer.PostJournalEntry(ctx, schema, tenant, entry.ID, user, "confirmed SmartAccounts GL-authoritative import"); err != nil {
				return p, err
			}
		} else if entry.Status != accounting.StatusPosted {
			return p, fmt.Errorf("journal %s requires review because its reserved source entry is not draft or posted", planned.ExternalID)
		}
		if err = s.store.MarkPostingApplied(ctx, schema, tenant, identity.ReservationID, entry.ID, user); err != nil {
			return p, err
		}
	}
	if s.receipts != nil {
		mappings, mapErr := appliedMappings(p)
		if mapErr != nil {
			return p, mapErr
		}
		identities, identityErr := s.store.ListAppliedPostings(ctx, schema, tenant, p.SourceCompanyID, p.PackageID, p.ID)
		if identityErr != nil {
			return p, identityErr
		}
		firstActor, actorErr := firstAppliedActor(identities)
		if actorErr != nil {
			return p, actorErr
		}
		if err := s.receipts.RecordFirstGLApply(ctx, ApplyReceiptInput{TenantID: p.TenantID, SourceCompanyID: p.SourceCompanyID, PackageID: p.PackageID, PreviewID: p.ID, PreviewSHA256: p.PreviewSHA256, TolerancePolicySHA256: req.TolerancePolicySHA256, Mappings: mappings, Identities: identities, ActorID: firstActor}); err != nil {
			return p, err
		}
	}
	// Mark the preview APPLIED only after its append-only receipt has been
	// persisted. If receipt persistence fails, posted identities remain
	// durable but this preview stays retryable; a later exact confirm can
	// safely backfill the first receipt without a second financial write.
	if err = s.store.MarkPreviewApplied(ctx, schema, tenant, p.ID); err != nil {
		return p, err
	}
	p.Status = PlanStatusApplied
	p.FinancialWritesApplied = true
	return p, nil
}

// verifyAppliedPreview proves that an already-applied plan still has exactly
// the posted target effects described by its immutable reservation identities.
// It intentionally does not write or repair anything: failures are review
// conditions so an exact replay cannot attest a changed financial result.
func (s *Service) verifyAppliedPreview(ctx context.Context, schema, tenant string, preview *Preview, user string) error {
	if preview == nil || s.store == nil || s.writer == nil {
		return ErrApplyReceiptRequired
	}
	identities, err := s.store.ListAppliedPostings(ctx, schema, tenant, preview.SourceCompanyID, preview.PackageID, preview.ID)
	if err != nil {
		return err
	}
	if len(identities) != plannedPostingCount(preview) {
		return errors.New("SmartAccounts applied preview has incomplete posting identities and requires review")
	}
	byExternal := make(map[string]AppliedIdentity, len(identities))
	for _, identity := range identities {
		if _, duplicate := byExternal[identity.ExternalID]; duplicate {
			return errors.New("SmartAccounts applied preview has duplicate posting identities and requires review")
		}
		byExternal[identity.ExternalID] = identity
	}
	for _, planned := range preview.Journals {
		if planned.Action != "CREATE_AND_POST" {
			continue
		}
		identity, ok := byExternal[planned.ExternalID]
		if !ok || identity.Revision != planned.Revision {
			// ListAppliedPostings intentionally returns only safe receipt IDs.
			// Its query already binds package/preview; non-empty unexpected values
			// are nevertheless a fail-closed signal for alternate Store impls.
			return errors.New("SmartAccounts applied preview identity binding differs and requires review")
		}
		if err := s.verifyAppliedJournal(ctx, schema, tenant, preview, planned, &PostedIdentity{ExternalID: identity.ExternalID, Revision: identity.Revision, ReservationID: identity.ReservationID, JournalID: identity.JournalID, PackageID: preview.PackageID, PreviewID: preview.ID, Status: PlanStatusApplied}, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) verifyAppliedJournal(ctx context.Context, schema, tenant string, preview *Preview, planned PlannedJournal, identity *PostedIdentity, user string) error {
	if identity == nil || identity.Status != PlanStatusApplied || identity.Revision != planned.Revision || identity.PackageID != preview.PackageID || identity.PreviewID != preview.ID || strings.TrimSpace(identity.ReservationID) == "" || strings.TrimSpace(identity.JournalID) == "" {
		return fmt.Errorf("journal %s requires review because its applied posting identity binding differs", planned.ExternalID)
	}
	request, err := journalCreateRequest(preview, planned, identity.ReservationID, user)
	if err != nil {
		return err
	}
	entry, err := s.writer.GetJournalEntryBySource(ctx, schema, tenant, SmartAccountsGLSourceType, identity.ReservationID)
	if err != nil || entry == nil || entry.ID != identity.JournalID || entry.Status != accounting.StatusPosted || !sameReservedJournal(entry, request) {
		return fmt.Errorf("journal %s requires review because its applied target journal is missing, voided, or differs", planned.ExternalID)
	}
	return nil
}

func firstAppliedActor(identities []AppliedIdentity) (string, error) {
	if len(identities) == 0 {
		return "", ErrApplyReceiptRequired
	}
	actor := strings.TrimSpace(identities[0].AppliedBy)
	if actor == "" {
		return "", ErrApplyReceiptRequired
	}
	for _, identity := range identities[1:] {
		if strings.TrimSpace(identity.AppliedBy) != actor {
			return "", ErrApplyReceiptRequired
		}
	}
	return actor, nil
}

func plannedPostingCount(preview *Preview) int {
	count := 0
	if preview == nil {
		return count
	}
	for _, journal := range preview.Journals {
		if journal.Action == "CREATE_AND_POST" {
			count++
		}
	}
	return count
}

func journalCreateRequest(preview *Preview, planned PlannedJournal, sourceID, user string) (*accounting.CreateJournalEntryRequest, error) {
	if preview == nil || strings.TrimSpace(sourceID) == "" {
		return nil, ErrApplyReceiptRequired
	}
	lines := make([]accounting.CreateJournalEntryLineReq, 0, len(planned.Lines))
	for i, line := range planned.Lines {
		target := ""
		for _, mapped := range planned.MappedLines {
			if mapped.SourceAccountExternalID == line.SourceAccountExternalID {
				target = mapped.TargetAccountID
				break
			}
		}
		if target == "" {
			return nil, fmt.Errorf("journal %s line %d is unmapped", planned.ExternalID, i+1)
		}
		debit, credit := line.Debit, line.Credit
		if planned.Currency != "EUR" {
			debit, credit = line.DebitOriginalCurrency, line.CreditOriginalCurrency
		}
		lines = append(lines, accounting.CreateJournalEntryLineReq{AccountID: target, Description: line.Description, DebitAmount: debit, CreditAmount: credit, Currency: planned.Currency, ExchangeRate: exchangeRate(planned.Currency, planned.ExchangeRate)})
	}
	date, err := time.Parse("2006-01-02", planned.PostingDate)
	if err != nil {
		return nil, err
	}
	return &accounting.CreateJournalEntryRequest{EntryDate: date, Description: "SmartAccounts GL journal " + planned.ExternalID, Reference: planned.DocumentReference, SourceType: SmartAccountsGLSourceType, SourceID: &sourceID, Lines: lines, UserID: user}, nil
}

// findOrCreateReservedJournal makes a reservation retry safe. The reservation
// UUID is the immutable OA journal source_id. A pre-existing row is accepted
// only after every generated journal field and line matches the exact preview.
// This deliberately fails closed on ambiguity instead of guessing whether a
// previous CreateJournalEntry returned after a database commit.
func (s *Service) findOrCreateReservedJournal(ctx context.Context, schema, tenant string, expected *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	if expected == nil || expected.SourceID == nil {
		return nil, ErrApplyReceiptRequired
	}
	existing, err := s.writer.GetJournalEntryBySource(ctx, schema, tenant, SmartAccountsGLSourceType, *expected.SourceID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !sameReservedJournal(existing, expected) {
			return nil, errors.New("reserved SmartAccounts source journal differs from the immutable preview and requires review")
		}
		return existing, nil
	}
	entry, err := s.writer.CreateJournalEntry(ctx, schema, tenant, expected)
	if err == nil {
		return entry, nil
	}
	if !isJournalSourceUniqueError(err) {
		return nil, err
	}
	// A concurrent create can only be recovered by reading the exact source
	// identity and verifying it. Never treat a generic unique error as success.
	entry, readErr := s.writer.GetJournalEntryBySource(ctx, schema, tenant, SmartAccountsGLSourceType, *expected.SourceID)
	if readErr != nil {
		return nil, readErr
	}
	if entry == nil || !sameReservedJournal(entry, expected) {
		return nil, errors.New("reserved SmartAccounts source journal unique conflict requires review")
	}
	return entry, nil
}

func isJournalSourceUniqueError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}

func sameReservedJournal(entry *accounting.JournalEntry, expected *accounting.CreateJournalEntryRequest) bool {
	if entry == nil || expected == nil || expected.SourceID == nil || entry.SourceID == nil || entry.SourceType != expected.SourceType || *entry.SourceID != *expected.SourceID || entry.EntryDate.Format("2006-01-02") != expected.EntryDate.Format("2006-01-02") || entry.Description != expected.Description || entry.Reference != expected.Reference || len(entry.Lines) != len(expected.Lines) {
		return false
	}
	actualLines := make([]string, 0, len(entry.Lines))
	for _, line := range entry.Lines {
		actualLines = append(actualLines, journalLineFingerprint(line.AccountID, line.Description, line.DebitAmount, line.CreditAmount, line.Currency, line.ExchangeRate, line.BaseDebit, line.BaseCredit, line.VATRate, line.IsVATInclusive))
	}
	expectedLines := make([]string, 0, len(expected.Lines))
	for _, line := range expected.Lines {
		baseDebit, baseCredit := line.DebitAmount.Mul(line.ExchangeRate), line.CreditAmount.Mul(line.ExchangeRate)
		expectedLines = append(expectedLines, journalLineFingerprint(line.AccountID, line.Description, line.DebitAmount, line.CreditAmount, line.Currency, line.ExchangeRate, baseDebit, baseCredit, line.VATRate, line.IsVATInclusive))
	}
	sort.Strings(actualLines)
	sort.Strings(expectedLines)
	return strings.Join(actualLines, "\x00") == strings.Join(expectedLines, "\x00")
}

func journalLineFingerprint(accountID, description string, debit, credit decimal.Decimal, currency string, exchangeRate, baseDebit, baseCredit, vatRate decimal.Decimal, vatInclusive bool) string {
	return strings.Join([]string{accountID, description, debit.String(), credit.String(), currency, exchangeRate.String(), baseDebit.String(), baseCredit.String(), vatRate.String(), fmt.Sprintf("%t", vatInclusive)}, "\x1f")
}
func deterministicAccountID(tenant, source, account string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenant+"\x00"+source+"\x00"+account)).String()
}
func exchangeRate(currency string, rate decimal.Decimal) decimal.Decimal {
	if currency == "EUR" {
		return decimal.NewFromInt(1)
	}
	return rate
}

func validApplyReceiptDigest(v string) bool {
	return len(v) == 64 && strings.Trim(v, "0123456789abcdef") == ""
}

func appliedMappings(p *Preview) ([]AppliedMapping, error) {
	if p == nil {
		return nil, ErrApplyReceiptRequired
	}
	bySource := make(map[string]string)
	for _, journal := range p.Journals {
		if journal.Action != "CREATE_AND_POST" {
			continue
		}
		for _, line := range journal.MappedLines {
			source, target := strings.TrimSpace(line.SourceAccountExternalID), strings.TrimSpace(line.TargetAccountID)
			if source == "" || target == "" {
				return nil, ErrApplyReceiptRequired
			}
			if existing, found := bySource[source]; found && existing != target {
				return nil, ErrApplyReceiptRequired
			}
			bySource[source] = target
		}
	}
	result := make([]AppliedMapping, 0, len(bySource))
	for source, target := range bySource {
		result = append(result, AppliedMapping{SourceAccountExternalID: source, TargetAccountID: target})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceAccountExternalID < result[j].SourceAccountExternalID })
	if len(result) == 0 {
		return nil, ErrApplyReceiptRequired
	}
	return result, nil
}
