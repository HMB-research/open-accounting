package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/plugin"
)

const (
	defaultHTTPTimeout = 10 * time.Second
	maxResponseBytes   = 4096
)

// Service manages outbound webhook endpoints and delivery.
type Service struct {
	repo       Repository
	httpClient *http.Client
	now        func() time.Time
}

// NewService creates a webhook service backed by the ORM repository.
func NewService(pool *pgxpool.Pool) *Service {
	if pool == nil {
		return NewServiceWithRepository(nil, &http.Client{Timeout: defaultHTTPTimeout})
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create webhook GORM repository: %w", err))
	}
	return NewServiceWithRepository(NewGORMRepository(gormDB), &http.Client{Timeout: defaultHTTPTimeout})
}

// NewServiceWithRepository creates a webhook service with injected dependencies.
func NewServiceWithRepository(repo Repository, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Service{
		repo:       repo,
		httpClient: httpClient,
		now:        time.Now,
	}
}

// ListEventTypes returns the event names endpoints can subscribe to.
func ListEventTypes() []string {
	events := append([]string(nil), plugin.AllEventTypes...)
	sort.Strings(events)
	return events
}

// ListEndpoints returns tenant webhook endpoints.
func (s *Service) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]Endpoint, error) {
	endpoints, err := s.repo.ListEndpoints(ctx, tenantID, activeOnly)
	if err != nil {
		return nil, err
	}
	for i := range endpoints {
		sanitizeEndpoint(&endpoints[i])
	}
	return endpoints, nil
}

// GetEndpoint returns one tenant webhook endpoint.
func (s *Service) GetEndpoint(ctx context.Context, tenantID, endpointID string) (*Endpoint, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, tenantID, endpointID)
	if err != nil {
		return nil, err
	}
	sanitizeEndpoint(endpoint)
	return endpoint, nil
}

// CreateEndpoint creates a tenant webhook endpoint.
func (s *Service) CreateEndpoint(ctx context.Context, tenantID string, req *CreateEndpointRequest) (*Endpoint, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	events, err := normalizeEvents(req.Events)
	if err != nil {
		return nil, err
	}
	if err := validateWebhookURL(req.URL); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	now := s.now()
	endpoint := &Endpoint{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Name:      name,
		URL:       strings.TrimSpace(req.URL),
		Events:    events,
		Secret:    strings.TrimSpace(req.Secret),
		SecretSet: strings.TrimSpace(req.Secret) != "",
		IsActive:  isActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	sanitizeEndpoint(endpoint)
	return endpoint, nil
}

// UpdateEndpoint updates a tenant webhook endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, tenantID, endpointID string, req *UpdateEndpointRequest) (*Endpoint, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	endpoint, err := s.repo.GetEndpoint(ctx, tenantID, endpointID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		endpoint.Name = name
	}
	if req.URL != nil {
		if err := validateWebhookURL(*req.URL); err != nil {
			return nil, err
		}
		endpoint.URL = strings.TrimSpace(*req.URL)
	}
	if req.Events != nil {
		events, err := normalizeEvents(req.Events)
		if err != nil {
			return nil, err
		}
		endpoint.Events = events
	}
	if req.Secret != nil {
		endpoint.Secret = strings.TrimSpace(*req.Secret)
		endpoint.SecretSet = endpoint.Secret != ""
	}
	if req.IsActive != nil {
		endpoint.IsActive = *req.IsActive
	}
	endpoint.UpdatedAt = s.now()
	if err := s.repo.UpdateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	sanitizeEndpoint(endpoint)
	return endpoint, nil
}

// DeleteEndpoint deletes a tenant webhook endpoint.
func (s *Service) DeleteEndpoint(ctx context.Context, tenantID, endpointID string) error {
	rows, err := s.repo.DeleteEndpoint(ctx, tenantID, endpointID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("webhook endpoint not found")
	}
	return nil
}

// ListDeliveries returns recent webhook delivery attempts.
func (s *Service) ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		return nil, fmt.Errorf("limit must be between 1 and 200")
	}
	if _, err := s.repo.GetEndpoint(ctx, tenantID, endpointID); err != nil {
		return nil, err
	}
	return s.repo.ListDeliveries(ctx, tenantID, endpointID, limit)
}

// Dispatch sends an event to every active endpoint subscribed to the event.
func (s *Service) Dispatch(ctx context.Context, event Event) (*DeliveryResult, error) {
	normalized, err := s.normalizeEvent(event)
	if err != nil {
		return nil, err
	}
	endpoints, err := s.repo.ListEndpoints(ctx, normalized.TenantID, true)
	if err != nil {
		return nil, err
	}
	result := &DeliveryResult{Event: normalized}
	for _, endpoint := range endpoints {
		if !subscribesTo(endpoint.Events, normalized.Type) {
			continue
		}
		delivery, err := s.deliver(ctx, &endpoint, normalized)
		if err != nil {
			return nil, err
		}
		result.Deliveries = append(result.Deliveries, *delivery)
	}
	return result, nil
}

// DispatchAsync sends an event without blocking the caller.
func (s *Service) DispatchAsync(event Event) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout+5*time.Second)
		defer cancel()
		_, _ = s.Dispatch(ctx, event)
	}()
}

// DispatchTest sends a deterministic event to one endpoint, regardless of subscription.
func (s *Service) DispatchTest(ctx context.Context, tenantID, endpointID string, req *TestDeliveryRequest) (*DeliveryResult, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, tenantID, endpointID)
	if err != nil {
		return nil, err
	}
	eventType := plugin.EventWebhookTest
	payload := json.RawMessage(`{"source":"manual_test"}`)
	if req != nil {
		if strings.TrimSpace(req.EventType) != "" {
			eventType = strings.TrimSpace(req.EventType)
		}
		if len(req.Payload) > 0 {
			payload = req.Payload
		}
	}
	event, err := s.normalizeEvent(Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     payload,
	})
	if err != nil {
		return nil, err
	}
	delivery, err := s.deliver(ctx, endpoint, event)
	if err != nil {
		return nil, err
	}
	return &DeliveryResult{Event: event, Deliveries: []Delivery{*delivery}}, nil
}

// RegisterPluginHooks routes supported plugin events into outbound webhook delivery.
func (s *Service) RegisterPluginHooks(registry *plugin.HookRegistry) {
	if registry == nil {
		return
	}
	for _, eventType := range plugin.AllEventTypes {
		eventType := eventType
		registry.Register(eventType, func(ctx context.Context, event plugin.Event) error {
			_, err := s.Dispatch(ctx, Event{
				Type:      event.Type,
				TenantID:  event.TenantID.String(),
				Data:      event.Data,
				CreatedAt: event.Time,
			})
			return err
		})
	}
}

func (s *Service) normalizeEvent(event Event) (Event, error) {
	event.Type = strings.TrimSpace(event.Type)
	event.TenantID = strings.TrimSpace(event.TenantID)
	if event.Type == "" {
		return Event{}, fmt.Errorf("event_type is required")
	}
	if !plugin.IsValidEventType(event.Type) {
		return Event{}, fmt.Errorf("unsupported event_type %q", event.Type)
	}
	if event.TenantID == "" {
		return Event{}, fmt.Errorf("tenant_id is required")
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = s.now()
	}
	if len(event.Data) == 0 {
		event.Data = json.RawMessage(`{}`)
	}
	if !json.Valid(event.Data) {
		return Event{}, fmt.Errorf("payload must be valid JSON")
	}
	return event, nil
}

func (s *Service) deliver(ctx context.Context, endpoint *Endpoint, event Event) (*Delivery, error) {
	body, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("encode webhook event: %w", err)
	}

	now := s.now()
	delivery := &Delivery{
		ID:            uuid.New().String(),
		TenantID:      event.TenantID,
		EndpointID:    endpoint.ID,
		EventID:       event.ID,
		EventType:     event.Type,
		Status:        DeliveryStatusFailed,
		AttemptNumber: 1,
		RequestBody:   body,
		DeliveredAt:   now,
		CreatedAt:     now,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		delivery.Error = err.Error()
		return s.recordDelivery(ctx, endpoint, delivery)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "open-accounting-webhooks/1.0")
	req.Header.Set("X-Open-Accounting-Event", event.Type)
	req.Header.Set("X-Open-Accounting-Event-ID", event.ID)
	req.Header.Set("X-Open-Accounting-Tenant-ID", event.TenantID)
	if strings.TrimSpace(endpoint.Secret) != "" {
		req.Header.Set("X-Open-Accounting-Signature", signPayload(endpoint.Secret, body))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		delivery.Error = err.Error()
		return s.recordDelivery(ctx, endpoint, delivery)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	delivery.StatusCode = resp.StatusCode
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		delivery.Error = readErr.Error()
	} else {
		delivery.ResponseBody = string(respBody)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && delivery.Error == "" {
		delivery.Status = DeliveryStatusSucceeded
	} else if delivery.Error == "" {
		delivery.Error = fmt.Sprintf("webhook endpoint returned HTTP %d", resp.StatusCode)
	}

	return s.recordDelivery(ctx, endpoint, delivery)
}

func (s *Service) recordDelivery(ctx context.Context, endpoint *Endpoint, delivery *Delivery) (*Delivery, error) {
	if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEndpointLastDelivery(ctx, delivery.TenantID, endpoint.ID, delivery.DeliveredAt); err != nil {
		return nil, err
	}
	return delivery, nil
}

func normalizeEvents(events []string) ([]string, error) {
	seen := make(map[string]struct{}, len(events))
	normalized := make([]string, 0, len(events))
	for _, event := range events {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}
		if !plugin.IsValidEventType(event) {
			return nil, fmt.Errorf("unsupported event_type %q", event)
		}
		if _, ok := seen[event]; ok {
			continue
		}
		seen[event] = struct{}{}
		normalized = append(normalized, event)
	}
	sort.Strings(normalized)
	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one event is required")
	}
	return normalized, nil
}

func validateWebhookURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("url must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("url host is required")
	}
	return nil
}

func subscribesTo(events []string, eventType string) bool {
	for _, event := range events {
		if event == eventType {
			return true
		}
	}
	return false
}

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func sanitizeEndpoint(endpoint *Endpoint) {
	endpoint.SecretSet = strings.TrimSpace(endpoint.Secret) != ""
	endpoint.Secret = ""
}
