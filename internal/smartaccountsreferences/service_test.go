package smartaccountsreferences

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type referenceArchive struct {
	status   importdelivery.Status
	manifest importdelivery.Manifest
	records  []json.RawMessage
}

// referenceDeliveryStore is intentionally tiny but drives the real bounded
// importdelivery service in the end-to-end test below. It demonstrates that a
// master-detail reference preview cannot bypass raw delivery finalization.
type referenceDeliveryStore struct {
	manifest importdelivery.Manifest
	status   importdelivery.Status
	chunks   []importdelivery.StoredRecordChunk
}

func (s *referenceDeliveryStore) CreateManifest(_ context.Context, _ string, tenant string, manifest importdelivery.Manifest) (importdelivery.Status, error) {
	if s.status.PackageID != "" {
		if s.status.TenantID != tenant || s.manifest.ManifestSHA256 != manifest.ManifestSHA256 || s.manifest.PackageSHA256 != manifest.PackageSHA256 {
			return importdelivery.Status{}, importdelivery.ErrChunkConflict
		}
		return s.status, nil
	}
	s.manifest = manifest
	s.status = importdelivery.Status{PackageID: manifest.PackageID, TenantID: tenant, SourceCompanyID: manifest.SourceCompanyID, Status: importdelivery.StatusReceiving, ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordCount: manifest.RecordCount, ArtifactCount: len(manifest.Artifacts), CreatedAt: time.Now().UTC()}
	return s.status, nil
}
func (s *referenceDeliveryStore) GetStatus(_ context.Context, _ string, tenant, packageID string) (importdelivery.Status, error) {
	if s.status.TenantID != tenant || s.status.PackageID != packageID {
		return importdelivery.Status{}, importdelivery.ErrNotFound
	}
	result := s.status
	result.RecordChunks, result.NextRecordSequence = len(s.chunks), len(s.chunks)
	return result, nil
}
func (s *referenceDeliveryStore) GetManifest(_ context.Context, _ string, tenant, packageID string) (importdelivery.Manifest, error) {
	if s.status.TenantID != tenant || s.status.PackageID != packageID {
		return importdelivery.Manifest{}, importdelivery.ErrNotFound
	}
	return s.manifest, nil
}
func (s *referenceDeliveryStore) PutRecordChunk(_ context.Context, _ string, tenant, packageID string, chunk importdelivery.StoredRecordChunk) (importdelivery.ChunkResult, error) {
	if s.status.TenantID != tenant || s.status.PackageID != packageID || s.status.Status != importdelivery.StatusReceiving {
		return importdelivery.ChunkResult{}, importdelivery.ErrNotFound
	}
	if chunk.Sequence != len(s.chunks) {
		return importdelivery.ChunkResult{}, importdelivery.ErrChunkOutOfOrder
	}
	s.chunks = append(s.chunks, chunk)
	return importdelivery.ChunkResult{Status: "records_accepted", NextRecordSequence: len(s.chunks), Created: true}, nil
}
func (*referenceDeliveryStore) PutArtifactChunk(context.Context, string, string, string, string, importdelivery.StoredArtifactChunk) (importdelivery.ChunkResult, error) {
	return importdelivery.ChunkResult{}, importdelivery.ErrChunkInvalid
}
func (s *referenceDeliveryStore) ListRecordChunks(_ context.Context, _ string, tenant, packageID string) ([]importdelivery.StoredRecordChunk, error) {
	if s.status.TenantID != tenant || s.status.PackageID != packageID {
		return nil, importdelivery.ErrNotFound
	}
	return append([]importdelivery.StoredRecordChunk(nil), s.chunks...), nil
}
func (*referenceDeliveryStore) ListArtifactChunks(context.Context, string, string, string, string) ([]importdelivery.StoredArtifactChunk, error) {
	return nil, nil
}
func (s *referenceDeliveryStore) Finalize(_ context.Context, _ string, tenant, packageID, stagedID string, at time.Time) (importdelivery.Status, error) {
	if s.status.TenantID != tenant || s.status.PackageID != packageID {
		return importdelivery.Status{}, importdelivery.ErrNotFound
	}
	s.status.Status, s.status.StagedSessionID, s.status.UpdatedAt = importdelivery.StatusStagedReview, stagedID, at
	return s.GetStatus(context.Background(), "", tenant, packageID)
}
func (s *referenceDeliveryStore) IterateRecords(ctx context.Context, schema, tenant, packageID string, visit func(json.RawMessage) error) error {
	status, err := s.GetStatus(ctx, schema, tenant, packageID)
	if err != nil || status.Status != importdelivery.StatusStagedReview {
		return importdelivery.ErrFinalizeIncomplete
	}
	for _, chunk := range s.chunks {
		for _, line := range bytes.Split(bytes.TrimSpace(chunk.Data), []byte{'\n'}) {
			if len(line) > 0 {
				if err := visit(append(json.RawMessage(nil), line...)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type referenceDeliveryBinder struct{ tenant, source string }

func (b referenceDeliveryBinder) EnsureSourceCompanyBinding(_ context.Context, tenant, provider, source string) error {
	if provider != Provider || tenant != b.tenant || source != b.source {
		return importdelivery.ErrSourceBindingConflict
	}
	return nil
}

func (a *referenceArchive) GetStatus(context.Context, string, string, string) (importdelivery.Status, error) {
	return a.status, nil
}
func (a *referenceArchive) GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error) {
	return a.manifest, nil
}
func (a *referenceArchive) IterateRecords(_ context.Context, _, _, _ string, visit func(json.RawMessage) error) error {
	for _, record := range a.records {
		if err := visit(record); err != nil {
			return err
		}
	}
	return nil
}

type referenceStore struct {
	previews   map[string]*StoredPreview
	identities map[string]*Identity
}

func (s *referenceStore) SavePreview(_ context.Context, _ string, p *StoredPreview, _ string) error {
	if s.previews == nil {
		s.previews = map[string]*StoredPreview{}
	}
	s.previews[p.Preview.ID] = p
	return nil
}
func (s *referenceStore) GetPreview(_ context.Context, _, tenant, id string) (*StoredPreview, error) {
	p := s.previews[id]
	if p == nil || p.Preview.TenantID != tenant {
		return nil, ErrPreviewNotFound
	}
	return p, nil
}
func (s *referenceStore) GetLatestPreviewForPackage(_ context.Context, _ string, tenant, packageID string) (*StoredPreview, error) {
	for _, preview := range s.previews {
		if preview.Preview.TenantID == tenant && preview.Preview.PackageID == packageID {
			return preview, nil
		}
	}
	return nil, ErrPreviewNotFound
}
func identityKey(tenant, provider, source, entity, external string) string {
	return tenant + "/" + provider + "/" + source + "/" + entity + "/" + external
}
func (s *referenceStore) GetIdentity(_ context.Context, _, tenant, provider, source, entity, external string) (*Identity, error) {
	return s.identities[identityKey(tenant, provider, source, entity, external)], nil
}
func (s *referenceStore) ReserveIdentity(_ context.Context, _, tenant, provider, source, entity, external, revision, target string) (*Identity, bool, error) {
	if s.identities == nil {
		s.identities = map[string]*Identity{}
	}
	key := identityKey(tenant, provider, source, entity, external)
	if found := s.identities[key]; found != nil {
		return found, false, nil
	}
	created := &Identity{EntityType: entity, ExternalID: external, Revision: revision, TargetID: target, Status: IdentityPending}
	s.identities[key] = created
	return created, true, nil
}
func (s *referenceStore) MarkIdentityApplied(_ context.Context, _, tenant, provider, source, entity, external string) error {
	s.identities[identityKey(tenant, provider, source, entity, external)].Status = IdentityApplied
	return nil
}
func (s *referenceStore) MarkPreviewApplied(_ context.Context, _, tenant, id string) error {
	s.previews[id].Preview.Status = StatusApplied
	return nil
}

type referenceWriter struct {
	accounts, contacts, products int
	err                          error
}

func (w *referenceWriter) EnsureAccount(context.Context, string, string, *accounting.CreateAccountRequest) error {
	if w.err != nil {
		return w.err
	}
	w.accounts++
	return nil
}
func (w *referenceWriter) EnsureContact(context.Context, string, string, *contacts.CreateContactRequest) error {
	if w.err != nil {
		return w.err
	}
	w.contacts++
	return nil
}
func (w *referenceWriter) EnsureProduct(context.Context, string, string, *inventory.CreateProductRequest) error {
	if w.err != nil {
		return w.err
	}
	w.products++
	return nil
}

type referenceCatalog struct{}

func (referenceCatalog) ListAccounts(context.Context, string, string) ([]accounting.Account, error) {
	return nil, nil
}
func (referenceCatalog) ListContacts(context.Context, string, string) ([]contacts.Contact, error) {
	return nil, nil
}
func (referenceCatalog) ListProducts(context.Context, string, string) ([]inventory.Product, error) {
	return nil, nil
}

func canonicalRecord(t *testing.T, entity, schema, resource, external string, payload any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	record := bridgeRecord{EntityType: entity, SourceSchema: schema, ExternalID: external, Revision: digest, Operation: "upsert", Payload: raw, PayloadSHA256: digest, SourceCompanyID: "source", Resource: resource, GLPostingMode: "non_posting_reference"}
	out, err := json.Marshal(record)
	require.NoError(t, err)
	return out
}

func browserMasterDetailRecord(t *testing.T, entity, schema, resource, external string, payload any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	record := bridgeRecord{EntityType: entity, SourceSchema: schema, ExternalID: external, ExternalIDMode: BrowserDetailExternalIDMode, Revision: digest, Operation: "upsert", Payload: raw, PayloadSHA256: digest, SourceCompanyID: "source", Resource: resource, GLPostingMode: "non_posting_reference", Relationship: BrowserDetailRelationship}
	out, err := json.Marshal(record)
	require.NoError(t, err)
	return out
}

func browserPartyPayload(external, partyID, name, country string) map[string]any {
	return map[string]any{
		"source_party_id":   partyID,
		"name":              name,
		"registration_code": "reg-1",
		"vat_number":        "EE123",
		"address": map[string]any{
			"country_code": country,
			"county":       "Harju",
			"city":         "Tallinn",
			"line1":        "Example 1",
			"line2":        "Suite 2",
			"postal_code":  "10111",
		},
		"source_detail": map[string]any{
			"id":              external,
			"path_sha256":     strings.Repeat("a", 64),
			"contract_sha256": strings.Repeat("b", 64),
		},
	}
}
func readyReferenceService(t *testing.T, records ...json.RawMessage) (*Service, *referenceStore, *referenceWriter) {
	t.Helper()
	archive := &referenceArchive{status: importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: "source"}, manifest: importdelivery.Manifest{Provider: Provider, SourceCompanyID: "source"}, records: records}
	store := &referenceStore{identities: map[string]*Identity{}}
	writer := &referenceWriter{}
	return NewService(archive, store, writer, referenceCatalog{}), store, writer
}

func TestReferencePreviewApplyIsConfirmedTenantBoundAndIdempotent(t *testing.T) {
	account := canonicalRecord(t, EntityAccount, "smartaccounts-api-v1.7/account_v1", "settings.accounts.get", "a-1000", map[string]any{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	client := canonicalRecord(t, EntityCustomer, "smartaccounts-api-v1.7/customer_v1", "purchasesales.clients.get", "client-1", map[string]any{"id": "client-1", "name": "Client", "address": map[string]any{"country": "EE", "city": "Tallinn"}, "contacts": []any{map[string]any{"type": "EMAIL", "value": "client@example.invalid"}}})
	vendor := canonicalRecord(t, EntityVendor, "smartaccounts-api-v1.7/vendor_v1", "purchasesales.vendors.get", "vendor-1", map[string]any{"id": "vendor-1", "name": "Vendor", "address": map[string]any{"country": "EE"}})
	item := canonicalRecord(t, EntityItem, "smartaccounts-api-v1.7/article_v1", "purchasesales.articles.get", "item-1", map[string]any{"code": "item-1", "description": "Service", "type": "SERVICE", "unit": "pc", "vatPc": "24", "priceSales": "12.50", "activeSales": true, "activePurchase": true})
	svc, store, writer := readyReferenceService(t, account, client, vendor, item)
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{})
	require.NoError(t, err)
	require.Equal(t, StatusPreviewReady, preview.Status)
	assert.Len(t, preview.Actions, 4)
	assert.Len(t, preview.Reconciliation, 4)
	_, err = svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.ErrorIs(t, err, ErrConfirmationRequired)
	applied, err := svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, StatusApplied, applied.Status)
	assert.Equal(t, 1, writer.accounts)
	assert.Equal(t, 2, writer.contacts)
	assert.Equal(t, 1, writer.products)
	_, err = svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, 1, writer.accounts)
	assert.Equal(t, 2, writer.contacts)
	assert.Equal(t, 1, writer.products)
	_, err = store.GetPreview(context.Background(), "schema", "other-tenant", preview.ID)
	require.ErrorIs(t, err, ErrPreviewNotFound)
}

func TestReferencePreviewSupportsSubsetAndReviewsRevisionsAndTombstones(t *testing.T) {
	account := canonicalRecord(t, EntityAccount, "smartaccounts-api-v1.7/account_v1", "settings.accounts.get", "a-1000", map[string]any{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	item := canonicalRecord(t, EntityItem, "smartaccounts-api-v1.7/article_v1", "purchasesales.articles.get", "item-1", map[string]any{"code": "item-1", "description": "Service", "type": "SERVICE", "unit": "pc", "vatPc": "24", "priceSales": "12.50", "activeSales": true, "activePurchase": true})
	svc, store, _ := readyReferenceService(t, account, item)
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{EntityTypes: []string{EntityAccount}})
	require.NoError(t, err)
	assert.Len(t, preview.Actions, 1)
	assert.Equal(t, EntityAccount, preview.Actions[0].EntityType)
	stored := store.previews[preview.ID]
	store.identities[identityKey("tenant", Provider, "source", EntityAccount, "a-1000")] = &Identity{EntityType: EntityAccount, ExternalID: "a-1000", Revision: "different", TargetID: stored.Actions[0].TargetID, Status: IdentityApplied}
	conflict, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{EntityTypes: []string{EntityAccount}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "source_revision_review_required", conflict.Issues[0].Code)

	tombstone := bridgeRecord{EntityType: EntityAccount, SourceSchema: "smartaccounts-api-v1.7/account_v1", ExternalID: "a-deleted", Revision: strings.Repeat("a", 64), Operation: "tombstone", SourceCompanyID: "source", Resource: "settings.accounts.get", GLPostingMode: "review_required"}
	raw, _ := json.Marshal(tombstone)
	svc, _, _ = readyReferenceService(t, raw)
	review, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{EntityTypes: []string{EntityAccount}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "source_tombstone_review_required", review.Issues[0].Code)
}

func TestReferencePreviewResumesExactPendingIdentityWithoutOverwritingRevision(t *testing.T) {
	account := canonicalRecord(t, EntityAccount, "smartaccounts-api-v1.7/account_v1", "settings.accounts.get", "a-1000", map[string]any{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	svc, store, writer := readyReferenceService(t, account)
	first, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{})
	require.NoError(t, err)
	require.Len(t, first.Actions, 1)

	// Simulate a process interruption after identity reservation but before the
	// target write completed. The same canonical revision is safe to resume;
	// a changed revision remains review-only in the preceding test.
	store.identities[identityKey("tenant", Provider, "source", EntityAccount, "a-1000")] = &Identity{
		EntityType: EntityAccount,
		ExternalID: "a-1000",
		Revision:   first.Actions[0].Revision,
		TargetID:   first.Actions[0].TargetID,
		Status:     IdentityPending,
	}
	resumed, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{EntityTypes: []string{EntityAccount}})
	require.NoError(t, err)
	require.Equal(t, StatusPreviewReady, resumed.Status)
	require.Len(t, resumed.Actions, 1)
	assert.Equal(t, "RESUME", resumed.Actions[0].Action)

	_, err = svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: resumed.ID, PreviewSHA256: resumed.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, 1, writer.accounts)
	assert.Equal(t, IdentityApplied, store.identities[identityKey("tenant", Provider, "source", EntityAccount, "a-1000")].Status)
}

func TestReferenceApplyDoesNotTreatAnUnrelatedUniqueCollisionAsSuccess(t *testing.T) {
	account := canonicalRecord(t, EntityAccount, "smartaccounts-api-v1.7/account_v1", "settings.accounts.get", "a-1000", map[string]any{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	svc, store, writer := readyReferenceService(t, account)
	writer.err = errors.New("duplicate key value violates unique constraint accounts_code_key")
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{})
	require.NoError(t, err)

	_, err = svc.Apply(context.Background(), "schema", "tenant", "user", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.Error(t, err)
	assert.Equal(t, StatusPreviewReady, store.previews[preview.ID].Preview.Status)
	identity := store.identities[identityKey("tenant", Provider, "source", EntityAccount, "a-1000")]
	require.NotNil(t, identity)
	assert.Equal(t, IdentityPending, identity.Status)
}

func TestReferencePreviewRejectsRecordFromAnotherSourceBinding(t *testing.T) {
	account := canonicalRecord(t, EntityAccount, "smartaccounts-api-v1.7/account_v1", "settings.accounts.get", "a-1000", map[string]any{"id": "a-1000", "code": "1000", "type": "ASSET", "descriptionEt": "Cash"})
	var record bridgeRecord
	require.NoError(t, json.Unmarshal(account, &record))
	record.SourceCompanyID = "other-source"
	account, err := json.Marshal(record)
	require.NoError(t, err)
	svc, _, writer := readyReferenceService(t, account)
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "user", PreviewRequest{})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	require.Len(t, preview.Issues, 1)
	assert.Equal(t, "source_binding_mismatch", preview.Issues[0].Code)
	assert.Zero(t, writer.accounts)
}

func TestBrowserMasterDetailPartyPreviewAppliesOnlyExactConfirmedContacts(t *testing.T) {
	client := browserMasterDetailRecord(t, EntityCustomer, BrowserClientsDetailSchema, "clients", "101", browserPartyPayload("101", "client-1", "Client OÜ", "EE"))
	vendor := browserMasterDetailRecord(t, EntityVendor, BrowserVendorsDetailSchema, "vendors", "202", browserPartyPayload("202", "vendor-2", "Vendor OÜ", "FI"))
	svc, store, writer := readyReferenceService(t, client, vendor)

	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer, EntityVendor}})
	require.NoError(t, err)
	require.Equal(t, StatusPreviewReady, preview.Status)
	require.Len(t, preview.Actions, 2)
	assert.Equal(t, deterministicTargetID("source", EntityCustomer, "101"), preview.Actions[0].TargetID)
	assert.Equal(t, deterministicTargetID("source", EntityVendor, "202"), preview.Actions[1].TargetID)

	_, err = svc.Apply(context.Background(), "schema", "tenant", "owner", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, 2, writer.contacts)
	// Exact replay is a no-op; a reference preview never creates finance.
	_, err = svc.Apply(context.Background(), "schema", "tenant", "owner", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, 2, writer.contacts)
	require.Equal(t, IdentityApplied, store.identities[identityKey("tenant", Provider, "source", EntityCustomer, "101")].Status)
}

func TestBrowserMasterDetailPartyRejectsRevisionCollisionAndTenantMismatch(t *testing.T) {
	client := browserMasterDetailRecord(t, EntityCustomer, BrowserClientsDetailSchema, "clients", "101", browserPartyPayload("101", "client-1", "Client OÜ", "EE"))
	svc, store, _ := readyReferenceService(t, client)
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.NoError(t, err)
	store.identities[identityKey("tenant", Provider, "source", EntityCustomer, "101")] = &Identity{EntityType: EntityCustomer, ExternalID: "101", Revision: strings.Repeat("c", 64), TargetID: preview.Actions[0].TargetID, Status: IdentityApplied}
	review, err := svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "source_revision_review_required", review.Issues[0].Code)

	var malformed bridgeRecord
	require.NoError(t, json.Unmarshal(client, &malformed))
	malformed.ExternalIDMode = "other"
	raw, err := json.Marshal(malformed)
	require.NoError(t, err)
	svc, _, _ = readyReferenceService(t, raw)
	review, err = svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "source_schema_mismatch", review.Issues[0].Code)

	invalidCountry := browserMasterDetailRecord(t, EntityCustomer, BrowserClientsDetailSchema, "clients", "101", browserPartyPayload("101", "client-1", "Client OÜ", "E1"))
	svc, _, _ = readyReferenceService(t, invalidCountry)
	review, err = svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "reference_mapping_required", review.Issues[0].Code)

	var tombstone bridgeRecord
	require.NoError(t, json.Unmarshal(client, &tombstone))
	tombstone.Operation, tombstone.ReviewReason = "tombstone", "not_observable_from_detail_snapshot"
	raw, err = json.Marshal(tombstone)
	require.NoError(t, err)
	svc, _, _ = readyReferenceService(t, raw)
	review, err = svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, "source_tombstone_review_required", review.Issues[0].Code)
}

func TestBrowserMasterDetailArticleIsAlwaysVATReviewOnly(t *testing.T) {
	article := browserMasterDetailRecord(t, EntityItem, BrowserArticlesDetailSchema, "articles", "303", map[string]any{
		"code": "item-1", "name": "Widget", "product_type": "PRODUCT", "unit": "pcs", "sales_price": "12.50",
		"source_detail": map[string]any{"id": "item-1", "path_sha256": strings.Repeat("a", 64), "contract_sha256": strings.Repeat("b", 64)},
	})
	svc, _, writer := readyReferenceService(t, article)
	preview, err := svc.Preview(context.Background(), "schema", "tenant", "package", "owner", PreviewRequest{EntityTypes: []string{EntityItem}})
	require.ErrorIs(t, err, ErrPreviewReviewRequired)
	assert.Equal(t, StatusReviewRequired, preview.Status)
	require.Len(t, preview.Issues, 1)
	assert.Equal(t, "article_vat_mapping_review_required", preview.Issues[0].Code)
	assert.Zero(t, writer.products)
}

func TestBrowserMasterDetailDeliveryToPreviewToConfirmedContactReplay(t *testing.T) {
	client := browserMasterDetailRecord(t, EntityCustomer, BrowserClientsDetailSchema, "clients", "101", browserPartyPayload("101", "client-1", "Client OÜ", "EE"))
	recordBytes := append(append([]byte(nil), client...), '\n')
	sha := func(value []byte) string {
		sum := sha256.Sum256(value)
		return hex.EncodeToString(sum[:])
	}
	manifest := importdelivery.Manifest{
		SchemaVersion: importdelivery.SchemaVersionV1, PackageID: "master-detail-client-1", ManifestSHA256: sha([]byte("manifest")), PackageSHA256: sha([]byte("package")), RecordsSHA256: sha(recordBytes), Provider: Provider, SourceCompanyID: "source",
		SourceIdentity: importdelivery.SourceIdentity{ID: "source", ValidationSnapshotSHA256: sha([]byte("snapshot"))},
		Authority:      importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true},
		Scope:          importdelivery.Scope{Mode: "partial_browser_master_detail", CutoffAt: "2026-08-28T12:00:00Z"},
		RecordCount:    1,
	}
	deliveryStore := &referenceDeliveryStore{}
	delivery := importdelivery.NewService(deliveryStore, referenceDeliveryBinder{tenant: "tenant", source: "source"})
	_, err := delivery.AcceptManifest(context.Background(), "schema", "tenant", manifest)
	require.NoError(t, err)
	_, err = delivery.AcceptRawRecordChunk(context.Background(), "schema", "tenant", manifest.PackageID, 0, 1, sha(recordBytes), recordBytes)
	require.NoError(t, err)
	_, err = delivery.Finalize(context.Background(), "schema", "tenant", manifest.PackageID, importdelivery.FinalizeRequest{ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: manifest.RecordsSHA256, RecordCount: 1})
	require.NoError(t, err)
	require.NoError(t, delivery.VerifyStagedPackage(context.Background(), "schema", "tenant", "source", manifest.PackageID, manifest.PackageSHA256))
	assert.ErrorIs(t, delivery.VerifyStagedPackage(context.Background(), "schema", "other-tenant", "source", manifest.PackageID, manifest.PackageSHA256), importdelivery.ErrStagedPackageMismatch)
	assert.ErrorIs(t, delivery.VerifyStagedPackage(context.Background(), "schema", "tenant", "source", manifest.PackageID, sha([]byte("wrong"))), importdelivery.ErrStagedPackageMismatch)

	store := &referenceStore{identities: map[string]*Identity{}}
	writer := &referenceWriter{}
	svc := NewService(delivery, store, writer, referenceCatalog{})
	preview, err := svc.Preview(context.Background(), "schema", "tenant", manifest.PackageID, "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.NoError(t, err)
	require.Equal(t, StatusPreviewReady, preview.Status)
	require.Len(t, preview.Actions, 1)
	applied, err := svc.Apply(context.Background(), "schema", "tenant", "owner", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, StatusApplied, applied.Status)
	assert.Equal(t, 1, writer.contacts)
	_, err = svc.Apply(context.Background(), "schema", "tenant", "owner", ConfirmRequest{Confirm: true, PreviewID: preview.ID, PreviewSHA256: preview.PreviewSHA256})
	require.NoError(t, err)
	assert.Equal(t, 1, writer.contacts)

	// The receiver service, unlike a browser-status mock, cannot read this
	// staged package from any other tenant.
	_, err = svc.Preview(context.Background(), "schema", "other-tenant", manifest.PackageID, "owner", PreviewRequest{EntityTypes: []string{EntityCustomer}})
	require.Error(t, err)
}
