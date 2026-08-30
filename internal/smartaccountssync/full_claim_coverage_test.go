package smartaccountssync

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentFullClaimCoverageMatrixCoversEveryV2BrowserAndAPIv17Surface(t *testing.T) {
	matrix := CurrentFullClaimCoverageMatrix()
	bySource := map[string][]FullClaimCoverageRow{}
	for _, row := range matrix {
		require.NotEmpty(t, row.Source)
		require.NotEmpty(t, row.ResourceID)
		require.NotEmpty(t, row.ContractVersion)
		require.NotEmpty(t, row.Disposition)
		bySource[row.Source] = append(bySource[row.Source], row)
	}

	browser := bySource[fullClaimSourceBraveV2]
	require.Len(t, browser, len(browserDiscoveryProtocolResources))
	seenBrowser := map[string]FullClaimCoverageRow{}
	for _, row := range browser {
		_, exists := seenBrowser[row.ResourceID]
		assert.False(t, exists, "duplicate browser v2 resource %q", row.ResourceID)
		seenBrowser[row.ResourceID] = row
	}
	for _, resource := range browserDiscoveryProtocolResources {
		row, found := seenBrowser[resource.ID]
		require.True(t, found, "v2 manifest resource %q is absent from the full-claim matrix", resource.ID)
		if resource.ID == BrowserGeneralLedgerResourceID {
			assert.Equal(t, FullClaimDispositionGLApplyGated, row.Disposition)
			assert.Equal(t, BrowserGeneralLedgerCSVSourceSchema, row.ContractVersion)
			continue
		}
		assert.Equal(t, BrowserDiscoveryManifestVersion, row.ContractVersion, resource.ID)
		if resource.ID == BrowserJournalEntriesSummaryResourceID {
			assert.Equal(t, FullClaimDispositionUnconsumed, row.Disposition)
			continue
		}
		if resource.Coverage == "page_only" {
			assert.Equal(t, FullClaimDispositionPageOnlyRequired, row.Disposition)
			continue
		}
		assert.Equal(t, FullClaimDispositionFilterRequired, row.Disposition)
	}
	assert.Equal(t, 22, countDisposition(browser, FullClaimDispositionFilterRequired))
	assert.Equal(t, 23, countDisposition(browser, FullClaimDispositionFilterRequired)+countDisposition(browser, FullClaimDispositionUnconsumed), "the 24 v2 CSV surfaces are one GL plus 23 still not full-claim complete")
	assert.Equal(t, 7, countDisposition(browser, FullClaimDispositionPageOnlyRequired))

	api := bySource[fullClaimSourceAPIv17]
	require.Len(t, api, len(fullClaimAPIResourceIDs))
	seenAPI := map[string]bool{}
	for _, row := range api {
		assert.Equal(t, fullClaimAPIContractVersion, row.ContractVersion)
		assert.False(t, seenAPI[row.ResourceID], "duplicate API resource %q", row.ResourceID)
		seenAPI[row.ResourceID] = true
	}
	for _, resourceID := range fullClaimAPIResourceIDs {
		assert.True(t, seenAPI[resourceID], "documented API resource %q is absent from the full-claim matrix", resourceID)
	}
	apiIDsSHA256 := sha256.Sum256([]byte(strings.Join(fullClaimAPIResourceIDs[:], "\n")))
	assert.Equal(t, "af100df74f218152c1014014c69cd2d98fc8d8bdc0d9425f07b5841c3a333ec0", hex.EncodeToString(apiIDsSHA256[:]), "the full-claim catalog must be reviewed when the documented API inventory changes")
	assert.Equal(t, FullClaimDispositionMissingAPIEndpoint, matrixRow(t, api, "general.account_balances.get").Disposition)
	assert.Equal(t, FullClaimDispositionMissingAPIEndpoint, matrixRow(t, api, "settings.warehouses.get").Disposition)

	assert.Len(t, bySource[fullClaimSourceBraveMasterDetail], 3)
	assert.Len(t, bySource[fullClaimSourceBraveCommercial], 2)
}

func TestAssessFullClaimEligibilityFailsClosedForEveryKnownGap(t *testing.T) {
	ready := FullClaimCoverageRow{Source: "synthetic", ResourceID: "resource", ContractVersion: "v1", Disposition: FullClaimDispositionResolved}
	assert.True(t, AssessFullClaimEligibility([]FullClaimCoverageRow{ready}, 0).FullClaimEligible)

	for _, disposition := range []string{
		FullClaimDispositionFilterRequired,
		FullClaimDispositionPageOnlyRequired,
		FullClaimDispositionReviewRequired,
		FullClaimDispositionUnconsumed,
		FullClaimDispositionMissingAPIEndpoint,
	} {
		result := AssessFullClaimEligibility([]FullClaimCoverageRow{{Source: "synthetic", ResourceID: "resource", ContractVersion: "v1", Disposition: disposition}}, 0)
		assert.False(t, result.FullClaimEligible, disposition)
		assert.Equal(t, []string{"synthetic:resource"}, result.BlockingResources, disposition)
	}

	result := AssessFullClaimEligibility([]FullClaimCoverageRow{ready}, 1)
	assert.False(t, result.FullClaimEligible)
	assert.Equal(t, []string{"package:unresolved_tombstones"}, result.BlockingResources)

	result = AssessFullClaimEligibility([]FullClaimCoverageRow{{Source: "synthetic", ResourceID: "resource", ContractVersion: "v1", Disposition: FullClaimDispositionArchiveOnly, BlockReason: "pending_review"}}, 0)
	assert.False(t, result.FullClaimEligible, "a declared blocker must be fail-closed even for a reviewed archival disposition")

	result = AssessFullClaimEligibility([]FullClaimCoverageRow{{Source: "synthetic", ResourceID: "resource", ContractVersion: "v1", Disposition: "future_disposition"}}, 0)
	assert.False(t, result.FullClaimEligible, "an unknown disposition must fail closed")
}

func TestCurrentFullClaimCoverageMatrixIsNotAFullSyncClaim(t *testing.T) {
	result := AssessFullClaimEligibility(CurrentFullClaimCoverageMatrix(), 0)
	assert.False(t, result.FullClaimEligible)
	assert.Contains(t, result.BlockingResources, fullClaimSourceBraveV2+":warehouse_inventory")
	assert.Contains(t, result.BlockingResources, fullClaimSourceAPIv17+":general.account_balances.get")
	assert.Contains(t, result.BlockingResources, fullClaimSourceBraveCommercial+":client_invoices")
}

func TestCurrentFullClaimDomainPlanSelectsOneRouteAndRetainsEveryAlternative(t *testing.T) {
	matrix := CurrentFullClaimCoverageMatrix()
	plan := CurrentFullClaimDomainPlan()
	require.NotEmpty(t, plan)

	inventory := make(map[string]struct{}, len(matrix))
	for _, row := range matrix {
		domainID, classified := fullClaimDomainForRoute(row)
		require.True(t, classified, "route must be assigned to a business domain: %#v", row)
		require.NotEmpty(t, domainID)
		key := fullClaimRouteKey(row)
		_, duplicate := inventory[key]
		require.False(t, duplicate, "duplicate inventory route %q", key)
		inventory[key] = struct{}{}
	}

	covered := make(map[string]struct{}, len(matrix))
	seenDomains := make(map[string]struct{}, len(plan))
	for _, entry := range plan {
		require.True(t, validFullClaimDomainPlanEntry(entry), "invalid plan entry %#v", entry)
		_, duplicate := seenDomains[entry.DomainID]
		require.False(t, duplicate, "duplicate domain %q", entry.DomainID)
		seenDomains[entry.DomainID] = struct{}{}
		for _, route := range append([]FullClaimCoverageRow{entry.Selected}, entry.Alternatives...) {
			key := fullClaimRouteKey(route)
			_, known := inventory[key]
			require.True(t, known, "plan includes non-inventory route %q", key)
			_, duplicate := covered[key]
			require.False(t, duplicate, "inventory route selected more than once %q", key)
			covered[key] = struct{}{}
		}
	}
	assert.Equal(t, inventory, covered, "every observed route must remain selected or auditable as an alternative")
}

func TestFullClaimDomainPlanAPISourceDoesNotRequireBrowserDuplicate(t *testing.T) {
	clients := fullClaimPlanEntry(t, "clients")
	require.Equal(t, fullClaimSourceAPIv17, clients.Selected.Source)
	require.Equal(t, "purchasesales.clients.get", clients.Selected.ResourceID)
	assert.True(t, fullClaimPlanContains(clients.Alternatives, fullClaimSourceBraveV2, "clients"))
	assert.True(t, fullClaimPlanContains(clients.Alternatives, fullClaimSourceBraveMasterDetail, BrowserMasterDetailClientsResource))

	result := AssessFullClaimDomainPlanEligibility([]FullClaimDomainPlanEntry{clients}, []FullClaimDomainEvidence{completeDomainEvidence(clients)}, 0)
	assert.True(t, result.FullClaimEligible, "an evidence-complete API primary must not be blocked by unselected browser fallbacks: %#v", result)
}

func TestFullClaimDomainPlanFailsClosedForUnprovenAndHardBlockedSelections(t *testing.T) {
	clients := fullClaimPlanEntry(t, "clients")
	missingEvidence := AssessFullClaimDomainPlanEligibility([]FullClaimDomainPlanEntry{clients}, nil, 0)
	assert.False(t, missingEvidence.FullClaimEligible)
	assert.Equal(t, []string{"domain:clients"}, missingEvidence.BlockingResources)

	incomplete := completeDomainEvidence(clients)
	incomplete.SchemaValidated = false
	missingSchema := AssessFullClaimDomainPlanEligibility([]FullClaimDomainPlanEntry{clients}, []FullClaimDomainEvidence{incomplete}, 0)
	assert.False(t, missingSchema.FullClaimEligible)
	assert.Equal(t, []string{"domain:clients"}, missingSchema.BlockingResources)

	warehouses := fullClaimPlanEntry(t, "warehouses")
	require.Equal(t, fullClaimSourceAPIv17, warehouses.Selected.Source)
	require.Equal(t, FullClaimDispositionMissingAPIEndpoint, warehouses.Selected.Disposition)
	hardBlocked := AssessFullClaimDomainPlanEligibility([]FullClaimDomainPlanEntry{warehouses}, []FullClaimDomainEvidence{completeDomainEvidence(warehouses)}, 0)
	assert.False(t, hardBlocked.FullClaimEligible)
	assert.Equal(t, []string{"domain:warehouses"}, hardBlocked.BlockingResources)

	assert.False(t, AssessFullClaimDomainPlanEligibility([]FullClaimDomainPlanEntry{clients}, []FullClaimDomainEvidence{completeDomainEvidence(clients)}, 1).FullClaimEligible, "a selected route cannot bypass unresolved tombstones")
}

func fullClaimPlanEntry(t *testing.T, domainID string) FullClaimDomainPlanEntry {
	t.Helper()
	for _, entry := range CurrentFullClaimDomainPlan() {
		if entry.DomainID == domainID {
			return entry
		}
	}
	t.Fatalf("domain plan entry %q not found", domainID)
	return FullClaimDomainPlanEntry{}
}

func fullClaimPlanContains(rows []FullClaimCoverageRow, source, resourceID string) bool {
	for _, row := range rows {
		if row.Source == source && row.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func completeDomainEvidence(entry FullClaimDomainPlanEntry) FullClaimDomainEvidence {
	return FullClaimDomainEvidence{
		PlanVersion:             entry.PlanVersion,
		DomainID:                entry.DomainID,
		Source:                  entry.Selected.Source,
		ResourceID:              entry.Selected.ResourceID,
		ContractVersion:         entry.Selected.ContractVersion,
		LiveSourceValidated:     true,
		SchemaValidated:         true,
		CompletenessValidated:   true,
		ReconciliationValidated: true,
		TombstonesResolved:      true,
		AccountantAttested:      true,
	}
}

func countDisposition(rows []FullClaimCoverageRow, disposition string) int {
	count := 0
	for _, row := range rows {
		if row.Disposition == disposition {
			count++
		}
	}
	return count
}

func matrixRow(t *testing.T, rows []FullClaimCoverageRow, resourceID string) FullClaimCoverageRow {
	t.Helper()
	for _, row := range rows {
		if row.ResourceID == resourceID {
			return row
		}
	}
	t.Fatalf("matrix row %q not found", resourceID)
	return FullClaimCoverageRow{}
}
