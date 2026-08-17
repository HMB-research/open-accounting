package docs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPilotOperationsAssetsDeclareRecoveryAlertsAndSafeEgressGuard(t *testing.T) {
	rules, err := os.ReadFile(filepath.Join("..", "deploy", "monitoring", "open-accounting-pilot-alerts.yml"))
	require.NoError(t, err)
	for _, value := range []string{
		"OpenAccountingBackupFailed",
		"OpenAccountingBackupStale",
		"OpenAccountingOffsiteCopyFailed",
		"OpenAccountingRestoreDrillFailed",
		"OpenAccountingRestoreDrillOverdue",
		"OpenAccountingBackupTimerFailed",
		"runbook_url:",
	} {
		require.Contains(t, string(rules), value)
	}

	script := filepath.Join("..", "deploy", "docker", "apply-webhook-egress-policy.sh")
	check := exec.Command("bash", "-n", script)
	output, err := check.CombinedOutput()
	require.NoErrorf(t, err, "validate egress script: %s", output)
	egress, err := os.ReadFile(script)
	require.NoError(t, err)
	require.Contains(t, string(egress), "ip6tables")
	require.Contains(t, string(egress), "fc00::/7")

	operations, err := os.ReadFile("PILOT_OPERATIONS.md")
	require.NoError(t, err)
	require.True(t, strings.Contains(string(operations), "--dry-run"))
	require.True(t, strings.Contains(string(operations), "26 hours"))
}

func TestSmartAccountsPilotRunbookKeepsProofEvidencePrivate(t *testing.T) {
	runbook, err := os.ReadFile("SMARTACCOUNTS_PILOT_CUTOVER.md")
	require.NoError(t, err)
	for _, value := range []string{
		"outside this public worktree",
		"smartaccounts-proof-plan",
		"smartaccounts-proof-result --require-ready",
		"Never commit original exports",
	} {
		require.Contains(t, string(runbook), value)
	}
}
