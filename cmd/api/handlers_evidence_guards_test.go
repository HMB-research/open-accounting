package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func installApprovedEvidenceDocuments(t *testing.T, h *Handlers, docs ...documents.Document) *mockDocumentRepository {
	t.Helper()

	repo := newMockDocumentRepository()
	h.documentsService = documents.NewService(repo, nil)
	now := time.Now().UTC()
	for i := range docs {
		doc := docs[i]
		if doc.ID == "" {
			doc.ID = fmt.Sprintf("doc-%d", i+1)
		}
		if doc.TenantID == "" {
			doc.TenantID = "tenant-1"
		}
		if doc.FileName == "" {
			doc.FileName = doc.ID + ".pdf"
		}
		if doc.ReviewStatus == "" {
			doc.ReviewStatus = documents.ReviewStatusApproved
		}
		if doc.LifecycleStatus == "" {
			doc.LifecycleStatus = documents.LifecycleStatusActive
		}
		if doc.UploadedBy == "" {
			doc.UploadedBy = "user-1"
		}
		if doc.CreatedAt.IsZero() {
			doc.CreatedAt = now
		}
		repo.docs[doc.ID] = &doc
	}
	return repo
}

func installApprovedClosePackEvidence(t *testing.T, h *Handlers, tenantID, periodEndDate string) {
	t.Helper()

	entityID, err := accounting.YearEndCloseEvidenceEntityID(tenantID, periodEndDate)
	require.NoError(t, err)
	installApprovedEvidenceDocuments(t, h, documents.Document{
		ID:           "doc-close-pack-" + periodEndDate,
		EntityType:   documents.EntityTypeYearEndClose,
		EntityID:     entityID,
		DocumentType: documents.DocumentTypeClosePack,
	})
}

func TestRequiredEvidenceGuardsFailClosedWithoutDocumentService(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	ctx := context.Background()

	err := h.requireApprovedJournalEntryEvidence(ctx, "tenant_test", "tenant-1", &accounting.JournalEntry{
		ID:               "entry-1",
		RequiresEvidence: true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedJournalEntryEvidenceRequired)
	assert.Contains(t, err.Error(), "entry-1")

	err = h.requireApprovedJournalEntryEvidence(ctx, "tenant_test", "tenant-1", &accounting.JournalEntry{
		ID:               "entry-2",
		RequiresEvidence: false,
	})
	require.NoError(t, err)

	err = h.requireApprovedTSDSubmissionEvidence(ctx, "tenant_test", "tenant-1", "tsd-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedTSDSubmissionEvidenceRequired)

	err = h.requireApprovedTSDAcceptanceEvidence(ctx, "tenant_test", "tenant-1", "tsd-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedTSDAcceptanceEvidenceRequired)

	err = h.requireApprovedKMDSubmissionEvidence(ctx, "tenant_test", "tenant-1", "kmd-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedKMDSubmissionEvidenceRequired)

	err = h.requireApprovedKMDAcceptanceEvidence(ctx, "tenant_test", "tenant-1", "kmd-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedKMDAcceptanceEvidenceRequired)
}

func TestPilotEvidencePolicyHelperFailurePaths(t *testing.T) {
	ctx := context.Background()
	repo := newMockTenantRepository()
	tenantRecord := repo.addTestTenant("tenant-1", "Pilot", "pilot")
	tenantRecord.Settings.EvidencePolicyMode = tenant.EvidencePolicyModeBlockHighRisk
	h := &Handlers{tenantService: tenant.NewServiceWithRepository(repo)}

	require.NoError(t, h.requireApprovedJournalEntryEvidence(ctx, "tenant_pilot", "tenant-1", nil))
	repo.getTenantErr = fmt.Errorf("tenant settings unavailable")
	_, err := h.requiresHighRiskEvidence(ctx, "tenant-1")
	require.ErrorContains(t, err, "tenant settings unavailable")
	err = h.requireApprovedJournalEntryEvidence(ctx, "tenant_pilot", "tenant-1", &accounting.JournalEntry{ID: "entry-1"})
	require.ErrorContains(t, err, "load tenant evidence policy")
	err = h.requireApprovedExpensePostingEvidence(ctx, "tenant_pilot", "tenant-1", "expense-1")
	require.ErrorContains(t, err, "load tenant evidence policy")
	repo.getTenantErr = nil

	err = h.requireApprovedExpensePostingEvidence(ctx, "tenant_pilot", "tenant-1", "expense-1")
	require.ErrorContains(t, err, "approved expense ledger-posting evidence")

	repo.createAuditEventErr = fmt.Errorf("audit store unavailable")
	h.recordHighRiskEvidenceBlock(ctx, "tenant-1", "user-1", "journal_post")

	assert.Empty(t, evidencePolicyActorID(ctx))
	assert.Equal(t, "user-1", evidencePolicyActorID(contextWithClaims(ctx, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner))))
}

func TestYearEndClosePackEvidenceGuardFailsClosedOnlyForFiscalYearEnd(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	tenantRecord := &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
		Settings: tenant.TenantSettings{
			FiscalYearStart: 1,
		},
	}

	err := h.requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "2026-12-31")
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedClosePackEvidenceRequired)
	assert.Contains(t, err.Error(), "2026-12-31")

	err = h.requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "2026-11-30")
	require.NoError(t, err)

	err = h.requireApprovedYearEndClosePackEvidence(context.Background(), &tenant.Tenant{
		Settings: tenant.TenantSettings{EvidencePolicyMode: tenant.EvidencePolicyModeBlockHighRisk},
	}, "2026-11-30")
	require.ErrorContains(t, err, "tenant id is required")
}

func TestPilotEvidencePolicyRequiresJournalEvidenceWithoutEntryFlag(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	tenantRecord := repo.addTestTenant("tenant-1", "Pilot", "pilot")
	tenantRecord.Settings = tenant.DefaultSettings()
	tenantRecord.Settings.EvidencePolicyMode = tenant.EvidencePolicyModeBlockHighRisk

	entry := &accounting.JournalEntry{ID: "entry-1", TenantID: "tenant-1", RequiresEvidence: false}
	err := h.requireApprovedJournalEntryEvidence(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, entry)
	require.ErrorIs(t, err, errApprovedJournalEntryEvidenceRequired)

	installApprovedEvidenceDocuments(t, h, documents.Document{
		EntityType:   documents.EntityTypeJournalEntry,
		EntityID:     entry.ID,
		DocumentType: documents.DocumentTypeSupportingDocument,
	})
	err = h.requireApprovedJournalEntryEvidence(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, entry)
	require.NoError(t, err)
}
