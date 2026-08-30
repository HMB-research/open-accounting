package smartaccountsreferences

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/google/uuid"
)

var (
	ErrPreviewNotFound       = errors.New("SmartAccounts reference preview not found")
	ErrConfirmationRequired  = errors.New("explicit matching reference preview confirmation is required")
	ErrPackageNotReady       = errors.New("SmartAccounts reference package is not staged for review")
	ErrPreviewReviewRequired = errors.New("SmartAccounts reference preview requires review")
	ErrArticleVATMapping     = errors.New("browser article VAT mapping requires owner review")
)

type ArchiveReader interface {
	GetStatus(context.Context, string, string, string) (importdelivery.Status, error)
	GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error)
	IterateRecords(context.Context, string, string, string, func(json.RawMessage) error) error
}

type Store interface {
	SavePreview(context.Context, string, *StoredPreview, string) error
	GetPreview(context.Context, string, string, string) (*StoredPreview, error)
	GetLatestPreviewForPackage(context.Context, string, string, string) (*StoredPreview, error)
	GetIdentity(context.Context, string, string, string, string, string, string) (*Identity, error)
	ReserveIdentity(context.Context, string, string, string, string, string, string, string, string) (*Identity, bool, error)
	MarkIdentityApplied(context.Context, string, string, string, string, string, string) error
	MarkPreviewApplied(context.Context, string, string, string) error
}

// Writer deliberately has no journal, invoice, or payment method.
type Writer interface {
	// Ensure methods may accept an existing target only after it was read by
	// its deterministic ID and exactly matched to the projected request. A
	// generic unique/code collision is never treated as an applied source row.
	EnsureAccount(context.Context, string, string, *accounting.CreateAccountRequest) error
	EnsureContact(context.Context, string, string, *contacts.CreateContactRequest) error
	EnsureProduct(context.Context, string, string, *inventory.CreateProductRequest) error
}

type Catalog interface {
	ListAccounts(context.Context, string, string) ([]accounting.Account, error)
	ListContacts(context.Context, string, string) ([]contacts.Contact, error)
	ListProducts(context.Context, string, string) ([]inventory.Product, error)
}

type Service struct {
	archive ArchiveReader
	store   Store
	writer  Writer
	catalog Catalog
	now     func() time.Time
}

func NewService(archive ArchiveReader, store Store, writer Writer, catalog Catalog) *Service {
	return &Service{archive: archive, store: store, writer: writer, catalog: catalog, now: time.Now}
}

func (s *Service) Preview(ctx context.Context, schema, tenantID, packageID, userID string, req PreviewRequest) (*Preview, error) {
	if s == nil || s.archive == nil || s.store == nil || s.catalog == nil {
		return nil, errors.New("SmartAccounts reference import is not configured")
	}
	preview, actions, err := s.plan(ctx, schema, tenantID, packageID, req)
	if preview != nil {
		if saveErr := s.store.SavePreview(ctx, schema, &StoredPreview{Preview: *preview, Actions: actions}, userID); saveErr != nil {
			return nil, saveErr
		}
	}
	return preview, err
}

func (s *Service) Apply(ctx context.Context, schema, tenantID, _ string, req ConfirmRequest) (*Preview, error) {
	if s == nil || s.store == nil || s.writer == nil {
		return nil, errors.New("SmartAccounts reference import is not configured")
	}
	if !req.Confirm || strings.TrimSpace(req.PreviewID) == "" || strings.TrimSpace(req.PreviewSHA256) == "" {
		return nil, ErrConfirmationRequired
	}
	stored, err := s.store.GetPreview(ctx, schema, tenantID, req.PreviewID)
	if err != nil {
		return nil, err
	}
	p := &stored.Preview
	if p.Status == StatusApplied {
		return p, nil
	}
	if p.Status != StatusPreviewReady || p.PreviewSHA256 != req.PreviewSHA256 {
		return p, ErrConfirmationRequired
	}
	for _, action := range stored.Actions {
		identity, created, err := s.store.ReserveIdentity(ctx, schema, tenantID, Provider, p.SourceCompanyID, action.EntityType, action.ExternalID, action.Revision, action.TargetID)
		if err != nil {
			return p, err
		}
		if !created && (identity == nil || identity.Revision != action.Revision || (identity.Status != IdentityApplied && identity.Status != IdentityPending)) {
			return p, fmt.Errorf("%s %s requires manual source revision review", action.EntityType, action.ExternalID)
		}
		if identity != nil && identity.Status == IdentityApplied && identity.Revision == action.Revision {
			continue
		}
		if err := s.applyAction(ctx, schema, tenantID, action); err != nil {
			return p, err
		}
		if err := s.store.MarkIdentityApplied(ctx, schema, tenantID, Provider, p.SourceCompanyID, action.EntityType, action.ExternalID); err != nil {
			return p, err
		}
	}
	if err := s.store.MarkPreviewApplied(ctx, schema, tenantID, p.ID); err != nil {
		return p, err
	}
	now := s.now().UTC()
	p.Status, p.AppliedAt = StatusApplied, &now
	return p, nil
}

// PackageEvidence is a safe reconciliation seam. It counts only fixed
// review categories and exposes no projected action or source payload.
type PackageEvidence struct {
	Applicable          bool
	PreviewID           string
	PreviewSHA256       string
	Applied             bool
	RevisionUnresolved  int
	TombstoneUnresolved int
}

func (s *Service) GetPackageEvidence(ctx context.Context, schema, tenantID, packageID string) (PackageEvidence, error) {
	if s == nil || s.archive == nil || s.store == nil {
		return PackageEvidence{}, errors.New("SmartAccounts reference import is not configured")
	}
	applicable := false
	err := s.archive.IterateRecords(ctx, schema, tenantID, packageID, func(raw json.RawMessage) error {
		var record struct {
			EntityType string `json:"entity_type"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		switch record.EntityType {
		case EntityAccount, EntityCustomer, EntityVendor, EntityItem:
			applicable = true
		}
		return nil
	})
	if err != nil {
		return PackageEvidence{}, err
	}
	if !applicable {
		return PackageEvidence{Applicable: false}, nil
	}
	stored, err := s.store.GetLatestPreviewForPackage(ctx, schema, tenantID, packageID)
	if err != nil {
		if errors.Is(err, ErrPreviewNotFound) {
			return PackageEvidence{Applicable: true}, nil
		}
		return PackageEvidence{}, err
	}
	evidence := PackageEvidence{Applicable: true, PreviewID: stored.Preview.ID, PreviewSHA256: stored.Preview.PreviewSHA256, Applied: stored.Preview.Status == StatusApplied}
	for _, issue := range stored.Preview.Issues {
		switch issue.Code {
		case "source_revision_review_required":
			evidence.RevisionUnresolved++
		case "source_tombstone_review_required":
			evidence.TombstoneUnresolved++
		}
	}
	return evidence, nil
}

func (s *Service) applyAction(ctx context.Context, schema, tenantID string, action storedAction) error {
	switch action.EntityType {
	case EntityAccount:
		return s.writer.EnsureAccount(ctx, schema, tenantID, &accounting.CreateAccountRequest{ID: action.TargetID, Code: action.Code, Name: action.Name, AccountType: accounting.AccountType(action.AccountType)})
	case EntityCustomer, EntityVendor:
		return s.writer.EnsureContact(ctx, tenantID, schema, &contacts.CreateContactRequest{ID: action.TargetID, Code: action.Code, Name: action.Name, ContactType: contacts.ContactType(action.ContactType), RegCode: action.RegCode, VATNumber: action.VATNumber, Email: action.Email, Phone: action.Phone, AddressLine1: action.AddressLine1, AddressLine2: action.AddressLine2, City: action.City, PostalCode: action.PostalCode, CountryCode: action.CountryCode})
	case EntityItem:
		return s.writer.EnsureProduct(ctx, tenantID, schema, &inventory.CreateProductRequest{ID: action.TargetID, Code: action.Code, Name: action.Name, Description: action.Description, ProductType: action.ProductType, Unit: action.Unit, SalesPrice: action.SalesPrice, VATRate: action.VATRate, TrackInventory: false})
	default:
		return errors.New("unsupported SmartAccounts reference action")
	}
}

// StoredPreview holds server-only projected apply data. The actions remain
// unexported so they cannot become a browser response by accident.
type StoredPreview struct {
	Preview Preview
	Actions []storedAction
}

type storedAction struct {
	Action
	Code, Name, AccountType, ContactType string
	RegCode, VATNumber, Email, Phone     string
	AddressLine1, AddressLine2           string
	City, PostalCode, CountryCode        string
	Description, ProductType, Unit       string
	SalesPrice, VATRate                  string
}

type bridgeRecord struct {
	EntityType      string          `json:"entity_type"`
	SourceSchema    string          `json:"source_schema"`
	ExternalID      string          `json:"external_id"`
	ExternalIDMode  string          `json:"external_id_mode"`
	Revision        string          `json:"revision"`
	Operation       string          `json:"operation"`
	Payload         json.RawMessage `json:"payload"`
	PayloadSHA256   string          `json:"payload_sha256"`
	SourceCompanyID string          `json:"source_company_id"`
	Resource        string          `json:"resource"`
	GLPostingMode   string          `json:"gl_posting_mode"`
	Relationship    string          `json:"relationship"`
	ReviewReason    string          `json:"review_reason"`
}

func (s *Service) plan(ctx context.Context, schema, tenantID, packageID string, req PreviewRequest) (*Preview, []storedAction, error) {
	status, err := s.archive.GetStatus(ctx, schema, tenantID, packageID)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := s.archive.GetManifest(ctx, schema, tenantID, packageID)
	if err != nil {
		return nil, nil, err
	}
	p := &Preview{ID: uuid.NewString(), TenantID: tenantID, PackageID: packageID, SourceCompanyID: status.SourceCompanyID, Status: StatusReviewRequired}
	if status.Status != importdelivery.StatusStagedReview || manifest.Provider != Provider || manifest.SourceCompanyID != status.SourceCompanyID {
		p.Issues = append(p.Issues, Issue{Code: "package_not_staged", Message: "reference preview requires a tenant-bound SmartAccounts staged package"})
		p.PreviewSHA256 = digest(*p, nil)
		return p, nil, ErrPackageNotReady
	}
	selected, ok := selectEntities(req.EntityTypes)
	if !ok {
		p.Issues = append(p.Issues, Issue{Code: "invalid_entity_selection", Message: "only account, customer, vendor, and item reference masters may be selected"})
		p.PreviewSHA256 = digest(*p, nil)
		return p, nil, ErrPreviewReviewRequired
	}
	catalog, err := s.loadCatalog(ctx, schema, tenantID)
	if err != nil {
		return nil, nil, err
	}
	recon := map[string]*Reconciliation{}
	for entity := range selected {
		recon[entity] = &Reconciliation{EntityType: entity}
	}
	var actions []storedAction
	// The bridge normalizer promises source_party_id uniqueness per resource
	// snapshot. Recheck it while planning so a malformed staged package cannot
	// turn two source parties into one OA contact code.
	seenBrowserPartyIDs := map[string]struct{}{}
	issue := func(code, entity, external, message string) {
		p.Issues = append(p.Issues, Issue{Code: code, EntityType: entity, ExternalID: external, Message: message})
		if recon[entity] != nil {
			recon[entity].ReviewRequired++
		}
	}
	err = s.archive.IterateRecords(ctx, schema, tenantID, packageID, func(raw json.RawMessage) error {
		var record bridgeRecord
		if json.Unmarshal(raw, &record) != nil {
			issue("invalid_archive_record", "", "", "archived canonical record cannot be decoded")
			return nil
		}
		if !selected[record.EntityType] {
			return nil
		}
		r := recon[record.EntityType]
		r.SourceRecords++
		if strings.TrimSpace(record.SourceCompanyID) != manifest.SourceCompanyID {
			issue("source_binding_mismatch", record.EntityType, record.ExternalID, "record source does not match the package binding")
			return nil
		}
		if record.Operation == "tombstone" {
			r.Tombstones++
			issue("source_tombstone_review_required", record.EntityType, record.ExternalID, "source deletion is retained for manual review and never removes OA data")
			return nil
		}
		if record.Operation != "upsert" || record.GLPostingMode != "non_posting_reference" || strings.TrimSpace(record.ReviewReason) != "" || !validDigest(record.Revision) || record.Revision != record.PayloadSHA256 || !payloadMatches(record.Payload, record.PayloadSHA256) {
			issue("source_record_review_required", record.EntityType, record.ExternalID, "reference record is not an approved non-posting canonical upsert")
			return nil
		}
		if !sourceSchemaMatches(record) {
			issue("source_schema_mismatch", record.EntityType, record.ExternalID, "record does not match the reviewed SmartAccounts reference schema")
			return nil
		}
		if isBrowserMasterDetailParty(record) {
			partyID, partyErr := browserMasterDetailPartyID(record)
			if partyErr != nil {
				issue("reference_mapping_required", record.EntityType, record.ExternalID, partyErr.Error())
				return nil
			}
			if _, duplicate := seenBrowserPartyIDs[partyID]; duplicate {
				issue("source_party_collision_review_required", record.EntityType, record.ExternalID, "duplicate browser source party identifier requires manual review")
				return nil
			}
			seenBrowserPartyIDs[partyID] = struct{}{}
		}
		existing, getErr := s.store.GetIdentity(ctx, schema, tenantID, Provider, manifest.SourceCompanyID, record.EntityType, record.ExternalID)
		if getErr != nil {
			return getErr
		}
		if existing != nil {
			if existing.Revision == record.Revision && existing.Status == IdentityApplied {
				r.AlreadyApplied++
				p.Actions = append(p.Actions, Action{EntityType: record.EntityType, ExternalID: record.ExternalID, TargetID: existing.TargetID, Revision: record.Revision, Action: "ALREADY_APPLIED"})
				return nil
			}
			if existing.Revision == record.Revision && existing.Status == IdentityPending {
				action, actionErr := planAction(record, catalog)
				if actionErr != nil || action.TargetID != existing.TargetID {
					issue("source_revision_review_required", record.EntityType, record.ExternalID, "unfinished source identity no longer has the exact reviewed mapping")
					return nil
				}
				action.Action.Action = "RESUME"
				actions = append(actions, action)
				r.CreatePlanned++
				p.Actions = append(p.Actions, action.Action)
				return nil
			}
			issue("source_revision_review_required", record.EntityType, record.ExternalID, "changed or unfinished source identity requires manual review; it is never overwritten")
			return nil
		}
		action, actionErr := planAction(record, catalog)
		if actionErr != nil {
			if errors.Is(actionErr, ErrArticleVATMapping) {
				issue("article_vat_mapping_review_required", record.EntityType, record.ExternalID, actionErr.Error())
			} else {
				issue("reference_mapping_required", record.EntityType, record.ExternalID, actionErr.Error())
			}
			return nil
		}
		actions = append(actions, action)
		r.CreatePlanned++
		p.Actions = append(p.Actions, action.Action)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for _, entity := range []string{EntityAccount, EntityCustomer, EntityVendor, EntityItem} {
		if r := recon[entity]; r != nil {
			p.Reconciliation = append(p.Reconciliation, *r)
		}
	}
	sort.Slice(p.Actions, func(i, j int) bool {
		if p.Actions[i].EntityType != p.Actions[j].EntityType {
			return p.Actions[i].EntityType < p.Actions[j].EntityType
		}
		return p.Actions[i].ExternalID < p.Actions[j].ExternalID
	})
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].EntityType != actions[j].EntityType {
			return actions[i].EntityType < actions[j].EntityType
		}
		return actions[i].ExternalID < actions[j].ExternalID
	})
	if len(p.Issues) == 0 {
		p.Status = StatusPreviewReady
	}
	p.PreviewSHA256 = digest(*p, actions)
	if p.Status != StatusPreviewReady {
		return p, actions, ErrPreviewReviewRequired
	}
	return p, actions, nil
}

type localCatalog struct{ accountCodes, accountNames, contactCodes, productCodes map[string]bool }

func (s *Service) loadCatalog(ctx context.Context, schema, tenant string) (localCatalog, error) {
	accounts, err := s.catalog.ListAccounts(ctx, schema, tenant)
	if err != nil {
		return localCatalog{}, err
	}
	contactsList, err := s.catalog.ListContacts(ctx, schema, tenant)
	if err != nil {
		return localCatalog{}, err
	}
	products, err := s.catalog.ListProducts(ctx, schema, tenant)
	if err != nil {
		return localCatalog{}, err
	}
	c := localCatalog{map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}}
	for _, v := range accounts {
		c.accountCodes[key(v.Code)], c.accountNames[key(v.Name)] = true, true
	}
	for _, v := range contactsList {
		c.contactCodes[key(v.Code)] = true
	}
	for _, v := range products {
		c.productCodes[key(v.Code)] = true
	}
	return c, nil
}
func key(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }

func selectEntities(values []string) (map[string]bool, bool) {
	result := map[string]bool{}
	if len(values) == 0 {
		for _, e := range []string{EntityAccount, EntityCustomer, EntityVendor, EntityItem} {
			result[e] = true
		}
		return result, true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != EntityAccount && value != EntityCustomer && value != EntityVendor && value != EntityItem || result[value] {
			return nil, false
		}
		result[value] = true
	}
	return result, true
}

func sourceSchemaMatches(r bridgeRecord) bool {
	switch r.EntityType {
	case EntityAccount:
		return r.Resource == "settings.accounts.get" && r.SourceSchema == "smartaccounts-api-v1.7/account_v1"
	case EntityCustomer:
		return (r.Resource == "purchasesales.clients.get" && r.SourceSchema == "smartaccounts-api-v1.7/customer_v1") ||
			browserMasterDetailRecordMatches(r, BrowserClientsDetailSchema, "clients")
	case EntityVendor:
		return (r.Resource == "purchasesales.vendors.get" && r.SourceSchema == "smartaccounts-api-v1.7/vendor_v1") ||
			browserMasterDetailRecordMatches(r, BrowserVendorsDetailSchema, "vendors")
	case EntityItem:
		return (r.Resource == "purchasesales.articles.get" && r.SourceSchema == "smartaccounts-api-v1.7/article_v1") ||
			browserMasterDetailRecordMatches(r, BrowserArticlesDetailSchema, "articles")
	}
	return false
}

func browserMasterDetailRecordMatches(r bridgeRecord, schema, resource string) bool {
	return r.SourceSchema == schema && r.Resource == resource && r.ExternalIDMode == BrowserDetailExternalIDMode && r.Relationship == BrowserDetailRelationship && isNumericID(r.ExternalID)
}

func isBrowserMasterDetailParty(r bridgeRecord) bool {
	return (r.EntityType == EntityCustomer && r.SourceSchema == BrowserClientsDetailSchema) || (r.EntityType == EntityVendor && r.SourceSchema == BrowserVendorsDetailSchema)
}

func planAction(r bridgeRecord, catalog localCatalog) (storedAction, error) {
	a := storedAction{Action: Action{EntityType: r.EntityType, ExternalID: r.ExternalID, TargetID: deterministicTargetID(r.SourceCompanyID, r.EntityType, r.ExternalID), Revision: r.Revision, Action: "CREATE"}}
	switch r.EntityType {
	case EntityAccount:
		var source struct {
			ID            string `json:"id"`
			Code          string `json:"code"`
			Type          string `json:"type"`
			DescriptionET string `json:"descriptionEt"`
			DescriptionEN string `json:"descriptionEn"`
		}
		if err := json.Unmarshal(r.Payload, &source); err != nil {
			return a, errors.New("account payload cannot be decoded")
		}
		id, code, typ := strings.TrimSpace(source.ID), strings.TrimSpace(source.Code), strings.TrimSpace(source.Type)
		name := strings.TrimSpace(source.DescriptionET)
		if name == "" {
			name = strings.TrimSpace(source.DescriptionEN)
		}
		mapped := map[string]string{"ASSET": "ASSET", "LIABILITY": "LIABILITY", "INCOME": "REVENUE", "EXPENSE": "EXPENSE"}[typ]
		if id == "" || id != r.ExternalID || code == "" || name == "" || mapped == "" || catalog.accountCodes[key(code)] || catalog.accountNames[key(name)] {
			return a, errors.New("account ID, code, name, type, and collision-free target chart entry are required")
		}
		a.Code, a.Name, a.AccountType = code, name, mapped
	case EntityCustomer, EntityVendor:
		if isBrowserMasterDetailParty(r) {
			return planBrowserMasterDetailPartyAction(r, catalog)
		}
		var source struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			RegCode   string `json:"regCode"`
			VATNumber string `json:"vatNumber"`
			Address   struct {
				Country    string `json:"country"`
				City       string `json:"city"`
				Address1   string `json:"address1"`
				Address2   string `json:"address2"`
				PostalCode string `json:"postalCode"`
			} `json:"address"`
			Contacts []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"contacts"`
		}
		if err := json.Unmarshal(r.Payload, &source); err != nil {
			return a, errors.New("party payload cannot be decoded")
		}
		country := strings.ToUpper(strings.TrimSpace(source.Address.Country))
		if source.ID != r.ExternalID || strings.TrimSpace(source.Name) == "" || len(r.ExternalID) > 50 || len(country) != 2 || catalog.contactCodes[key(r.ExternalID)] {
			return a, errors.New("party ID, name, ISO-2 country, and collision-free contact code are required")
		}
		a.Code, a.Name, a.RegCode, a.VATNumber, a.CountryCode = r.ExternalID, strings.TrimSpace(source.Name), strings.TrimSpace(source.RegCode), strings.TrimSpace(source.VATNumber), country
		a.AddressLine1, a.AddressLine2, a.City, a.PostalCode = strings.TrimSpace(source.Address.Address1), strings.TrimSpace(source.Address.Address2), strings.TrimSpace(source.Address.City), strings.TrimSpace(source.Address.PostalCode)
		if r.EntityType == EntityCustomer {
			a.ContactType = string(contacts.ContactTypeCustomer)
		} else {
			a.ContactType = string(contacts.ContactTypeSupplier)
		}
		for _, c := range source.Contacts {
			if a.Email == "" && c.Type == "EMAIL" {
				a.Email = strings.TrimSpace(c.Value)
			}
			if a.Phone == "" && c.Type == "PHONE" {
				a.Phone = strings.TrimSpace(c.Value)
			}
		}
	case EntityItem:
		if r.SourceSchema == BrowserArticlesDetailSchema && r.Resource == "articles" {
			// The reviewed browser projection intentionally emits no VAT rate or
			// approved source-token mapping. Never default an OA VAT rate.
			return a, ErrArticleVATMapping
		}
		var source struct {
			Code           string `json:"code"`
			Description    string `json:"description"`
			Type           string `json:"type"`
			Unit           string `json:"unit"`
			VATPc          string `json:"vatPc"`
			PriceSales     string `json:"priceSales"`
			ActiveSales    *bool  `json:"activeSales"`
			ActivePurchase *bool  `json:"activePurchase"`
		}
		if err := json.Unmarshal(r.Payload, &source); err != nil {
			return a, errors.New("item payload cannot be decoded")
		}
		kind := map[string]string{"PRODUCT": "GOODS", "SERVICE": "SERVICE"}[strings.TrimSpace(source.Type)]
		if source.Code != r.ExternalID || strings.TrimSpace(source.Code) == "" || strings.TrimSpace(source.Description) == "" || kind == "" || strings.TrimSpace(source.Unit) == "" || strings.TrimSpace(source.PriceSales) == "" || strings.TrimSpace(source.VATPc) == "" || source.ActiveSales == nil || source.ActivePurchase == nil || !*source.ActiveSales && !*source.ActivePurchase || catalog.productCodes[key(source.Code)] {
			return a, errors.New("item code, description, PRODUCT/SERVICE type, unit, active state, source price, VAT percentage, and collision-free target code are required")
		}
		a.Code, a.Name, a.Description, a.ProductType, a.Unit, a.SalesPrice, a.VATRate = source.Code, source.Description, source.Description, kind, strings.TrimSpace(source.Unit), strings.TrimSpace(source.PriceSales), strings.TrimSpace(source.VATPc)
	default:
		return a, errors.New("unsupported reference entity")
	}
	return a, nil
}

type browserMasterDetailPartyPayload struct {
	SourcePartyID    string `json:"source_party_id"`
	Name             string `json:"name"`
	RegistrationCode string `json:"registration_code"`
	VATNumber        string `json:"vat_number"`
	Address          struct {
		CountryCode string `json:"country_code"`
		County      string `json:"county"`
		City        string `json:"city"`
		Line1       string `json:"line1"`
		Line2       string `json:"line2"`
		PostalCode  string `json:"postal_code"`
	} `json:"address"`
	SourceDetail struct {
		ID             string `json:"id"`
		PathSHA256     string `json:"path_sha256"`
		ContractSHA256 string `json:"contract_sha256"`
	} `json:"source_detail"`
}

func decodeBrowserMasterDetailParty(payload json.RawMessage) (browserMasterDetailPartyPayload, error) {
	var value browserMasterDetailPartyPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, errors.New("browser party payload cannot be decoded as the reviewed schema")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value, errors.New("browser party payload must contain exactly one reviewed object")
	}
	return value, nil
}

func browserMasterDetailPartyID(r bridgeRecord) (string, error) {
	value, err := decodeBrowserMasterDetailParty(r.Payload)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value.SourcePartyID) != value.SourcePartyID || value.SourcePartyID == "" || len(value.SourcePartyID) > 20 || !utf8.ValidString(value.SourcePartyID) {
		return "", errors.New("browser source_party_id is required and must be a stable value of at most 20 UTF-8 bytes")
	}
	return value.SourcePartyID, nil
}

func planBrowserMasterDetailPartyAction(r bridgeRecord, catalog localCatalog) (storedAction, error) {
	a := storedAction{Action: Action{EntityType: r.EntityType, ExternalID: r.ExternalID, TargetID: deterministicTargetID(r.SourceCompanyID, r.EntityType, r.ExternalID), Revision: r.Revision, Action: "CREATE"}}
	value, err := decodeBrowserMasterDetailParty(r.Payload)
	if err != nil {
		return a, err
	}
	partyID, err := browserMasterDetailPartyID(r)
	if err != nil {
		return a, err
	}
	if !isNumericID(r.ExternalID) || value.SourceDetail.ID != r.ExternalID || !validDigest(value.SourceDetail.PathSHA256) || !validDigest(value.SourceDetail.ContractSHA256) || strings.TrimSpace(value.Name) == "" || !validBrowserISO2(value.Address.CountryCode) || catalog.contactCodes[key(partyID)] {
		return a, errors.New("browser party requires a collision-free source_party_id, name, proven ISO-2 country, and exact source-detail provenance")
	}
	a.Code, a.Name, a.RegCode, a.VATNumber = partyID, strings.TrimSpace(value.Name), strings.TrimSpace(value.RegistrationCode), strings.TrimSpace(value.VATNumber)
	a.AddressLine1, a.AddressLine2, a.City, a.PostalCode = strings.TrimSpace(value.Address.Line1), strings.TrimSpace(value.Address.Line2), strings.TrimSpace(value.Address.City), strings.TrimSpace(value.Address.PostalCode)
	// County has no direct OA contact field. It remains archived canonical
	// evidence rather than being guessed into city or address text.
	a.CountryCode = value.Address.CountryCode
	if r.EntityType == EntityCustomer {
		a.ContactType = string(contacts.ContactTypeCustomer)
	} else {
		a.ContactType = string(contacts.ContactTypeSupplier)
	}
	return a, nil
}

func validBrowserISO2(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func isNumericID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func deterministicTargetID(source, entity, external string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(source+"\x00"+entity+"\x00"+external)).String()
}
func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func payloadMatches(payload json.RawMessage, digest string) bool {
	var v any
	if json.Unmarshal(payload, &v) != nil {
		return false
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return false
	}
	h := sha256.Sum256(encoded)
	return hex.EncodeToString(h[:]) == digest
}
func digest(p Preview, actions []storedAction) string {
	p.ID = ""
	p.PreviewSHA256 = ""
	p.AppliedAt = nil
	b, _ := json.Marshal(struct {
		Preview Preview        `json:"preview"`
		Actions []storedAction `json:"actions"`
	}{p, actions})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
