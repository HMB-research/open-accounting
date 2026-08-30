package smartaccountssync

// These identifiers are the one reviewed browser-to-canonical GL boundary.
// They intentionally differ from journal_entries: that visible grid is an
// account/journal summary and may be retained only as raw archive evidence.
const (
	BrowserGeneralLedgerResourceID         = "general_ledger"
	BrowserGeneralLedgerCSVSchemaID        = "general_ledger_csv_v1"
	BrowserGeneralLedgerCSVSourceSchema    = BrowserCaptureManifestVersion + "/" + BrowserGeneralLedgerCSVSchemaID
	BrowserJournalEntriesSummaryResourceID = "journal_entries"
)

func approvedBrowserGeneralLedgerSchema(resourceID, schemaID string) bool {
	return resourceID == BrowserGeneralLedgerResourceID && schemaID == BrowserGeneralLedgerCSVSchemaID
}
