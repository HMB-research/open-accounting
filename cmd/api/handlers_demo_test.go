package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/HMB-research/open-accounting/internal/demo"
)

type mockDemoResetter struct {
	users    []demo.ResetUser
	userNums []int
	err      error
	calls    int
}

func (m *mockDemoResetter) Reset(ctx context.Context, users []demo.ResetUser, userNums []int) error {
	m.calls++
	m.users = append([]demo.ResetUser(nil), users...)
	m.userNums = append([]int(nil), userNums...)
	return m.err
}

type mockDemoStatusReader struct {
	status  demo.StatusResponse
	err     error
	schema  string
	userNum int
	calls   int
}

func (m *mockDemoStatusReader) ReadDemoStatus(ctx context.Context, schema string, userNum int) (demo.StatusResponse, error) {
	m.calls++
	m.schema = schema
	m.userNum = userNum
	return m.status, m.err
}

func TestDemoUserSelectionHelpers(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3, 4}, demoUserNumbers())

	user, ok := demoUserByNumber(3)
	assert.True(t, ok)
	assert.Equal(t, "demo3@example.com", user.email)
	assert.Equal(t, "tenant_demo3", user.schema)

	_, ok = demoUserByNumber(99)
	assert.False(t, ok)

	userNum, err := parseDemoUserNumber("2")
	assert.NoError(t, err)
	assert.Equal(t, 2, userNum)

	_, err = parseDemoUserNumber("bad")
	assert.Error(t, err)
	_, err = parseDemoUserNumber("99")
	assert.Error(t, err)

	users, userNums, err := demoUsersForSelection("")
	assert.NoError(t, err)
	assert.Len(t, users, 4)
	assert.Equal(t, []int{1, 2, 3, 4}, userNums)

	users, userNums, err = demoUsersForSelection("4")
	assert.NoError(t, err)
	if assert.Len(t, users, 1) {
		assert.Equal(t, "demo4", users[0].slug)
	}
	assert.Equal(t, []int{4}, userNums)
}

func TestDemoResetHandler(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	rr := httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Demo mode is not enabled")

	t.Setenv("DEMO_MODE", "true")
	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Demo reset not configured")

	t.Setenv("DEMO_RESET_SECRET", "demo-secret")
	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	req.Header.Set("X-Demo-Secret", "wrong-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid or missing secret key")

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?user=99", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid user parameter")

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?secret=demo-secret", nil)
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to initialize demo reset")

	resetter := &mockDemoResetter{}
	h.demoResetService = resetter
	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?secret=demo-secret", nil)
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, resetter.calls)
	assert.Equal(t, []int{1, 2, 3, 4}, resetter.userNums)
	assert.Len(t, resetter.users, 4)
	assert.Equal(t, "demo1@example.com", resetter.users[0].Email)

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?user=2", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []int{2}, resetter.userNums)
	if assert.Len(t, resetter.users, 1) {
		assert.Equal(t, "tenant_demo2", resetter.users[0].Schema)
	}

	resetter.err = errors.New("seed failed")
	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to reset demo data")
}

func TestDemoStatusHandler(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	rr := httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Demo mode is not enabled")

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("DEMO_RESET_SECRET", "demo-secret")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "User parameter is required")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=99", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid user parameter")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to read demo status")

	reader := &mockDemoStatusReader{
		status: demo.StatusResponse{
			User:              2,
			Accounts:          demo.EntityStatus{Count: 2, Keys: []string{"Cash", "Revenue"}},
			Contacts:          demo.EntityStatus{Count: 1, Keys: []string{"Customer A"}},
			Invoices:          demo.EntityStatus{Count: 1, Keys: []string{"INV-001"}},
			Employees:         demo.EntityStatus{Count: 1, Keys: []string{"Mari Maasikas"}},
			Payments:          demo.EntityStatus{Count: 1, Keys: []string{"PAY-001"}},
			JournalEntries:    demo.EntityStatus{Count: 1, Keys: []string{"JE-001"}},
			BankAccounts:      demo.EntityStatus{Count: 1, Keys: []string{"Operating"}},
			RecurringInvoices: demo.EntityStatus{Count: 1, Keys: []string{"Monthly"}},
			PayrollRuns:       demo.EntityStatus{Count: 1, Keys: []string{"2026-03"}},
			TsdDeclarations:   demo.EntityStatus{Count: 1, Keys: []string{"2026-01"}},
		},
	}
	h.demoStatusReader = reader
	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=2", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "tenant_demo2", reader.schema)
	assert.Equal(t, 2, reader.userNum)

	var response DemoStatusResponse
	if assert.NoError(t, json.NewDecoder(rr.Body).Decode(&response)) {
		assert.Equal(t, 2, response.User)
		assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"Cash", "Revenue"}}, response.Accounts)
		assert.Equal(t, EntityStatus{Count: 1, Keys: []string{"2026-01"}}, response.TsdDeclarations)
	}

	reader.err = errors.New("status failed")
	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=2", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to read demo status")
}
