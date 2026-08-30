package smartaccountssync

import "testing"

func TestGeneratedBrowserDiscoveryProtocolRegression(t *testing.T) {
	// Updating this digest is an explicit cross-repository protocol review: the
	// private generator must regenerate this file and its checked JSON/extension
	// consumers in the same change.
	const expectedProtocolSHA256 = "94d8793b474ca1c57e722c57dc608031aed70dced93f31437c54a3310b5a1553"
	if BrowserDiscoveryProtocolVersion != "smartaccounts-browser-discovery-protocol-v1" || BrowserDiscoveryProtocolSHA256 != expectedProtocolSHA256 {
		t.Fatalf("unexpected generated browser discovery protocol %q/%q", BrowserDiscoveryProtocolVersion, BrowserDiscoveryProtocolSHA256)
	}
	if BrowserDiscoveryManifestVersion != "smartaccounts-brave-ui-v2" || BrowserDiscoveryContractVersion != "smartaccounts-brave-discovery-contract-v1" {
		t.Fatalf("unexpected generated browser versions %q/%q", BrowserDiscoveryManifestVersion, BrowserDiscoveryContractVersion)
	}
	if BrowserDiscoveryMaxReceiptBytes != 1<<20 || BrowserDiscoveryMaxHeaderNames != 128 || BrowserDiscoveryMaxHeaderNameUTF8Bytes != 120 || BrowserDiscoveryMaxControlIDBytes != 80 || BrowserDiscoveryMaxPathBytes != 512 || BrowserCaptureMaxResourceBytes != 32<<20 {
		t.Fatalf("unexpected generated browser discovery limits")
	}

	ids := browserDiscoveryResourceIDs()
	if len(ids) != 31 {
		t.Fatalf("generated browser discovery resource count = %d, want 31", len(ids))
	}
	seen := make(map[string]struct{}, len(ids))
	exportCount, pageOnlyCount := 0, 0
	for _, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("generated browser discovery resource %q is duplicated", id)
		}
		seen[id] = struct{}{}
		coverage, found := browserDiscoveryResourceCoverage(id)
		if !found {
			t.Fatalf("generated browser discovery resource %q has no coverage", id)
		}
		switch coverage {
		case "export_csv":
			exportCount++
		case "page_only":
			pageOnlyCount++
		default:
			t.Fatalf("generated browser discovery resource %q has invalid coverage %q", id, coverage)
		}
	}
	if exportCount != 24 || pageOnlyCount != 7 {
		t.Fatalf("generated coverage counts = %d export / %d page-only, want 24 / 7", exportCount, pageOnlyCount)
	}
	if coverage, found := browserDiscoveryResourceCoverage("journal_entries"); !found || coverage != "export_csv" {
		t.Fatalf("journal_entries coverage = %q, found=%t", coverage, found)
	}
	if !approvedBrowserGeneralLedgerSchema(BrowserGeneralLedgerResourceID, BrowserGeneralLedgerCSVSchemaID) || approvedBrowserGeneralLedgerSchema(BrowserJournalEntriesSummaryResourceID, BrowserGeneralLedgerCSVSchemaID) {
		t.Fatalf("generated contract must allow only the reviewed General Ledger CSV adapter")
	}
	if coverage, found := browserDiscoveryResourceCoverage("warehouse_inventory"); !found || coverage != "page_only" {
		t.Fatalf("warehouse_inventory coverage = %q, found=%t", coverage, found)
	}
}
