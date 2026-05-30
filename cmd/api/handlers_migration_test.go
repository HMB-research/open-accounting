package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMigrationBundleHandler(t *testing.T) {
	h := &Handlers{}
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name\nCUST-1,Customer One\n",
			},
			{
				Kind:       cutover.KindInvoices,
				FileName:   "invoices.csv",
				CSVContent: "invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nINV-1,CUST-404,2026-05-30,Work,1,100,22\n",
			},
		},
	}, claims), map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var report cutover.BundleValidationReport
	require.NoError(t, json.NewDecoder(w.Body).Decode(&report))
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, cutover.KindContacts, report.Issues[0].TargetKind)
}

func TestValidateMigrationBundleHandlerRejectsEmptyRequest(t *testing.T) {
	h := &Handlers{}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "at least one migration file is required")
}
