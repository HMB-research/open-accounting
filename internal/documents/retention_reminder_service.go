package documents

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/email"
)

const DefaultRetentionReminderHorizonDays = 30

// RetentionReminderEmailService captures the email operations needed by scheduled retention reminders.
type RetentionReminderEmailService interface {
	GetTemplate(ctx context.Context, schemaName, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error)
	RenderTemplate(tmpl *email.EmailTemplate, data *email.TemplateData) (subject, bodyHTML, bodyText string, err error)
	SendEmail(ctx context.Context, schemaName, tenantID, emailType, recipient, recipientName, subject, bodyHTML, bodyText string, attachments []email.Attachment, relatedID string) (*email.EmailSentResponse, error)
}

// RetentionReminderDeliveryResult summarizes one scheduled retention reminder run for a tenant.
type RetentionReminderDeliveryResult struct {
	TenantID       string `json:"tenant_id"`
	RecipientEmail string `json:"recipient_email,omitempty"`
	AsOfDate       string `json:"as_of_date"`
	CutoffDate     string `json:"cutoff_date"`
	ActionsFound   int    `json:"actions_found"`
	EmailSent      bool   `json:"email_sent"`
	Skipped        bool   `json:"skipped"`
	SkipReason     string `json:"skip_reason,omitempty"`
	Failed         bool   `json:"failed"`
	ErrorMessage   string `json:"error_message,omitempty"`
	EmailLogID     string `json:"email_log_id,omitempty"`
}

// RetentionReminderService sends a digest of document retention follow-up actions.
type RetentionReminderService struct {
	documents *Service
	email     RetentionReminderEmailService
}

func NewRetentionReminderService(documentService *Service, emailService RetentionReminderEmailService) *RetentionReminderService {
	return &RetentionReminderService{
		documents: documentService,
		email:     emailService,
	}
}

func (s *RetentionReminderService) ProcessRetentionRemindersForTenant(ctx context.Context, tenantID, schemaName, companyName, recipientEmail string, asOf time.Time, horizonDays int, includeMissing bool) (RetentionReminderDeliveryResult, error) {
	normalizedAsOf := dateOnlyUTC(asOf)
	result := RetentionReminderDeliveryResult{
		TenantID:       tenantID,
		RecipientEmail: strings.TrimSpace(recipientEmail),
		AsOfDate:       normalizedAsOf.Format("2006-01-02"),
	}

	if s == nil || s.documents == nil || s.email == nil {
		return result, fmt.Errorf("retention reminder service is not configured")
	}
	if result.RecipientEmail == "" {
		result.Skipped = true
		result.SkipReason = "tenant settings email is not configured"
		return result, nil
	}

	review, err := s.documents.GetRetentionReview(ctx, schemaName, tenantID, normalizedAsOf, horizonDays, includeMissing)
	if err != nil {
		return result, err
	}
	result.CutoffDate = review.CutoffDate
	result.ActionsFound = len(review.ReminderActions)
	if result.ActionsFound == 0 {
		result.Skipped = true
		result.SkipReason = "no document retention reminder actions"
		return result, nil
	}

	template, err := s.email.GetTemplate(ctx, schemaName, tenantID, email.TemplateDocumentRetentionReminder)
	if err != nil {
		result.Failed = true
		result.ErrorMessage = fmt.Sprintf("get template: %v", err)
		return result, nil
	}

	templateData := documentRetentionTemplateData(companyName, review)
	subject, bodyHTML, bodyText, err := s.email.RenderTemplate(template, templateData)
	if err != nil {
		result.Failed = true
		result.ErrorMessage = fmt.Sprintf("render template: %v", err)
		return result, nil
	}

	sent, err := s.email.SendEmail(
		ctx,
		schemaName,
		tenantID,
		string(email.TemplateDocumentRetentionReminder),
		result.RecipientEmail,
		strings.TrimSpace(companyName),
		subject,
		bodyHTML,
		bodyText,
		nil,
		"",
	)
	if err != nil {
		result.Failed = true
		result.ErrorMessage = fmt.Sprintf("send email: %v", err)
		return result, nil
	}
	result.EmailSent = true
	if sent != nil {
		result.EmailLogID = sent.LogID
	}
	return result, nil
}

func documentRetentionTemplateData(companyName string, review *RetentionReview) *email.TemplateData {
	if strings.TrimSpace(companyName) == "" {
		companyName = "Open Accounting"
	}

	actions := make([]email.RetentionReminderTemplateAction, 0, len(review.ReminderActions))
	for _, action := range review.ReminderActions {
		actions = append(actions, email.RetentionReminderTemplateAction{
			Action:             action.Action,
			DocumentID:         action.DocumentID,
			DocumentType:       action.DocumentType,
			FileName:           action.FileName,
			EntityType:         action.EntityType,
			EntityID:           action.EntityID,
			Message:            action.Message,
			DaysUntilRetention: formatRetentionReminderDays(action.DaysUntilRetention),
			RetentionUntil:     formatRetentionReminderDate(action.RetentionUntil),
		})
	}

	return &email.TemplateData{
		CompanyName:                    strings.TrimSpace(companyName),
		RetentionAsOfDate:              review.AsOfDate,
		RetentionCutoffDate:            review.CutoffDate,
		RetentionActionCount:           len(review.ReminderActions),
		RetentionTotalCount:            review.TotalCount,
		RetentionExpiredCount:          review.ExpiredCount,
		RetentionDueSoonCount:          review.DueSoonCount,
		RetentionMissingRetentionCount: review.MissingRetentionCount,
		RetentionPendingReviewCount:    review.PendingReviewCount,
		RetentionRejectedCount:         review.RejectedCount,
		RetentionActions:               actions,
	}
}

func formatRetentionReminderDays(days *int) string {
	if days == nil {
		return ""
	}
	return strconv.Itoa(*days)
}

func formatRetentionReminderDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return dateOnlyUTC(*value).Format("2006-01-02")
}
