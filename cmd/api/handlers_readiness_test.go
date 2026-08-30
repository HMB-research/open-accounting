package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

type readinessBridgeClient struct {
	err error
}

type readinessDatabase struct {
	err error
}

func (database readinessDatabase) Ping(context.Context) error { return database.err }

func (c readinessBridgeClient) Health(context.Context) error { return c.err }

type bridgeClientWithHealth struct {
	smartaccountssync.UnavailableBridgeClient
	readinessBridgeClient readinessBridgeClient
}

func (c bridgeClientWithHealth) Health(ctx context.Context) error {
	return c.readinessBridgeClient.Health(ctx)
}

func TestReadinessCheckReportsOnlySafeAggregateState(t *testing.T) {
	for _, test := range []struct {
		name       string
		client     any
		database   readinessPinger
		wantStatus int
		wantBody   string
	}{
		{name: "ready", client: readinessBridgeClient{}, database: readinessDatabase{}, wantStatus: http.StatusOK, wantBody: `"status":"ready"`},
		{name: "bridge unavailable", client: readinessBridgeClient{err: errors.New("sensitive upstream failure")}, database: readinessDatabase{}, wantStatus: http.StatusServiceUnavailable, wantBody: `"status":"not_ready"`},
		{name: "database unavailable", client: readinessBridgeClient{}, database: readinessDatabase{err: errors.New("sensitive database failure")}, wantStatus: http.StatusServiceUnavailable, wantBody: `"status":"not_ready"`},
		{name: "no health seam", client: nil, database: readinessDatabase{}, wantStatus: http.StatusServiceUnavailable, wantBody: `"status":"not_ready"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			handlers := &Handlers{}
			handlers.readinessDatabase = test.database
			if client, ok := test.client.(readinessBridgeClient); ok {
				// The readiness route uses only the optional health seam. The full
				// BridgeClient remains intentionally absent in this focused test.
				handlers.smartAccountsBridgeClient = bridgeClientWithHealth{readinessBridgeClient: client}
			}
			request := httptest.NewRequest(http.MethodGet, "/ready", nil)
			recorder := httptest.NewRecorder()
			handlers.ReadinessCheck(recorder, request)
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantBody) {
				t.Fatalf("readiness = %d %s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "sensitive") || recorder.Header().Get("Cache-Control") != "no-store" || !strings.Contains(recorder.Body.String(), `"name":"database"`) {
				t.Fatalf("unsafe readiness response: headers=%v body=%s", recorder.Header(), recorder.Body.String())
			}
		})
	}
}
