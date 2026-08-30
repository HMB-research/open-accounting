package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
)

type commercialDetailPanicReader struct{}

func (commercialDetailPanicReader) Read([]byte) (int, error) {
	panic("commercial source body must not be read while selector is blocked")
}
func (commercialDetailPanicReader) Close() error { return nil }

func TestSmartAccountsBrowserCommercialDetailCORSAndSelectorPreflightRejectBeforeBodyRead(t *testing.T) {
	h := &Handlers{smartAccountsBrowserCommercialDetailService: smartaccountssync.NewBrowserCommercialDetailService(nil, nil, nil)}
	options := httptest.NewRequest(http.MethodOptions, "/api/v1/smartaccounts-browser-commercial-captures/tenants/tenant/runs/run", nil)
	options.Header.Set("Origin", testBraveExtensionOrigin)
	optionsRecorder := httptest.NewRecorder()
	h.OptionsSmartAccountsBrowserCommercialDetail(optionsRecorder, options)
	assert.Equal(t, http.StatusNoContent, optionsRecorder.Code)
	assert.Contains(t, optionsRecorder.Header().Get("Access-Control-Allow-Methods"), "PUT")
	assert.Contains(t, optionsRecorder.Header().Get("Access-Control-Allow-Headers"), "X-SA-Browser-Commercial-SHA256")

	upload := httptest.NewRequest(http.MethodPut, "/", nil)
	upload.Body = commercialDetailPanicReader{}
	upload.Header.Set("Origin", testBraveExtensionOrigin)
	upload.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
	upload.Header.Set("Content-Type", "application/x-ndjson")
	uploaded := httptest.NewRecorder()
	h.UploadSmartAccountsBrowserCommercialDetail(uploaded, upload)
	assert.Equal(t, http.StatusConflict, uploaded.Code)

	cookieRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieRequest.Header.Set("Origin", testBraveExtensionOrigin)
	cookieRequest.Header.Set("Cookie", "oa_session=must-not-be-used")
	cookieRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserCommercialDetail(cookieRecorder, cookieRequest)
	assert.Equal(t, http.StatusForbidden, cookieRecorder.Code)

	finalize := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	finalize.Header.Set("Origin", testBraveExtensionOrigin)
	finalize.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
	finalize.Header.Set("Content-Type", "application/json")
	finalized := httptest.NewRecorder()
	h.FinalizeSmartAccountsBrowserCommercialDetail(finalized, finalize)
	assert.Equal(t, http.StatusConflict, finalized.Code)
}

var _ io.ReadCloser = commercialDetailPanicReader{}
