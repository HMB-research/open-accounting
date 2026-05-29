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

const (
	defaultReviewQueueLimit = 50
	maxReviewQueueLimit     = 200
)

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
				HasRejected:        false,
				TotalCount:         0,
				PendingReviewCount: 0,
				ReviewedCount:      0,
				ApprovedCount:      0,
				RejectedCount:      0,
			}
		}
		result = append(result, summary)
	}

	return result, nil
}

func (s *Service) GetReviewQueue(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) (*ReviewQueue, error) {
	entityType, err := normalizeOptionalEntityType(filter.EntityType)
	if err != nil {
		return nil, err
	}
	documentType, err := normalizeOptionalDocumentType(filter.DocumentType)
	if err != nil {
		return nil, err
	}
	reviewStatus, err := normalizeReviewQueueStatus(filter.ReviewStatus)
	if err != nil {
		return nil, err
	}
	limit, err := normalizeReviewQueueLimit(filter.Limit)
	if err != nil {
		return nil, err
	}

	normalizedFilter := ReviewQueueFilter{
		EntityType:   entityType,
		DocumentType: documentType,
		ReviewStatus: reviewStatus,
		Limit:        limit,
	}
	docs, err := s.repo.ListReviewQueueDocuments(ctx, schemaName, tenantID, normalizedFilter)
	if err != nil {
		return nil, err
	}

	queue := &ReviewQueue{
		EntityType:   entityType,
		DocumentType: documentType,
		ReviewStatus: reviewQueueStatusLabel(reviewStatus),
		Limit:        limit,
		TotalCount:   len(docs),
		Documents:    docs,
	}
	for _, doc := range docs {
		switch doc.ReviewStatus {
		case ReviewStatusReviewed:
			queue.ReviewedCount++
		case ReviewStatusApproved:
			queue.ReviewedCount++
			queue.ApprovedCount++
		case ReviewStatusRejected:
			queue.ReviewedCount++
			queue.RejectedCount++
		default:
			queue.PendingReviewCount++
		}
	}

	return queue, nil
}

func (s *Service) EvaluateEvidencePolicy(ctx context.Context, schemaName, tenantID string, req *EvidencePolicyRequest) ([]EvidencePolicyResult, error) {
	if req == nil {
		return nil, fmt.Errorf("evidence policy request is required")
	}

	normalizedType, err := normalizeEntityType(req.EntityType)
	if err != nil {
		return nil, err
	}
	normalizedIDs := normalizeEntityIDs(req.EntityIDs)
	if len(normalizedIDs) == 0 {
		return nil, fmt.Errorf("at least one entity ID is required")
	}
	rules, err := normalizeEvidencePolicyRules(req.Rules)
	if err != nil {
		return nil, err
	}

	results := make([]EvidencePolicyResult, 0, len(normalizedIDs))
	for _, entityID := range normalizedIDs {
		docs, err := s.repo.ListDocuments(ctx, schemaName, tenantID, normalizedType, entityID)
		if err != nil {
			return nil, err
		}
		result := evaluateEvidencePolicyForDocuments(normalizedType, entityID, docs, rules)
		results = append(results, result)
	}

	return results, nil
}

func (s *Service) GetRetentionReview(ctx context.Context, schemaName, tenantID string, asOfDate time.Time, horizonDays int, includeMissing bool) (*RetentionReview, error) {
	if horizonDays < 0 {
		return nil, fmt.Errorf("horizon days must be zero or greater")
	}
	asOf := dateOnlyUTC(asOfDate)
	cutoff := asOf.AddDate(0, 0, horizonDays)

	docs, err := s.repo.ListRetentionReviewDocuments(ctx, schemaName, tenantID, cutoff, includeMissing)
	if err != nil {
		return nil, err
	}

	review := &RetentionReview{
		AsOfDate:   asOf.Format("2006-01-02"),
		CutoffDate: cutoff.Format("2006-01-02"),
		TotalCount: len(docs),
		Documents:  docs,
	}
	for _, doc := range docs {
		if doc.RetentionUntil == nil {
			review.MissingRetentionCount++
		} else if dateOnlyUTC(*doc.RetentionUntil).After(asOf) {
			review.DueSoonCount++
		} else {
			review.ExpiredCount++
		}
		if doc.ReviewStatus == ReviewStatusPending {
			review.PendingReviewCount++
		}
		if doc.ReviewStatus == ReviewStatusRejected {
			review.RejectedCount++
		}
	}

	return review, nil
}

func (s *Service) MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string) (*Document, error) {
	return s.ReviewDocument(ctx, schemaName, tenantID, documentID, reviewedBy, &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusReviewed,
	})
}

func (s *Service) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) (*Document, error) {
	trimmedID := strings.TrimSpace(documentID)
	if trimmedID == "" {
		return nil, fmt.Errorf("document ID is required")
	}

	if _, err := s.repo.GetDocumentByID(ctx, schemaName, tenantID, trimmedID); err != nil {
		return nil, err
	}

	var normalizedRetention *time.Time
	if retentionUntil != nil {
		normalized := dateOnlyUTC(*retentionUntil)
		normalizedRetention = &normalized
	}

	if err := s.repo.UpdateDocumentRetention(ctx, schemaName, tenantID, trimmedID, normalizedRetention); err != nil {
		return nil, err
	}

	return s.repo.GetDocumentByID(ctx, schemaName, tenantID, trimmedID)
}

func dateOnlyUTC(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string, req *ReviewDocumentRequest) (*Document, error) {
	if strings.TrimSpace(reviewedBy) == "" {
		return nil, fmt.Errorf("reviewed by user is required")
	}

	if req == nil {
		return nil, fmt.Errorf("review request is required")
	}
	reviewStatus, err := normalizeReviewStatus(req.ReviewStatus)
	if err != nil {
		return nil, err
	}
	reviewNote := strings.TrimSpace(req.ReviewNote)
	if len(reviewNote) > 2000 {
		return nil, fmt.Errorf("review note must be 2000 characters or less")
	}
	if reviewStatus == ReviewStatusRejected && reviewNote == "" {
		return nil, fmt.Errorf("review note is required when rejecting a document")
	}

	doc, err := s.repo.GetDocumentByID(ctx, schemaName, tenantID, strings.TrimSpace(documentID))
	if err != nil {
		return nil, err
	}
	if doc.ReviewStatus == reviewStatus && strings.TrimSpace(doc.ReviewNote) == reviewNote {
		return doc, nil
	}

	if err := s.repo.ReviewDocument(ctx, schemaName, tenantID, strings.TrimSpace(documentID), reviewStatus, reviewNote, strings.TrimSpace(reviewedBy), time.Now().UTC()); err != nil {
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
	case EntityTypeYearEndClose:
		return EntityTypeYearEndClose, nil
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}

func normalizeOptionalEntityType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeEntityType(value)
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
	case DocumentTypeClosePack:
		return DocumentTypeClosePack, nil
	case DocumentTypeOther:
		return DocumentTypeOther, nil
	default:
		return "", fmt.Errorf("unsupported document type")
	}
}

func normalizeOptionalDocumentType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeDocumentType(value)
}

func normalizeReviewStatus(value string) (string, error) {
	switch strings.TrimSpace(strings.ToUpper(value)) {
	case ReviewStatusReviewed:
		return ReviewStatusReviewed, nil
	case ReviewStatusApproved:
		return ReviewStatusApproved, nil
	case ReviewStatusRejected:
		return ReviewStatusRejected, nil
	default:
		return "", fmt.Errorf("review_status must be REVIEWED, APPROVED, or REJECTED")
	}
}

func normalizeReviewQueueStatus(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ReviewStatusPending, nil
	}
	if strings.EqualFold(trimmed, "all") {
		return "", nil
	}

	switch strings.ToUpper(trimmed) {
	case ReviewStatusPending:
		return ReviewStatusPending, nil
	case ReviewStatusReviewed:
		return ReviewStatusReviewed, nil
	case ReviewStatusApproved:
		return ReviewStatusApproved, nil
	case ReviewStatusRejected:
		return ReviewStatusRejected, nil
	default:
		return "", fmt.Errorf("review_status must be PENDING, REVIEWED, APPROVED, REJECTED, or ALL")
	}
}

func normalizeReviewQueueLimit(value int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("limit must be zero or greater")
	}
	if value == 0 {
		return defaultReviewQueueLimit, nil
	}
	if value > maxReviewQueueLimit {
		return maxReviewQueueLimit, nil
	}
	return value, nil
}

func reviewQueueStatusLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "ALL"
	}
	return value
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

func normalizeEntityIDs(entityIDs []string) []string {
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
	return normalizedIDs
}

func normalizeEvidencePolicyRules(rules []EvidencePolicyRule) ([]EvidencePolicyRule, error) {
	if len(rules) == 0 {
		return nil, fmt.Errorf("at least one evidence policy rule is required")
	}

	normalizedRules := make([]EvidencePolicyRule, 0, len(rules))
	for idx, rule := range rules {
		minCount := rule.MinCount
		if minCount == 0 {
			minCount = 1
		}
		if minCount < 0 {
			return nil, fmt.Errorf("rule %d min_count must be one or greater", idx+1)
		}

		documentTypes := make([]string, 0, len(rule.DocumentTypes))
		seenTypes := make(map[string]struct{}, len(rule.DocumentTypes))
		for _, rawType := range rule.DocumentTypes {
			if strings.TrimSpace(rawType) == "" {
				continue
			}
			documentType, err := normalizeDocumentType(rawType)
			if err != nil {
				return nil, fmt.Errorf("rule %d: %w", idx+1, err)
			}
			if _, exists := seenTypes[documentType]; exists {
				continue
			}
			seenTypes[documentType] = struct{}{}
			documentTypes = append(documentTypes, documentType)
		}

		normalizedRules = append(normalizedRules, EvidencePolicyRule{
			DocumentTypes:   documentTypes,
			MinCount:        minCount,
			RequireApproved: rule.RequireApproved,
		})
	}

	return normalizedRules, nil
}

func evaluateEvidencePolicyForDocuments(entityType, entityID string, docs []Document, rules []EvidencePolicyRule) EvidencePolicyResult {
	result := EvidencePolicyResult{
		EntityType:                 entityType,
		EntityID:                   entityID,
		Compliant:                  true,
		MissingEvidence:            len(docs) == 0,
		DocumentTypeCounts:         make(map[string]int),
		ApprovedDocumentTypeCounts: make(map[string]int),
		RuleResults:                make([]EvidencePolicyRuleResult, 0, len(rules)),
		Violations:                 make([]EvidencePolicyRuleResult, 0),
	}

	for _, doc := range docs {
		result.TotalCount++
		result.DocumentTypeCounts[doc.DocumentType]++
		switch doc.ReviewStatus {
		case ReviewStatusApproved:
			result.ReviewedCount++
			result.ApprovedCount++
			result.ApprovedDocumentTypeCounts[doc.DocumentType]++
		case ReviewStatusRejected:
			result.ReviewedCount++
			result.RejectedCount++
		case ReviewStatusReviewed:
			result.ReviewedCount++
		default:
			result.PendingReviewCount++
		}
	}

	for idx, rule := range rules {
		ruleResult := evaluateEvidencePolicyRule(idx+1, docs, rule)
		result.RuleResults = append(result.RuleResults, ruleResult)
		if !ruleResult.Compliant {
			result.Compliant = false
			result.Violations = append(result.Violations, ruleResult)
		}
	}

	return result
}

func evaluateEvidencePolicyRule(ruleIndex int, docs []Document, rule EvidencePolicyRule) EvidencePolicyRuleResult {
	matchingCount := 0
	approvedMatchingCount := 0
	for _, doc := range docs {
		if !evidencePolicyRuleMatchesDocumentType(rule, doc.DocumentType) {
			continue
		}
		matchingCount++
		if doc.ReviewStatus == ReviewStatusApproved {
			approvedMatchingCount++
		}
	}

	acceptedCount := matchingCount
	if rule.RequireApproved {
		acceptedCount = approvedMatchingCount
	}
	result := EvidencePolicyRuleResult{
		RuleIndex:             ruleIndex,
		DocumentTypes:         append([]string(nil), rule.DocumentTypes...),
		RequiredCount:         rule.MinCount,
		MatchingCount:         matchingCount,
		ApprovedMatchingCount: approvedMatchingCount,
		AcceptedCount:         acceptedCount,
		RequireApproved:       rule.RequireApproved,
		Compliant:             acceptedCount >= rule.MinCount,
	}
	if !result.Compliant {
		result.Message = buildEvidencePolicyViolationMessage(rule, acceptedCount)
	}
	return result
}

func evidencePolicyRuleMatchesDocumentType(rule EvidencePolicyRule, documentType string) bool {
	if len(rule.DocumentTypes) == 0 {
		return true
	}
	for _, allowedType := range rule.DocumentTypes {
		if allowedType == documentType {
			return true
		}
	}
	return false
}

func buildEvidencePolicyViolationMessage(rule EvidencePolicyRule, acceptedCount int) string {
	scope := "any document type"
	if len(rule.DocumentTypes) > 0 {
		scope = strings.Join(rule.DocumentTypes, ", ")
	}
	qualifier := "documents"
	if rule.RequireApproved {
		qualifier = "approved documents"
	}
	return fmt.Sprintf("requires at least %d %s for %s; found %d", rule.MinCount, qualifier, scope, acceptedCount)
}
