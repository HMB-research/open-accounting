package demo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSeedForUser(t *testing.T) {
	tests := []struct {
		name        string
		userNum     int
		template    string
		contains    []string
		notContains []string
	}{
		{
			name:     "user 1 replacements",
			userNum:  1,
			template: "demo@example.com 'acme' tenant_acme a0000000-0000-0000-0000-000000000001",
			contains: []string{
				"demo1@example.com",
				"'demo1'",
				"tenant_demo1",
				"a0000000-0000-0000-0001-000000000001",
			},
			notContains: []string{
				"demo@example.com",
				"'acme'",
				"tenant_acme",
				"a0000000-0000-0000-0000-000000000001",
			},
		},
		{
			name:     "user 2 replacements",
			userNum:  2,
			template: "demo@example.com 'acme' tenant_acme b0000000-0000-0000-0000-000000000001",
			contains: []string{
				"demo2@example.com",
				"'demo2'",
				"tenant_demo2",
				"b0000000-0000-0000-0002-000000000001",
			},
		},
		{
			name:     "user 3 replacements",
			userNum:  3,
			template: "Acme Corporation @acme.ee info@acme.example.com",
			contains: []string{
				"Demo Company 3",
				"@demo3.example.com",
				"info@demo3.example.com",
			},
		},
		{
			name:     "uuid replacements for accounts",
			userNum:  1,
			template: "c0000000-0000-0000-0000-000000000001",
			contains: []string{"c1000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for contacts",
			userNum:  2,
			template: "d0000000-0000-0000-0000-000000000001",
			contains: []string{"d2000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for invoices",
			userNum:  1,
			template: "e0000000-0000-0000-0000-000000000001",
			contains: []string{"e1000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "uuid replacements for payments",
			userNum:  2,
			template: "f0000000-0000-0000-0000-000000000001",
			contains: []string{"f2000000-0000-0000-0000-000000000001"},
		},
		{
			name:     "employee id replacements",
			userNum:  1,
			template: "72000000-0000-0000-0000-000000000001",
			contains: []string{
				"72100000-0000-0000-0000-000000000001",
			},
		},
		{
			name:     "invoice number replacements",
			userNum:  1,
			template: "INV-2024-001 INV-2025-001 PAY-2024-001 JE-2024-001",
			contains: []string{
				"INV1-2024-001",
				"INV1-2025-001",
				"PAY1-2024-001",
				"JE1-2024-001",
			},
		},
		{
			name:     "fiscal year and bank account ids",
			userNum:  2,
			template: "80000000-0000-0000-0000-000000000001 90000000-0000-0000-0000-000000000001",
			contains: []string{
				"82000000-0000-0000-0000-000000000001",
				"92000000-0000-0000-0000-000000000001",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSeedForUser(tt.template, tt.userNum)

			for _, s := range tt.contains {
				require.Contains(t, result, s, "expected result to contain %q", s)
			}

			for _, s := range tt.notContains {
				require.NotContains(t, result, s, "expected result to NOT contain %q", s)
			}
		})
	}
}

func TestSeedTemplate(t *testing.T) {
	template := SeedTemplate()

	require.Contains(t, template, "demo@example.com", "template should contain demo email")
	require.Contains(t, template, "'acme'", "template should contain acme slug")
	require.Contains(t, template, "tenant_acme", "template should contain tenant_acme schema")
	require.Contains(t, template, "a0000000-0000-0000-0000-000000000001", "template should contain user UUID")
	require.Contains(t, template, "b0000000-0000-0000-0000-000000000001", "template should contain tenant UUID")
	require.Contains(t, template, "INSERT INTO users", "template should contain user insert")
	require.Contains(t, template, "INSERT INTO tenants", "template should contain tenant insert")
	require.Contains(t, template, "create_tenant_schema", "template should contain schema creation function")
}

func TestSeedSQLForUsers(t *testing.T) {
	sql := SeedSQLForUsers([]int{1, 2})

	require.Contains(t, sql, "demo1@example.com")
	require.Contains(t, sql, "demo2@example.com")
	require.Contains(t, sql, "tenant_demo1")
	require.Contains(t, sql, "tenant_demo2")
	require.NotContains(t, sql, "demo@example.com")
	require.NotContains(t, sql, "tenant_acme")
}
