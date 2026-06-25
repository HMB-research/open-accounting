package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wave12ArchiveWriter struct {
	writers     []io.Writer
	createErrs  []error
	closeErr    error
	createCalls int
}

func (w *wave12ArchiveWriter) Create(string) (io.Writer, error) {
	call := w.createCalls
	w.createCalls++
	if call < len(w.createErrs) && w.createErrs[call] != nil {
		return nil, w.createErrs[call]
	}
	if call < len(w.writers) && w.writers[call] != nil {
		return w.writers[call], nil
	}
	return io.Discard, nil
}

func (w *wave12ArchiveWriter) Close() error {
	return w.closeErr
}

type wave12ErrorWriter struct {
	err error
}

func (w wave12ErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func restoreWave12APISeams(t *testing.T) {
	t.Helper()
	oldEval := evaluateDocumentsEvidencePolicy
	oldMarshal := marshalYearEndCloseAuditManifest
	oldArchiveWriter := newYearEndCloseAuditArchiveWriter
	oldMainDeps := mainDepsProvider
	oldMainExit := apiMainExit
	oldConfigFatalExit := configFatalExit
	t.Cleanup(func() {
		evaluateDocumentsEvidencePolicy = oldEval
		marshalYearEndCloseAuditManifest = oldMarshal
		newYearEndCloseAuditArchiveWriter = oldArchiveWriter
		mainDepsProvider = oldMainDeps
		apiMainExit = oldMainExit
		configFatalExit = oldConfigFatalExit
	})
}

func TestWave12EvidencePolicyEmptyResults(t *testing.T) {
	restoreWave12APISeams(t)
	evaluateDocumentsEvidencePolicy = func(*documents.Service, context.Context, string, string, *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
		return nil, nil
	}

	t.Run("purchase invoice", func(t *testing.T) {
		h, _, invoiceRepo := setupInvoiceTestHandlers()
		h.documentsService = documents.NewService(newMockDocumentRepository(), nil)
		invoiceRepo.addTestInvoice("bill-1", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)

		err := h.requireApprovedPurchaseInvoiceEvidence(context.Background(), "tenant_test", "tenant-1", "bill-1")

		require.ErrorIs(t, err, errApprovedPurchaseInvoiceEvidenceRequired)
	})

	t.Run("commercial document", func(t *testing.T) {
		h := &Handlers{documentsService: documents.NewService(newMockDocumentRepository(), nil)}

		err := h.requireApprovedCommercialEvidence(context.Background(), "tenant_test", "tenant-1", documents.EntityTypeQuote, "quote-1", true, errApprovedQuoteEvidenceRequired, "sending quote")

		require.ErrorIs(t, err, errApprovedQuoteEvidenceRequired)
	})

	t.Run("asset activation and disposal", func(t *testing.T) {
		h, repo, _ := setupAssetsTestHandlers()
		h.documentsService = documents.NewService(newMockDocumentRepository(), nil)
		repo.assets["draft-asset"] = wave9Asset("draft-asset", assets.AssetStatusDraft)
		repo.assets["active-asset"] = wave9Asset("active-asset", assets.AssetStatusActive)

		err := h.requireApprovedAssetActivationEvidence(context.Background(), "tenant_test", "tenant-1", "draft-asset")
		require.ErrorIs(t, err, errApprovedAssetActivationEvidenceRequired)

		err = h.requireApprovedAssetDisposalEvidence(context.Background(), "tenant_test", "tenant-1", "active-asset")
		require.ErrorIs(t, err, errApprovedAssetDisposalEvidenceRequired)
	})

	t.Run("year-end close evidence status", func(t *testing.T) {
		h := &Handlers{documentsService: documents.NewService(newMockDocumentRepository(), nil)}
		status := &accounting.YearEndCloseStatus{
			ClosePackEvidenceEntityID: "close-pack-1",
			CarryForwardReady:         true,
		}

		err := h.attachYearEndCloseEvidenceStatus(context.Background(), "tenant_test", "tenant-1", status)

		require.NoError(t, err)
		assert.Nil(t, status.ClosePackEvidence)
		assert.True(t, status.CarryForwardReady)
	})
}

func TestWave12YearEndCloseAuditArchiveWriterErrors(t *testing.T) {
	tenantRecord := &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
	audit := &accounting.YearEndCloseAuditEvidence{}

	t.Run("manifest marshal", func(t *testing.T) {
		restoreWave12APISeams(t)
		marshalYearEndCloseAuditManifest = func(any, string, string) ([]byte, error) {
			return nil, errors.New("marshal denied")
		}

		archive, err := (&Handlers{}).buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)

		require.ErrorContains(t, err, "encode audit manifest")
		assert.Nil(t, archive)
	})

	t.Run("manifest create", func(t *testing.T) {
		restoreWave12APISeams(t)
		newYearEndCloseAuditArchiveWriter = func(io.Writer) yearEndCloseAuditArchiveWriter {
			return &wave12ArchiveWriter{createErrs: []error{errors.New("create denied")}}
		}

		archive, err := (&Handlers{}).buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)

		require.ErrorContains(t, err, "create audit manifest")
		assert.Nil(t, archive)
	})

	t.Run("manifest write", func(t *testing.T) {
		restoreWave12APISeams(t)
		newYearEndCloseAuditArchiveWriter = func(io.Writer) yearEndCloseAuditArchiveWriter {
			return &wave12ArchiveWriter{writers: []io.Writer{wave12ErrorWriter{err: errors.New("write denied")}}}
		}

		archive, err := (&Handlers{}).buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)

		require.ErrorContains(t, err, "write audit manifest")
		assert.Nil(t, archive)
	})

	t.Run("document entry create", func(t *testing.T) {
		restoreWave12APISeams(t)
		docRepo := newMockDocumentRepository()
		docRepo.docs["doc-1"] = &documents.Document{ID: "doc-1", TenantID: "tenant-1", FileName: "close pack.pdf", StorageKey: "close-pack.pdf"}
		h := &Handlers{documentsService: documents.NewService(docRepo, &wave6DocumentStore{reader: io.NopCloser(strings.NewReader("document"))})}
		auditWithDoc := &accounting.YearEndCloseAuditEvidence{Documents: []documents.Document{{ID: "doc-1"}}}
		newYearEndCloseAuditArchiveWriter = func(io.Writer) yearEndCloseAuditArchiveWriter {
			return &wave12ArchiveWriter{createErrs: []error{nil, errors.New("entry denied")}}
		}

		archive, err := h.buildYearEndCloseAuditArchive(context.Background(), tenantRecord, auditWithDoc)

		require.ErrorContains(t, err, "create archive document entry")
		assert.Nil(t, archive)
	})

	t.Run("archive close", func(t *testing.T) {
		restoreWave12APISeams(t)
		newYearEndCloseAuditArchiveWriter = func(io.Writer) yearEndCloseAuditArchiveWriter {
			return &wave12ArchiveWriter{closeErr: errors.New("close denied")}
		}

		archive, err := (&Handlers{}).buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)

		require.ErrorContains(t, err, "close audit archive")
		assert.Nil(t, archive)
	})
}

func TestWave12MainAndLoadConfigFatalSeams(t *testing.T) {
	restoreWave12APISeams(t)

	var mainExitCode int
	mainDepsProvider = func() apiMainDeps {
		deps := newRunAPITestDeps(nil, &Config{DatabaseURL: "postgres://unit"}, nil, nil, nil)
		deps.newPool = func(context.Context, string) (apiPool, error) {
			return nil, errors.New("connect failed")
		}
		return deps
	}
	apiMainExit = func(code int) {
		mainExitCode = code
	}

	main()

	require.Equal(t, 1, mainExitCode)

	configFatalCalls := 0
	configFatalExit = func(code int) {
		require.Equal(t, 1, code)
		configFatalCalls++
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("APP_ENV", "")
	_ = loadConfig()
	require.Equal(t, 1, configFatalCalls)

	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")
	_ = loadConfig()
	require.Equal(t, 2, configFatalCalls)

	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("ALLOWED_ORIGINS", "")
	_ = loadConfig()
	require.Equal(t, 3, configFatalCalls)
}
