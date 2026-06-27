package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"

	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func setupEmailHandlers() (*Handlers, *emailHandlerRepository, *emailHandlerMailer) {
	repo := &emailHandlerRepository{
		settings:  make(map[string][]byte),
		templates: make(map[string]email.EmailTemplate),
		logs:      []email.EmailLog{},
	}
	repo.settings["tenant-1"] = []byte(`{
		"smtp_host":"smtp.example.com",
		"smtp_port":587,
		"smtp_username":"user@example.com",
		"smtp_password":"secret",
		"smtp_from_email":"billing@example.com",
		"smtp_from_name":"Billing",
		"smtp_use_tls":true
	}`)
	repo.templates[emailTemplateKey("tenant-1", email.TemplateInvoiceSend)] = email.EmailTemplate{
		ID:           "template-1",
		TenantID:     "tenant-1",
		TemplateType: email.TemplateInvoiceSend,
		Subject:      "Invoice ready",
		BodyHTML:     "<p>Invoice</p>",
		BodyText:     "Invoice",
		IsActive:     true,
	}
	repo.logs = append(repo.logs, email.EmailLog{
		ID:             "log-1",
		TenantID:       "tenant-1",
		EmailType:      string(email.TemplateInvoiceSend),
		RecipientEmail: "customer@example.com",
		Subject:        "Invoice ready",
		Status:         email.StatusSent,
		CreatedAt:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	})

	mailer := &emailHandlerMailer{}
	tenantRepo := newMockTenantRepository()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

	h := &Handlers{
		tenantService: tenant.NewServiceWithRepository(tenantRepo),
		emailService:  email.NewServiceWithRepository(repo, mailer),
	}
	return h, repo, mailer
}

func configureEmailHandlerService(h *Handlers, tenantID string) (*emailHandlerRepository, *emailHandlerMailer) {
	repo := &emailHandlerRepository{
		settings:  make(map[string][]byte),
		templates: make(map[string]email.EmailTemplate),
		logs:      []email.EmailLog{},
	}
	repo.settings[tenantID] = []byte(`{
		"smtp_host":"smtp.example.com",
		"smtp_port":587,
		"smtp_username":"user@example.com",
		"smtp_password":"secret",
		"smtp_from_email":"billing@example.com",
		"smtp_from_name":"Billing",
		"smtp_use_tls":true
	}`)
	mailer := &emailHandlerMailer{}
	h.emailService = email.NewServiceWithRepository(repo, mailer)
	return repo, mailer
}

func TestEmailHandlersSettingsTemplatesAndLogs(t *testing.T) {
	h, repo, mailer := setupEmailHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/settings/smtp", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()
	h.GetSMTPConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var config email.SMTPConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&config))
	assert.Equal(t, "smtp.example.com", config.Host)
	assert.Empty(t, config.Password)

	updateReq := email.UpdateSMTPConfigRequest{
		Host:      "smtp.updated.example.com",
		Port:      465,
		Username:  "updated@example.com",
		Password:  "new-secret",
		FromEmail: "billing@example.com",
		FromName:  "Updated Billing",
		UseTLS:    true,
	}
	req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/settings/smtp", updateReq, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.UpdateSMTPConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	updatedConfig, err := email.ParseSMTPConfig(repo.settings["tenant-1"])
	require.NoError(t, err)
	assert.Equal(t, "smtp.updated.example.com", updatedConfig.Host)
	assert.Equal(t, "new-secret", updatedConfig.Password)

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/settings/smtp/test", email.TestSMTPRequest{
		RecipientEmail: "recipient@example.com",
	}, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.TestSMTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var smtpResult email.TestSMTPResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&smtpResult))
	assert.True(t, smtpResult.Success)
	assert.Equal(t, 1, mailer.sentCount)

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/email-templates", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.ListEmailTemplates(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var templates []email.EmailTemplate
	require.NoError(t, json.NewDecoder(w.Body).Decode(&templates))
	assert.GreaterOrEqual(t, len(templates), 5)

	templateReq := email.UpdateTemplateRequest{
		Subject:  "Receipt ready",
		BodyHTML: "<p>Receipt</p>",
		BodyText: "Receipt",
		IsActive: true,
	}
	req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/email-templates/PAYMENT_RECEIPT", templateReq, claims), map[string]string{
		"tenantID":     "tenant-1",
		"templateType": string(email.TemplatePaymentReceipt),
	})
	w = httptest.NewRecorder()
	h.UpdateEmailTemplate(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var updatedTemplate email.EmailTemplate
	require.NoError(t, json.NewDecoder(w.Body).Decode(&updatedTemplate))
	assert.Equal(t, email.TemplatePaymentReceipt, updatedTemplate.TemplateType)
	assert.Equal(t, "Receipt ready", updatedTemplate.Subject)

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/email-log?limit=1", nil, claims), map[string]string{"tenantID": "tenant-1"})
	w = httptest.NewRecorder()
	h.GetEmailLog(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var logs []email.EmailLog
	require.NoError(t, json.NewDecoder(w.Body).Decode(&logs))
	require.Len(t, logs, 1)
	assert.Equal(t, "customer@example.com", logs[0].RecipientEmail)
}

type emailHandlerRepository struct {
	settings       map[string][]byte
	templates      map[string]email.EmailTemplate
	logs           []email.EmailLog
	getTemplateErr error
}

func (r *emailHandlerRepository) GetTenantSettings(_ context.Context, tenantID string) ([]byte, error) {
	if settings, ok := r.settings[tenantID]; ok {
		return append([]byte(nil), settings...), nil
	}
	return []byte(`{}`), nil
}

func (r *emailHandlerRepository) UpdateTenantSettings(_ context.Context, tenantID string, settingsJSON []byte) error {
	r.settings[tenantID] = append([]byte(nil), settingsJSON...)
	return nil
}

func (r *emailHandlerRepository) GetTemplate(_ context.Context, _, tenantID string, templateType email.TemplateType) (*email.EmailTemplate, error) {
	if r.getTemplateErr != nil {
		return nil, r.getTemplateErr
	}
	template, ok := r.templates[emailTemplateKey(tenantID, templateType)]
	if !ok {
		return nil, email.ErrTemplateNotFound
	}
	return &template, nil
}

func (r *emailHandlerRepository) ListTemplates(_ context.Context, _, tenantID string) ([]email.EmailTemplate, error) {
	templates := make([]email.EmailTemplate, 0, len(r.templates))
	for _, template := range r.templates {
		if template.TenantID == tenantID {
			templates = append(templates, template)
		}
	}
	return templates, nil
}

func (r *emailHandlerRepository) UpsertTemplate(_ context.Context, _ string, template *email.EmailTemplate) error {
	r.templates[emailTemplateKey(template.TenantID, template.TemplateType)] = *template
	return nil
}

func (r *emailHandlerRepository) CreateEmailLog(_ context.Context, _ string, log *email.EmailLog) error {
	r.logs = append([]email.EmailLog{*log}, r.logs...)
	return nil
}

func (r *emailHandlerRepository) UpdateEmailLogStatus(_ context.Context, _, logID string, status email.EmailStatus, sentAt *time.Time, errorMessage string) error {
	for i := range r.logs {
		if r.logs[i].ID == logID {
			r.logs[i].Status = status
			r.logs[i].SentAt = sentAt
			r.logs[i].ErrorMessage = errorMessage
			return nil
		}
	}
	return nil
}

func (r *emailHandlerRepository) GetEmailLog(_ context.Context, _, tenantID string, limit int) ([]email.EmailLog, error) {
	logs := make([]email.EmailLog, 0, len(r.logs))
	for _, log := range r.logs {
		if log.TenantID == tenantID {
			logs = append(logs, log)
		}
	}
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

type emailHandlerMailer struct {
	sentCount int
}

func (m *emailHandlerMailer) SendMail(_ *email.SMTPConfig, _ *mail.Msg) error {
	m.sentCount++
	return nil
}

func emailTemplateKey(tenantID string, templateType email.TemplateType) string {
	return tenantID + ":" + string(templateType)
}
