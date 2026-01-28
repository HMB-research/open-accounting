package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewHookRegistry(t *testing.T) {
	registry := NewHookRegistry()
	if registry == nil {
		t.Fatal("NewHookRegistry() returned nil")
	}
	if registry.handlers == nil {
		t.Error("HookRegistry.handlers is nil")
	}
}

func TestHookRegistry_Register(t *testing.T) {
	registry := NewHookRegistry()

	handlerCalled := false
	handler := func(ctx context.Context, event Event) error {
		handlerCalled = true
		return nil
	}

	registry.Register(EventInvoiceCreated, handler)

	if !registry.HasHandlers(EventInvoiceCreated) {
		t.Error("HasHandlers() = false after Register()")
	}

	// Emit event to test handler
	event := Event{
		Type:     EventInvoiceCreated,
		TenantID: uuid.New(),
		Time:     time.Now(),
	}

	err := registry.Emit(context.Background(), event)
	if err != nil {
		t.Errorf("Emit() error = %v", err)
	}

	if !handlerCalled {
		t.Error("Handler was not called after Emit()")
	}
}

func TestHookRegistry_RegisterMultipleHandlers(t *testing.T) {
	registry := NewHookRegistry()

	var callOrder []int
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		idx := i
		registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
			mu.Lock()
			callOrder = append(callOrder, idx)
			mu.Unlock()
			return nil
		})
	}

	if registry.GetHandlerCount(EventInvoiceCreated) != 3 {
		t.Errorf("GetHandlerCount() = %d, want 3", registry.GetHandlerCount(EventInvoiceCreated))
	}

	event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
	_ = registry.Emit(context.Background(), event)

	if len(callOrder) != 3 {
		t.Errorf("Expected 3 handlers called, got %d", len(callOrder))
	}
}

func TestHookRegistry_EmitNoHandlers(t *testing.T) {
	registry := NewHookRegistry()

	event := Event{
		Type:     EventInvoiceCreated,
		TenantID: uuid.New(),
		Time:     time.Now(),
	}

	// Should not error when no handlers are registered
	err := registry.Emit(context.Background(), event)
	if err != nil {
		t.Errorf("Emit() with no handlers error = %v", err)
	}
}

func TestHookRegistry_EmitHandlerError(t *testing.T) {
	registry := NewHookRegistry()

	var firstHandlerCalled, secondHandlerCalled bool

	registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
		firstHandlerCalled = true
		return errors.New("handler error")
	})

	registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
		secondHandlerCalled = true
		return nil
	})

	event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
	err := registry.Emit(context.Background(), event)

	// Emit should not return error even if handlers fail
	if err != nil {
		t.Errorf("Emit() error = %v, want nil", err)
	}

	// Both handlers should be called even if first one fails
	if !firstHandlerCalled || !secondHandlerCalled {
		t.Error("Not all handlers were called when one failed")
	}
}

func TestHookRegistry_HasHandlers(t *testing.T) {
	registry := NewHookRegistry()

	if registry.HasHandlers(EventInvoiceCreated) {
		t.Error("HasHandlers() = true for empty registry")
	}

	registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
		return nil
	})

	if !registry.HasHandlers(EventInvoiceCreated) {
		t.Error("HasHandlers() = false after Register()")
	}

	if registry.HasHandlers(EventPaymentReceived) {
		t.Error("HasHandlers() = true for unregistered event")
	}
}

func TestHookRegistry_GetEventTypes(t *testing.T) {
	registry := NewHookRegistry()

	types := registry.GetEventTypes()
	if len(types) != 0 {
		t.Errorf("GetEventTypes() = %d for empty registry", len(types))
	}

	handler := func(ctx context.Context, event Event) error { return nil }
	registry.Register(EventInvoiceCreated, handler)
	registry.Register(EventPaymentReceived, handler)
	registry.Register(EventContactCreated, handler)

	types = registry.GetEventTypes()
	if len(types) != 3 {
		t.Errorf("GetEventTypes() = %d, want 3", len(types))
	}
}

func TestHookRegistry_GetHandlerCount(t *testing.T) {
	registry := NewHookRegistry()

	if registry.GetHandlerCount(EventInvoiceCreated) != 0 {
		t.Error("GetHandlerCount() != 0 for empty registry")
	}

	handler := func(ctx context.Context, event Event) error { return nil }
	registry.Register(EventInvoiceCreated, handler)
	registry.Register(EventInvoiceCreated, handler)

	if registry.GetHandlerCount(EventInvoiceCreated) != 2 {
		t.Errorf("GetHandlerCount() = %d, want 2", registry.GetHandlerCount(EventInvoiceCreated))
	}

	if registry.GetHandlerCount(EventPaymentReceived) != 0 {
		t.Error("GetHandlerCount() != 0 for unregistered event")
	}
}

func TestHookRegistry_RegisterPluginHook(t *testing.T) {
	registry := NewHookRegistry()
	pluginID := uuid.New()

	registry.registerPluginHook(pluginID, EventInvoiceCreated, "OnInvoiceCreated")

	if !registry.HasHandlers(EventInvoiceCreated) {
		t.Error("HasHandlers() = false after registerPluginHook()")
	}

	if registry.GetHandlerCount(EventInvoiceCreated) != 1 {
		t.Errorf("GetHandlerCount() = %d, want 1", registry.GetHandlerCount(EventInvoiceCreated))
	}
}

func TestHookRegistry_RegisterPluginHook_EmitEvent(t *testing.T) {
	registry := NewHookRegistry()
	pluginID := uuid.New()

	// Register a plugin hook
	registry.registerPluginHook(pluginID, EventInvoiceCreated, "OnInvoiceCreated")

	// Emit an event to the plugin hook handler
	event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
	err := registry.Emit(context.Background(), event)

	// The plugin hook handler just logs and returns nil
	if err != nil {
		t.Errorf("Emit() error = %v, want nil", err)
	}
}

func TestHookRegistry_UnregisterPluginHooks(t *testing.T) {
	registry := NewHookRegistry()
	pluginID1 := uuid.New()
	pluginID2 := uuid.New()

	registry.registerPluginHook(pluginID1, EventInvoiceCreated, "Handler1")
	registry.registerPluginHook(pluginID1, EventPaymentReceived, "Handler2")
	registry.registerPluginHook(pluginID2, EventInvoiceCreated, "Handler3")

	if registry.GetHandlerCount(EventInvoiceCreated) != 2 {
		t.Errorf("Before unregister: GetHandlerCount() = %d, want 2", registry.GetHandlerCount(EventInvoiceCreated))
	}

	registry.unregisterPluginHooks(pluginID1)

	if registry.GetHandlerCount(EventInvoiceCreated) != 1 {
		t.Errorf("After unregister: GetHandlerCount() = %d, want 1", registry.GetHandlerCount(EventInvoiceCreated))
	}

	if registry.HasHandlers(EventPaymentReceived) {
		t.Error("Plugin1's PaymentReceived handler should be removed")
	}
}

func TestHookRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewHookRegistry()
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent registrations
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
				return nil
			})
		}()
	}

	// Concurrent emissions
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
			_ = registry.Emit(context.Background(), event)
		}()
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.HasHandlers(EventInvoiceCreated)
			registry.GetHandlerCount(EventInvoiceCreated)
			registry.GetEventTypes()
		}()
	}

	wg.Wait()
	// If we get here without deadlock or panic, the test passes
}

func TestHookRegistry_EmitAsync(t *testing.T) {
	registry := NewHookRegistry()
	var callCount int32

	registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
	registry.EmitAsync(event)

	// Give async handler time to complete
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("EmitAsync handler call count = %d, want 1", callCount)
	}
}

func TestNewInvoiceEvent(t *testing.T) {
	tenantID := uuid.New()
	invoiceData := map[string]string{"id": "inv-123", "amount": "100.00"}

	event := NewInvoiceEvent(EventInvoiceCreated, tenantID, invoiceData)

	if event.Type != EventInvoiceCreated {
		t.Errorf("Event.Type = %q, want %q", event.Type, EventInvoiceCreated)
	}
	if event.TenantID != tenantID {
		t.Error("Event.TenantID mismatch")
	}
	if event.Time.IsZero() {
		t.Error("Event.Time is zero")
	}
	if len(event.Data) == 0 {
		t.Error("Event.Data is empty")
	}
}

func TestNewPaymentEvent(t *testing.T) {
	tenantID := uuid.New()
	paymentData := map[string]interface{}{"id": "pay-123", "amount": 50.00}

	event := NewPaymentEvent(EventPaymentReceived, tenantID, paymentData)

	if event.Type != EventPaymentReceived {
		t.Errorf("Event.Type = %q, want %q", event.Type, EventPaymentReceived)
	}
	if event.TenantID != tenantID {
		t.Error("Event.TenantID mismatch")
	}
}

func TestNewContactEvent(t *testing.T) {
	tenantID := uuid.New()
	contactData := map[string]string{"name": "Test Contact", "email": "test@example.com"}

	event := NewContactEvent(EventContactCreated, tenantID, contactData)

	if event.Type != EventContactCreated {
		t.Errorf("Event.Type = %q, want %q", event.Type, EventContactCreated)
	}
}

func TestNewGenericEvent(t *testing.T) {
	tenantID := uuid.New()
	data := map[string]interface{}{"custom": "data", "count": 42}

	event := NewGenericEvent("custom.event", tenantID, data)

	if event.Type != "custom.event" {
		t.Errorf("Event.Type = %q, want %q", event.Type, "custom.event")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(event.Data, &parsed); err != nil {
		t.Fatalf("Failed to parse event data: %v", err)
	}

	if parsed["custom"] != "data" {
		t.Error("Event data not properly serialized")
	}
}

func TestIsValidEventType(t *testing.T) {
	validEvents := []string{
		EventInvoiceCreated,
		EventInvoiceSent,
		EventInvoicePaid,
		EventPaymentReceived,
		EventContactCreated,
		EventJournalEntryPosted,
		EventPayrollCalculated,
		EventTenantCreated,
	}

	for _, event := range validEvents {
		if !IsValidEventType(event) {
			t.Errorf("IsValidEventType(%q) = false, want true", event)
		}
	}

	invalidEvents := []string{
		"",
		"invalid.event",
		"custom.event",
		"invoice_created",
		"INVOICE.CREATED",
	}

	for _, event := range invalidEvents {
		if IsValidEventType(event) {
			t.Errorf("IsValidEventType(%q) = true, want false", event)
		}
	}
}

func TestAllEventTypes(t *testing.T) {
	if len(AllEventTypes) == 0 {
		t.Fatal("AllEventTypes is empty")
	}

	// Verify expected events are in the list
	expectedEvents := []string{
		EventInvoiceCreated,
		EventPaymentReceived,
		EventContactCreated,
		EventJournalEntryPosted,
		EventPayrollCalculated,
		EventTenantCreated,
		EventEmailSent,
	}

	for _, expected := range expectedEvents {
		found := false
		for _, event := range AllEventTypes {
			if event == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event %q not found in AllEventTypes", expected)
		}
	}
}

func TestHookRegistry_EmitAsyncWithError(t *testing.T) {
	registry := NewHookRegistry()
	var callCount int32

	// Register a handler that returns an error
	registry.Register(EventInvoiceCreated, func(ctx context.Context, event Event) error {
		atomic.AddInt32(&callCount, 1)
		return errors.New("handler error for async test")
	})

	event := Event{Type: EventInvoiceCreated, TenantID: uuid.New(), Time: time.Now()}
	registry.EmitAsync(event)

	// Give async handler time to complete
	time.Sleep(100 * time.Millisecond)

	// Handler should still be called even though it returns an error
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("EmitAsync handler call count = %d, want 1", callCount)
	}
}

func TestEventDataSerialization(t *testing.T) {
	tenantID := uuid.New()

	type InvoiceData struct {
		ID     string  `json:"id"`
		Amount float64 `json:"amount"`
		Status string  `json:"status"`
	}

	invoiceData := InvoiceData{
		ID:     "inv-12345",
		Amount: 199.99,
		Status: "paid",
	}

	event := NewInvoiceEvent(EventInvoicePaid, tenantID, invoiceData)

	// Deserialize and verify
	var parsed InvoiceData
	if err := json.Unmarshal(event.Data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal event data: %v", err)
	}

	if parsed.ID != invoiceData.ID {
		t.Errorf("Parsed ID = %q, want %q", parsed.ID, invoiceData.ID)
	}
	if parsed.Amount != invoiceData.Amount {
		t.Errorf("Parsed Amount = %f, want %f", parsed.Amount, invoiceData.Amount)
	}
	if parsed.Status != invoiceData.Status {
		t.Errorf("Parsed Status = %q, want %q", parsed.Status, invoiceData.Status)
	}
}
