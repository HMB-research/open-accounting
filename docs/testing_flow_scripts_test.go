package docs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoTestPackageTimesScript(t *testing.T) {
	input := strings.Join([]string{
		"integration-test (1, 4)\tRun integration tests\t2026-06-10T12:03:26Z ok  \tgithub.com/HMB-research/open-accounting/internal/accounting\t76.415s\tcoverage: 78.2% of statements",
		"ok  \tgithub.com/HMB-research/open-accounting/cmd/api\t14.751s\tcoverage: 71.0% of statements",
		"?   \tgithub.com/HMB-research/open-accounting/internal/unused\t[no test files]",
		"ok  \tgithub.com/example/other/internal/nope\t1.000s\tcoverage: 99.0% of statements",
		"ok  \t./internal/local\t2.500s\tcoverage: 88.0% of statements",
	}, "\n")

	cmd := exec.Command("../scripts/parse-go-test-package-times.sh")
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse package times: %v\n%s", err, output)
	}

	expected := strings.Join([]string{
		"./cmd/api\t14.751",
		"./internal/accounting\t76.415",
		"./internal/local\t2.500",
		"",
	}, "\n")
	if string(output) != expected {
		t.Fatalf("unexpected package times:\nwant:\n%s\ngot:\n%s", expected, output)
	}
}

func TestParsePlaywrightSpecTimesScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	fixture := `{
  "suites": [
    {
      "title": "demo/env.spec.ts",
      "file": "demo/env.spec.ts",
      "specs": [
        {
          "title": "loads environment page",
          "file": "demo/env.spec.ts",
          "tests": [
            {
              "results": [
                {"status": "passed", "duration": 1500}
              ]
            }
          ]
        },
        {
          "title": "retries a slow check",
          "file": "demo/env.spec.ts",
          "tests": [
            {
              "results": [
                {"status": "failed", "duration": 1000},
                {"status": "passed", "duration": 2000}
              ]
            }
          ]
        }
      ]
    },
    {
      "title": "demo/mobile.spec.ts",
      "file": "demo/mobile.spec.ts",
      "specs": [],
      "suites": [
        {
          "title": "Mobile Navigation",
          "file": "demo/mobile.spec.ts",
          "specs": [
            {
              "title": "uses inherited suite file",
              "tests": [
                {
                  "results": [
                    {"status": "passed", "duration": 3000}
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}`

	path := filepath.Join(t.TempDir(), "demo-test-results.json")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command("../scripts/parse-playwright-spec-times.mjs", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse Playwright spec times: %v\n%s", err, output)
	}

	expected := strings.Join([]string{
		"demo/env.spec.ts\t4.500\t2\t3",
		"demo/mobile.spec.ts\t3.000\t1\t1",
		"",
	}, "\n")
	if string(output) != expected {
		t.Fatalf("unexpected Playwright spec times:\nwant:\n%s\ngot:\n%s", expected, output)
	}
}

func TestBackupSystemdScheduleGeneratesProviderSpecificInstallHelper(t *testing.T) {
	outputDir := t.TempDir()

	cmd := exec.Command(
		"../scripts/db-backup-systemd-schedule.sh",
		"--output-dir", outputDir,
		"--scripts-dir", "/opt/open-accounting/scripts",
		"--backup-dir", "/backups",
		"--status-dir", "/var/lib/node_exporter/textfile_collector",
		"--env-file", "/etc/open-accounting/backup.env",
		"--systemd-unit-dir", "/usr/local/lib/systemd/system",
		"--offsite-provider", "rclone",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate backup systemd schedule: %v\n%s", err, output)
	}

	envTemplate, err := os.ReadFile(filepath.Join(outputDir, "open-accounting-backup.env.example"))
	if err != nil {
		t.Fatalf("read generated env template: %v", err)
	}
	env := string(envTemplate)
	for _, snippet := range []string{
		"BACKUP_OFFSITE_RCLONE_REMOTE=remote:company-backups/open-accounting/prod",
		"RCLONE_CONFIG=/etc/open-accounting/rclone.conf",
	} {
		if !strings.Contains(env, snippet) {
			t.Fatalf("generated env template missing %q:\n%s", snippet, env)
		}
	}
	if strings.Contains(env, "AWS_SECRET_ACCESS_KEY") {
		t.Fatalf("rclone env template should not include S3 credentials:\n%s", env)
	}

	installHelperPath := filepath.Join(outputDir, "open-accounting-backup-install.sh")
	installHelper, err := os.ReadFile(installHelperPath)
	if err != nil {
		t.Fatalf("read generated install helper: %v", err)
	}
	install := string(installHelper)
	for _, snippet := range []string{
		`SYSTEMD_UNIT_DIR="${1:-/usr/local/lib/systemd/system}"`,
		`install -m 0600 "$SOURCE_DIR/open-accounting-backup.env.example" "$ENV_FILE"`,
		`"$SOURCE_DIR/open-accounting-backup-preflight.sh" "$ENV_FILE"`,
		"systemctl daemon-reload",
		"open-accounting-restore-drill.timer",
	} {
		if !strings.Contains(install, snippet) {
			t.Fatalf("generated install helper missing %q:\n%s", snippet, install)
		}
	}
	info, err := os.Stat(installHelperPath)
	if err != nil {
		t.Fatalf("stat generated install helper: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("generated install helper is not executable: %s", info.Mode().Perm())
	}

	preflightHelperPath := filepath.Join(outputDir, "open-accounting-backup-preflight.sh")
	preflightHelper, err := os.ReadFile(preflightHelperPath)
	if err != nil {
		t.Fatalf("read generated preflight helper: %v", err)
	}
	preflight := string(preflightHelper)
	for _, snippet := range []string{
		"Backup operations preflight passed",
		"db-backup-offsite-sync.sh\" --rclone-remote",
		"db-restore-drill.sh",
		"--preflight",
	} {
		if !strings.Contains(preflight, snippet) {
			t.Fatalf("generated preflight helper missing %q:\n%s", snippet, preflight)
		}
	}
	info, err = os.Stat(preflightHelperPath)
	if err != nil {
		t.Fatalf("stat generated preflight helper: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("generated preflight helper is not executable: %s", info.Mode().Perm())
	}
}

func TestBackupSystemdPreflightRejectsGeneratedPlaceholderEnv(t *testing.T) {
	outputDir := t.TempDir()
	envFile := filepath.Join(t.TempDir(), "backup.env")
	scriptsDir := absoluteScriptsDir(t)

	cmd := exec.Command(
		"../scripts/db-backup-systemd-schedule.sh",
		"--output-dir", outputDir,
		"--scripts-dir", scriptsDir,
		"--env-file", envFile,
		"--offsite-provider", "s3",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate backup systemd schedule: %v\n%s", err, output)
	}

	envTemplate, err := os.ReadFile(filepath.Join(outputDir, "open-accounting-backup.env.example"))
	if err != nil {
		t.Fatalf("read generated env template: %v", err)
	}
	if err := os.WriteFile(envFile, envTemplate, 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	preflight := exec.Command(filepath.Join(outputDir, "open-accounting-backup-preflight.sh"), envFile)
	preflight.Env = cleanScriptEnv(t)
	output, err = preflight.CombinedOutput()
	if err == nil {
		t.Fatalf("placeholder preflight unexpectedly passed:\n%s", output)
	}
	if !strings.Contains(string(output), "placeholder") {
		t.Fatalf("placeholder preflight output should mention placeholder:\n%s", output)
	}
}

func TestBackupSystemdPreflightAcceptsNonSecretHostConfig(t *testing.T) {
	outputDir := t.TempDir()
	hostDir := t.TempDir()
	backupDir := filepath.Join(hostDir, "backups")
	statusDir := filepath.Join(hostDir, "metrics")
	envFile := filepath.Join(hostDir, "backup.env")
	backupFile := filepath.Join(backupDir, "openaccounting_latest.dump")
	rcloneConfig := filepath.Join(hostDir, "rclone.conf")

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("create status dir: %v", err)
	}
	if err := os.WriteFile(backupFile, []byte("scratch backup for offline preflight\n"), 0o600); err != nil {
		t.Fatalf("write backup file: %v", err)
	}
	if err := os.WriteFile(rcloneConfig, []byte("[preflight]\ntype = s3\nprovider = Other\n"), 0o600); err != nil {
		t.Fatalf("write rclone config: %v", err)
	}
	env := strings.Join([]string{
		"DATABASE_URL=postgres://backup_preflight:local_only@db.internal:5432/openaccounting?sslmode=require",
		"RESTORE_DATABASE_URL=postgres://restore_preflight:local_only@localhost:5432/openaccounting_restore_drill?sslmode=disable",
		"RESTORE_DRILL_BACKUP_FILE=" + backupFile,
		"BACKUP_OFFSITE_RCLONE_REMOTE=preflight:open-accounting/prod",
		"RCLONE_CONFIG=" + rcloneConfig,
		"",
	}, "\n")
	if err := os.WriteFile(envFile, []byte(env), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cmd := exec.Command(
		"../scripts/db-backup-systemd-schedule.sh",
		"--output-dir", outputDir,
		"--scripts-dir", absoluteScriptsDir(t),
		"--backup-dir", backupDir,
		"--status-dir", statusDir,
		"--env-file", envFile,
		"--offsite-provider", "rclone",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate backup systemd schedule: %v\n%s", err, output)
	}

	preflight := exec.Command(filepath.Join(outputDir, "open-accounting-backup-preflight.sh"), envFile)
	preflight.Env = cleanScriptEnv(t)
	output, err = preflight.CombinedOutput()
	if err != nil {
		t.Fatalf("preflight non-secret host config: %v\n%s", err, output)
	}
	text := string(output)
	for _, snippet := range []string{
		"Offsite backup preflight passed",
		"Restore drill preflight passed",
		"Backup operations preflight passed",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("preflight output missing %q:\n%s", snippet, text)
		}
	}
}

func TestBackupOffsiteDryRunRejectsPlaceholderDestinationEnv(t *testing.T) {
	backupFile := filepath.Join(t.TempDir(), "openaccounting_20260615T000000Z.dump")
	if err := os.WriteFile(backupFile, []byte("scratch backup\n"), 0o600); err != nil {
		t.Fatalf("write backup file: %v", err)
	}

	cmd := exec.Command(
		"../scripts/db-backup-offsite-sync.sh",
		"--backup", backupFile,
		"--s3-uri", "s3://company-backups/open-accounting/prod",
		"--dry-run",
	)
	cmd.Env = append(cleanScriptEnv(t),
		"AWS_REGION=eu-north-1",
		"AWS_ACCESS_KEY_ID=replace-me",
		"AWS_SECRET_ACCESS_KEY=replace-me",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("placeholder offsite dry-run unexpectedly passed:\n%s", output)
	}
	if !strings.Contains(string(output), "placeholder") {
		t.Fatalf("placeholder offsite output should mention placeholder:\n%s", output)
	}
}

func TestRestoreDrillDryRunRejectsPlaceholderDatabaseEnv(t *testing.T) {
	backupFile := filepath.Join(t.TempDir(), "openaccounting_20260615T000000Z.dump")
	if err := os.WriteFile(backupFile, []byte("scratch backup\n"), 0o600); err != nil {
		t.Fatalf("write backup file: %v", err)
	}

	cmd := exec.Command(
		"../scripts/db-restore-drill.sh",
		"--backup", backupFile,
		"--restore-url", "postgres://user:pass@localhost:5432/openaccounting_restore_drill?sslmode=disable",
		"--dry-run",
	)
	cmd.Env = append(cleanScriptEnv(t), "DATABASE_URL=postgres://user:pass@db.example.com:5432/openaccounting?sslmode=require")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("placeholder restore dry-run unexpectedly passed:\n%s", output)
	}
	if !strings.Contains(string(output), "placeholder") {
		t.Fatalf("placeholder restore output should mention placeholder:\n%s", output)
	}
}

func absoluteScriptsDir(t *testing.T) string {
	t.Helper()

	scriptsDir, err := filepath.Abs("../scripts")
	if err != nil {
		t.Fatalf("resolve scripts dir: %v", err)
	}
	return scriptsDir
}

func cleanScriptEnv(t *testing.T) []string {
	t.Helper()

	env := []string{
		"HOME=" + t.TempDir(),
		"PATH=" + os.Getenv("PATH"),
	}
	if tmpDir := os.Getenv("TMPDIR"); tmpDir != "" {
		env = append(env, "TMPDIR="+tmpDir)
	}
	return env
}

func TestRunAffectedTestsScriptSelectsBackendDependants(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "internal/cutover/validate.go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected backend tests: %v\n%s", err, output)
	}

	text := string(output)
	for _, snippet := range []string{
		"go test -count=1 -race",
		"./internal/cutover",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("affected backend output missing %q:\n%s", snippet, text)
		}
	}
}

func TestRunAffectedTestsScriptSelectsDocsForScriptChanges(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "scripts/run-affected-tests.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected script tests: %v\n%s", err, output)
	}

	expected := "go test -timeout=3m ./docs -count=1"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected script affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsMigrationTests(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "migrations/053_quote_order_email_template_types.up.sql")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected migration tests: %v\n%s", err, output)
	}

	expected := "go test -timeout=5m -tags=integration ./cmd/migrate -count=1"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected migration affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsDocsForIntegrationWeightsChanges(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--changed-file", "scripts/integration-package-weights.tsv")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected integration package weights tests: %v\n%s", err, output)
	}

	expected := "go test -timeout=3m ./docs -count=1"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected integration weights affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}

func TestRunAffectedTestsScriptSelectsFrontendChangedTests(t *testing.T) {
	cmd := exec.Command("../scripts/run-affected-tests.sh", "--list", "--base", "HEAD", "--changed-file", "frontend/src/lib/api.ts")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list affected frontend tests: %v\n%s", err, output)
	}

	expected := "cd frontend && bun run paraglide && bun run test:prepared -- --changed HEAD"
	if strings.TrimSpace(string(output)) != expected {
		t.Fatalf("unexpected frontend affected tests:\nwant: %s\ngot:\n%s", expected, output)
	}
}
