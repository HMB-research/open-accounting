package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreferences"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

type referenceHandlerArchive struct{ record json.RawMessage }

func (referenceHandlerArchive) GetStatus(context.Context, string, string, string) (importdelivery.Status, error) {
	return importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: "source"}, nil
}
func (referenceHandlerArchive) GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error) {
	return importdelivery.Manifest{Provider: "smartaccounts", SourceCompanyID: "source"}, nil
}
func (a referenceHandlerArchive) IterateRecords(_ context.Context, _, _, _ string, visit func(json.RawMessage) error) error {
	return visit(a.record)
}

type referenceHandlerStore struct {
	preview    *smartaccountsreferences.StoredPreview
	identities map[string]*smartaccountsreferences.Identity
}

func (s *referenceHandlerStore) SavePreview(_ context.Context, _ string, p *smartaccountsreferences.StoredPreview, _ string) error {
	s.preview = p
	return nil
}
func (s *referenceHandlerStore) GetPreview(_ context.Context, _, tenant, id string) (*smartaccountsreferences.StoredPreview, error) {
	if s.preview == nil || s.preview.Preview.ID != id || s.preview.Preview.TenantID != tenant {
		return nil, smartaccountsreferences.ErrPreviewNotFound
	}
	return s.preview, nil
}
func (s *referenceHandlerStore) GetLatestPreviewForPackage(_ context.Context, _, tenant, packageID string) (*smartaccountsreferences.StoredPreview, error) {
	if s.preview == nil || s.preview.Preview.TenantID != tenant || s.preview.Preview.PackageID != packageID {
		return nil, smartaccountsreferences.ErrPreviewNotFound
	}
	return s.preview, nil
}
func (s *referenceHandlerStore) GetIdentity(context.Context, string, string, string, string, string, string) (*smartaccountsreferences.Identity, error) {
	return nil, nil
}
func (s *referenceHandlerStore) ReserveIdentity(_ context.Context, _, _, _, _, entity, external, revision, target string) (*smartaccountsreferences.Identity, bool, error) {
	if s.identities == nil {
		s.identities = map[string]*smartaccountsreferences.Identity{}
	}
	key := entity + "/" + external
	if found := s.identities[key]; found != nil {
		return found, false, nil
	}
	item := &smartaccountsreferences.Identity{EntityType: entity, ExternalID: external, Revision: revision, TargetID: target, Status: "PENDING"}
	s.identities[key] = item
	return item, true, nil
}
func (s *referenceHandlerStore) MarkIdentityApplied(_ context.Context, _, _, _, _, entity, external string) error {
	s.identities[entity+"/"+external].Status = "APPLIED"
	return nil
}
func (s *referenceHandlerStore) MarkPreviewApplied(context.Context, string, string, string) error {
	s.preview.Preview.Status = "APPLIED"
	return nil
}

type referenceHandlerWriter struct{ accounts int }

func (w *referenceHandlerWriter) EnsureAccount(context.Context, string, string, *accounting.CreateAccountRequest) error {
	w.accounts++
	return nil
}
func (*referenceHandlerWriter) EnsureContact(context.Context, string, string, *contacts.CreateContactRequest) error {
	return nil
}
func (*referenceHandlerWriter) EnsureProduct(context.Context, string, string, *inventory.CreateProductRequest) error {
	return nil
}

type referenceHandlerCatalog struct{}

func (referenceHandlerCatalog) ListAccounts(context.Context, string, string) ([]accounting.Account, error) {
	return nil, nil
}
func (referenceHandlerCatalog) ListContacts(context.Context, string, string) ([]contacts.Contact, error) {
	return nil, nil
}
func (referenceHandlerCatalog) ListProducts(context.Context, string, string) ([]inventory.Product, error) {
	return nil, nil
}

func TestSmartAccountsReferenceMasterHandlersAreConfirmedOnly(t *testing.T) {
	payload, err := json.Marshal(map[string]string{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	require.NoError(t, err)
	digest := sha256.Sum256(payload)
	record, err := json.Marshal(map[string]any{"entity_type": "account", "source_schema": "smartaccounts-api-v1.7/account_v1", "external_id": "a-1000", "revision": hex.EncodeToString(digest[:]), "operation": "upsert", "payload": json.RawMessage(payload), "payload_sha256": hex.EncodeToString(digest[:]), "source_company_id": "source", "resource": "settings.accounts.get", "gl_posting_mode": "non_posting_reference"})
	require.NoError(t, err)
	store := &referenceHandlerStore{}
	writer := &referenceHandlerWriter{}
	service := smartaccountsreferences.NewService(referenceHandlerArchive{record: record}, store, writer, referenceHandlerCatalog{})
	h := &Handlers{smartAccountsReferenceService: service}
	request := func(method, path, body string) *http.Request {
		r := httptest.NewRequest(method, path, httptestBody(body))
		ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, &auth.Claims{UserID: "user"})
		rc := chi.NewRouteContext()
		rc.URLParams.Add("tenantID", "tenant")
		rc.URLParams.Add("packageID", "package")
		return r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rc))
	}
	previewResponse := httptest.NewRecorder()
	h.PreviewSmartAccountsReferenceMasters(previewResponse, request(http.MethodPost, "/", `{}`))
	require.Equal(t, http.StatusOK, previewResponse.Code, previewResponse.Body.String())
	var preview smartaccountsreferences.Preview
	require.NoError(t, json.NewDecoder(previewResponse.Body).Decode(&preview))
	require.Len(t, preview.Actions, 1)
	applyResponse := httptest.NewRecorder()
	h.ApplySmartAccountsReferenceMasters(applyResponse, request(http.MethodPost, "/", `{"preview_id":"`+preview.ID+`","preview_sha256":"`+preview.PreviewSHA256+`"}`))
	require.Equal(t, http.StatusConflict, applyResponse.Code)
	require.Zero(t, writer.accounts)
	applyResponse = httptest.NewRecorder()
	h.ApplySmartAccountsReferenceMasters(applyResponse, request(http.MethodPost, "/", `{"confirm":true,"preview_id":"`+preview.ID+`","preview_sha256":"`+preview.PreviewSHA256+`"}`))
	require.Equal(t, http.StatusOK, applyResponse.Code, applyResponse.Body.String())
	require.Equal(t, 1, writer.accounts)
	require.NotContains(t, applyResponse.Body.String(), "descriptionEt")
}

func httptestBody(value string) *strings.Reader { return strings.NewReader(value) }
