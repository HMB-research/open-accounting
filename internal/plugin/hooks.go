package plugin

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Event types for the hook system
const (
	// Invoice events
	EventInvoiceCreated = "invoice.created"
	EventInvoiceSent    = "invoice.sent"
	EventInvoicePaid    = "invoice.paid"
	EventInvoiceVoided  = "invoice.voided"

	// Payment events
	EventPaymentReceived  = "payment.received"
	EventPaymentAllocated = "payment.allocated"

	// Contact events
	EventContactCreated = "contact.created"
	EventContactUpdated = "contact.updated"
	EventContactDeleted = "contact.deleted"

	// Journal entry events
	EventJournalEntryCreated = "journal_entry.created"
	EventJournalEntryPosted  = "journal_entry.posted"
	EventJournalEntryVoided  = "journal_entry.voided"

	// Recurring invoice events
	EventRecurringCreated   = "recurring.created"
	EventRecurringGenerated = "recurring.generated"
	EventRecurringStopped   = "recurring.stopped"

	// Banking events
	EventBankTransactionImported = "bank_transaction.imported"
	EventBankTransactionMatched  = "bank_transaction.matched"
	EventReconciliationCompleted = "reconciliation.completed"

	// Payroll events
	EventPayrollCalculated = "payroll.calculated"
	EventPayrollApproved   = "payroll.approved"
	EventEmployeeCreated   = "employee.created"

	// Tenant events
	EventTenantCreated = "tenant.created"
	EventTenantUpdated = "tenant.updated"

	// Email events
	EventEmailSent   = "email.sent"
	EventEmailFailed = "email.failed"
)

// Event represents an event emitted by the system
type Event struct {
	Type     string          `json:"type"`
	TenantID uuid.UUID       `json:"tenant_id"`
	Data     json.RawMessage `json:"data"`
	Time     time.Time       `json:"time"`
}

// HookHandler is a function that handles an event
type HookHandler func(ctx context.Context, event Event) error

// pluginHookHandler wraps a handler with plugin metadata
type pluginHookHandler struct {
	PluginID    uuid.UUID
	HandlerName string
	Handler     HookHandler
}

// HookRegistry manages event subscriptions
type HookRegistry struct {
	handlers map[string][]pluginHookHandler
	mu       sync.RWMutex
}

// NewHookRegistry creates a new hook registry
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		handlers: make(map[string][]pluginHookHandler),
	}
}

// Register registers a handler for an event type
func (r *HookRegistry) Register(eventType string, handler HookHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[eventType] = append(r.handlers[eventType], pluginHookHandler{
		Handler: handler,
	})
}

// registerPluginHook registers a plugin's hook handler
func (r *HookRegistry) registerPluginHook(pluginID uuid.UUID, eventType, handlerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Note: The actual handler implementation would be loaded from the plugin
	// For now, we store the metadata and log when the hook is called
	r.handlers[eventType] = append(r.handlers[eventType], pluginHookHandler{
		PluginID:    pluginID,
		HandlerName: handlerName,
		Handler: func(ctx context.Context, event Event) error {
			// This is a placeholder - actual implementation would invoke the plugin's handler
			log.Debug().
				Str("plugin_id", pluginID.String()).
				Str("handler", handlerName).
				Str("event", eventType).
				Msg("Plugin hook invoked")
			return nil
		},
	})
}

// unregisterPluginHooks removes all hooks for a plugin
func (r *HookRegistry) unregisterPluginHooks(pluginID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for eventType, handlers := range r.handlers {
		var filtered []pluginHookHandler
		for _, h := range handlers {
			if h.PluginID != pluginID {
				filtered = append(filtered, h)
			}
		}
		r.handlers[eventType] = filtered
	}
}

// Emit fires an event to all registered handlers
func (r *HookRegistry) Emit(ctx context.Context, event Event) error {
	r.mu.RLock()
	handlers := r.handlers[event.Type]
	r.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	log.Debug().
		Str("event", event.Type).
		Int("handlers", len(handlers)).
		Msg("Emitting event")

	// Execute handlers
	// Note: In production, consider running handlers in goroutines with timeouts
	for _, h := range handlers {
		if err := h.Handler(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("event", event.Type).
				Str("plugin_id", h.PluginID.String()).
				Str("handler", h.HandlerName).
				Msg("Hook handler failed")
			// Continue with other handlers despite errors
		}
	}

	return nil
}

// EmitAsync fires an event asynchronously (fire-and-forget)
func (r *HookRegistry) EmitAsync(event Event) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := r.Emit(ctx, event); err != nil {
			log.Error().Err(err).Str("event", event.Type).Msg("Async event emission failed")
		}
	}()
}

// HasHandlers checks if there are any handlers for an event type
func (r *HookRegistry) HasHandlers(eventType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[eventType]) > 0
}

// GetEventTypes returns all registered event types
func (r *HookRegistry) GetEventTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}

// GetHandlerCount returns the number of handlers for an event type
func (r *HookRegistry) GetHandlerCount(eventType string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[eventType])
}

// Helper functions to create common events

// NewInvoiceEvent creates an invoice-related event
func NewInvoiceEvent(eventType string, tenantID uuid.UUID, invoiceData interface{}) Event {
	data, _ := json.Marshal(invoiceData)
	return Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     data,
		Time:     time.Now(),
	}
}

// NewPaymentEvent creates a payment-related event
func NewPaymentEvent(eventType string, tenantID uuid.UUID, paymentData interface{}) Event {
	data, _ := json.Marshal(paymentData)
	return Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     data,
		Time:     time.Now(),
	}
}

// NewContactEvent creates a contact-related event
func NewContactEvent(eventType string, tenantID uuid.UUID, contactData interface{}) Event {
	data, _ := json.Marshal(contactData)
	return Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     data,
		Time:     time.Now(),
	}
}

// NewGenericEvent creates a generic event
func NewGenericEvent(eventType string, tenantID uuid.UUID, data interface{}) Event {
	jsonData, _ := json.Marshal(data)
	return Event{
		Type:     eventType,
		TenantID: tenantID,
		Data:     jsonData,
		Time:     time.Now(),
	}
}

// AllEventTypes returns all available event types
var AllEventTypes = []string{
	EventInvoiceCreated,
	EventInvoiceSent,
	EventInvoicePaid,
	EventInvoiceVoided,
	EventPaymentReceived,
	EventPaymentAllocated,
	EventContactCreated,
	EventContactUpdated,
	EventContactDeleted,
	EventJournalEntryCreated,
	EventJournalEntryPosted,
	EventJournalEntryVoided,
	EventRecurringCreated,
	EventRecurringGenerated,
	EventRecurringStopped,
	EventBankTransactionImported,
	EventBankTransactionMatched,
	EventReconciliationCompleted,
	EventPayrollCalculated,
	EventPayrollApproved,
	EventEmployeeCreated,
	EventTenantCreated,
	EventTenantUpdated,
	EventEmailSent,
	EventEmailFailed,
}

// IsValidEventType checks if an event type is valid
func IsValidEventType(eventType string) bool {
	for _, t := range AllEventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}
