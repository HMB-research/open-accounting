package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

type archiveCoverageHandlerStore struct {
	status   importdelivery.Status
	manifest importdelivery.Manifest
	records  []json.RawMessage
}

func (s archiveCoverageHandlerStore) CreateManifest(context.Context, string, string, importdelivery.Manifest) (importdelivery.Status, error) {
	return s.status, nil
}
func (s archiveCoverageHandlerStore) GetStatus(context.Context, string, string, string) (importdelivery.Status, error) {
	return s.status, nil
}
func (s archiveCoverageHandlerStore) GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error) {
	return s.manifest, nil
}
func (s archiveCoverageHandlerStore) IterateRecords(_ context.Context, _, _, _ string, visit func(json.RawMessage) error) error {
	for _, record := range s.records {
		if err := visit(record); err != nil {
			return err
		}
	}
	return nil
}
func (s archiveCoverageHandlerStore) PutRecordChunk(context.Context, string, string, string, importdelivery.StoredRecordChunk) (importdelivery.ChunkResult, error) {
	return importdelivery.ChunkResult{}, nil
}
func (s archiveCoverageHandlerStore) PutArtifactChunk(context.Context, string, string, string, string, importdelivery.StoredArtifactChunk) (importdelivery.ChunkResult, error) {
	return importdelivery.ChunkResult{}, nil
}
func (s archiveCoverageHandlerStore) ListRecordChunks(context.Context, string, string, string) ([]importdelivery.StoredRecordChunk, error) {
	return nil, nil
}
func (s archiveCoverageHandlerStore) ListArtifactChunks(context.Context, string, string, string, string) ([]importdelivery.StoredArtifactChunk, error) {
	return nil, nil
}
func (s archiveCoverageHandlerStore) Finalize(context.Context, string, string, string, string, time.Time) (importdelivery.Status, error) {
	return s.status, nil
}

func TestSmartAccountsPackageArchiveCoverageHandlerReturnsSafeCountOnlyReport(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	manifest := importdelivery.Manifest{PackageID: "package-1", Provider: "smartaccounts", SourceCompanyID: "source-1", ManifestSHA256: digest, PackageSHA256: digest, Scope: importdelivery.Scope{Mode: "partial_browser_capture"}}
	record := json.RawMessage(`{"entity_type":"browser_capture_evidence","source_company_id":"source-1","resource":"client_invoices","source_schema":"smartaccounts-browser-commercial-detail-v1/client_invoices_detail_v1","operation":"upsert","gl_posting_mode":"review_required","review_reason":"commercial_detail_identity_review_required","payload":{"private":"must-not-leak"}}`)
	service := importdelivery.NewService(archiveCoverageHandlerStore{
		status:   importdelivery.Status{PackageID: manifest.PackageID, TenantID: "tenant-1", SourceCompanyID: manifest.SourceCompanyID, Status: importdelivery.StatusStagedReview, PackageSHA256: digest, ManifestSHA256: digest, RecordCount: 1},
		manifest: manifest,
		records:  []json.RawMessage{record},
	}, nil)
	h := &Handlers{importDeliveryService: service}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("tenantID", "tenant-1")
	routeContext.URLParams.Add("packageID", "package-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()

	h.GetSmartAccountsPackageArchiveCoverage(response, req)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.NotContains(t, response.Body.String(), "must-not-leak")
	require.NotContains(t, response.Body.String(), "source-1")
	var report importdelivery.CoverageReport
	require.NoError(t, json.NewDecoder(response.Body).Decode(&report))
	require.Equal(t, 1, report.ReviewRequiredRecordCount)
}
