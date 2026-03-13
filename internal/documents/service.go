package documents

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var fileNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Service struct {
	repo  Repository
	store Store
}

func NewService(repo Repository, store Store) *Service {
	return &Service{
		repo:  repo,
		store: store,
	}
}

func (s *Service) UploadDocument(ctx context.Context, schemaName, tenantID string, req *UploadDocumentRequest, content io.Reader) (*Document, error) {
	entityType, err := normalizeEntityType(req.EntityType)
	if err != nil {
		return nil, err
	}
	documentType, err := normalizeDocumentType(req.DocumentType)
	if err != nil {
		return nil, err
	}

	entityID := strings.TrimSpace(req.EntityID)
	if entityID == "" {
		return nil, fmt.Errorf("entity ID is required")
	}
	if strings.TrimSpace(req.UploadedBy) == "" {
		return nil, fmt.Errorf("uploaded by user is required")
	}

	fileName := sanitizeFileName(req.FileName)
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}
	if req.FileSize <= 0 {
		return nil, fmt.Errorf("document file is empty")
	}
	if req.FileSize > MaxDocumentSizeBytes {
		return nil, fmt.Errorf("document exceeds the %d MB limit", MaxDocumentSizeBytes>>20)
	}

	exists, err := s.repo.EntityExists(ctx, schemaName, tenantID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("target record not found")
	}

	doc := &Document{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		EntityType:     entityType,
		EntityID:       entityID,
		DocumentType:   documentType,
		FileName:       fileName,
		ContentType:    normalizeContentType(req.ContentType, fileName),
		FileSize:       req.FileSize,
		Notes:          strings.TrimSpace(req.Notes),
		RetentionUntil: req.RetentionUntil,
		ReviewStatus:   ReviewStatusPending,
		UploadedBy:     strings.TrimSpace(req.UploadedBy),
		CreatedAt:      time.Now().UTC(),
	}
	doc.StorageKey = buildStorageKey(tenantID, doc.CreatedAt, doc.ID, fileName)

	if err := s.store.Save(ctx, doc.StorageKey, content); err != nil {
		return nil, err
	}

	if err := s.repo.CreateDocument(ctx, schemaName, doc); err != nil {
		_ = s.store.Delete(ctx, doc.StorageKey)
		return nil, err
	}

	return doc, nil
}

func (s *Service) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error) {
	normalizedType, err := normalizeEntityType(entityType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(entityID) == "" {
		return nil, fmt.Errorf("entity ID is required")
	}

	return s.repo.ListDocuments(ctx, schemaName, tenantID, normalizedType, strings.TrimSpace(entityID))
}

func (s *Service) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) ([]ReviewSummary, error) {
	normalizedType, err := normalizeEntityType(entityType)
	if err != nil {
		return nil, err
	}

	normalizedIDs := make([]string, 0, len(entityIDs))
	seen := make(map[string]struct{}, len(entityIDs))
	for _, entityID := range entityIDs {
		trimmed := strings.TrimSpace(entityID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedIDs = append(normalizedIDs, trimmed)
	}

	if len(normalizedIDs) == 0 {
		return []ReviewSummary{}, nil
	}

	summaryMap, err := s.repo.ListReviewSummaries(ctx, schemaName, tenantID, normalizedType, normalizedIDs)
	if err != nil {
		return nil, err
	}

	result := make([]ReviewSummary, 0, len(normalizedIDs))
	for _, entityID := range normalizedIDs {
		summary, ok := summaryMap[entityID]
		if !ok {
			summary = ReviewSummary{
				EntityType:         normalizedType,
				EntityID:           entityID,
				MissingEvidence:    true,
				HasPendingReview:   false,
				TotalCount:         0,
				PendingReviewCount: 0,
				ReviewedCount:      0,
			}
		}
		result = append(result, summary)
	}

	return result, nil
}

func (s *Service) MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string) (*Document, error) {
	if strings.TrimSpace(reviewedBy) == "" {
		return nil, fmt.Errorf("reviewed by user is required")
	}

	doc, err := s.repo.GetDocumentByID(ctx, schemaName, tenantID, strings.TrimSpace(documentID))
	if err != nil {
		return nil, err
	}
	if doc.ReviewStatus == ReviewStatusReviewed {
		return doc, nil
	}

	if err := s.repo.MarkDocumentReviewed(ctx, schemaName, tenantID, strings.TrimSpace(documentID), strings.TrimSpace(reviewedBy), time.Now().UTC()); err != nil {
		return nil, err
	}

	return s.repo.GetDocumentByID(ctx, schemaName, tenantID, strings.TrimSpace(documentID))
}

func (s *Service) OpenDocument(ctx context.Context, schemaName, tenantID, documentID string) (*Document, io.ReadCloser, error) {
	doc, err := s.repo.GetDocumentByID(ctx, schemaName, tenantID, strings.TrimSpace(documentID))
	if err != nil {
		return nil, nil, err
	}

	reader, err := s.store.Open(ctx, doc.StorageKey)
	if err != nil {
		return nil, nil, err
	}

	return doc, reader, nil
}

func (s *Service) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	doc, err := s.repo.GetDocumentByID(ctx, schemaName, tenantID, strings.TrimSpace(documentID))
	if err != nil {
		return err
	}

	if err := s.store.Delete(ctx, doc.StorageKey); err != nil {
		return err
	}

	return s.repo.DeleteDocument(ctx, schemaName, tenantID, documentID)
}

func normalizeEntityType(value string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case EntityTypeInvoice:
		return EntityTypeInvoice, nil
	case EntityTypeJournalEntry:
		return EntityTypeJournalEntry, nil
	case EntityTypePayment:
		return EntityTypePayment, nil
	case EntityTypeBankTxn:
		return EntityTypeBankTxn, nil
	case EntityTypeAsset:
		return EntityTypeAsset, nil
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}

func normalizeDocumentType(value string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", DocumentTypeSupportingDocument:
		return DocumentTypeSupportingDocument, nil
	case DocumentTypeReceipt:
		return DocumentTypeReceipt, nil
	case DocumentTypeReconciliation:
		return DocumentTypeReconciliation, nil
	case DocumentTypeContract:
		return DocumentTypeContract, nil
	case DocumentTypeAssetRecord:
		return DocumentTypeAssetRecord, nil
	case DocumentTypeTaxSupport:
		return DocumentTypeTaxSupport, nil
	case DocumentTypeOther:
		return DocumentTypeOther, nil
	default:
		return "", fmt.Errorf("unsupported document type")
	}
}

func normalizeContentType(contentType, fileName string) string {
	normalized := strings.TrimSpace(contentType)
	if normalized != "" {
		return normalized
	}

	if guessed := mime.TypeByExtension(filepath.Ext(fileName)); guessed != "" {
		return guessed
	}

	return "application/octet-stream"
}

func sanitizeFileName(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}

	base = fileNameSanitizer.ReplaceAllString(base, "_")
	base = strings.Trim(base, "._-")
	if base == "" {
		return ""
	}
	return base
}

func buildStorageKey(tenantID string, createdAt time.Time, documentID, fileName string) string {
	ext := filepath.Ext(fileName)
	name := strings.TrimSuffix(fileName, ext)
	if name == "" {
		name = "document"
	}

	return filepath.Join(
		tenantID,
		createdAt.Format("2006"),
		createdAt.Format("01"),
		fmt.Sprintf("%s_%s%s", documentID, name, ext),
	)
}
