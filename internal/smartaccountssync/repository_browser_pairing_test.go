package smartaccountssync

import (
	"testing"
	"time"
)

func TestBrowserPairingToRecordUsesNullUntilTheRelayClaimsASource(t *testing.T) {
	issued := browserPairingToRecord(BrowserPairing{
		ID:          "493a720c-c186-4b49-997f-637f0124cc65",
		TenantID:    "93eef4d0-0f71-4f42-8b32-33a07dc12191",
		TokenSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Status:      BrowserPairingStatusIssued,
		ExpiresAt:   time.Date(2026, 8, 28, 6, 10, 0, 0, time.UTC),
		CreatedAt:   time.Date(2026, 8, 28, 6, 0, 0, 0, time.UTC),
	})
	if issued.SourceCompanyID != nil {
		t.Fatalf("issued pairing source_company_id = %q, want SQL NULL", *issued.SourceCompanyID)
	}

	claimed := browserPairingToRecord(BrowserPairing{SourceCompanyID: " sa-browser-v1-1234 "})
	if claimed.SourceCompanyID == nil || *claimed.SourceCompanyID != "sa-browser-v1-1234" {
		t.Fatalf("claimed pairing source_company_id = %#v, want normalized opaque selector", claimed.SourceCompanyID)
	}
	if got := browserPairingFromRecord(&issued).SourceCompanyID; got != "" {
		t.Fatalf("issued pairing source company = %q, want empty API value", got)
	}
}
