package demo

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed seed_template.sql
var seedTemplateSQL string

// SeedSQLForUsers returns the SQL to seed specific demo users.
func SeedSQLForUsers(userNums []int) string {
	var sql strings.Builder
	template := SeedTemplate()

	for _, userNum := range userNums {
		sql.WriteString(GenerateSeedForUser(template, userNum))
	}

	return sql.String()
}

// GenerateSeedForUser adapts the template for a specific demo user number.
func GenerateSeedForUser(template string, userNum int) string {
	n := fmt.Sprintf("%d", userNum)

	result := strings.ReplaceAll(template, "demo@example.com", fmt.Sprintf("demo%s@example.com", n))
	result = strings.ReplaceAll(result, "'acme'", fmt.Sprintf("'demo%s'", n))
	result = strings.ReplaceAll(result, "tenant_acme", fmt.Sprintf("tenant_demo%s", n))
	result = strings.ReplaceAll(result, "Acme Corporation", fmt.Sprintf("Demo Company %s", n))
	result = strings.ReplaceAll(result, "@acme.ee", fmt.Sprintf("@demo%s.example.com", n))
	result = strings.ReplaceAll(result, "info@acme.example.com", fmt.Sprintf("info@demo%s.example.com", n))

	result = strings.ReplaceAll(result, "a0000000-0000-0000-0000-", fmt.Sprintf("a0000000-0000-0000-000%s-", n))
	result = strings.ReplaceAll(result, "b0000000-0000-0000-0000-", fmt.Sprintf("b0000000-0000-0000-000%s-", n))
	result = strings.ReplaceAll(result, "c0000000-0000-0000-", fmt.Sprintf("c%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "d0000000-0000-0000-", fmt.Sprintf("d%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "e0000000-0000-0000-", fmt.Sprintf("e%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "f0000000-0000-0000-", fmt.Sprintf("f%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "70000000-0000-0000-", fmt.Sprintf("7%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "71000000-0000-0000-", fmt.Sprintf("71%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "72000000-0000-0000-", fmt.Sprintf("72%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "73000000-0000-0000-", fmt.Sprintf("73%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "74000000-0000-0000-", fmt.Sprintf("74%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "75000000-0000-0000-", fmt.Sprintf("75%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "76000000-0000-0000-", fmt.Sprintf("76%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "77000000-0000-0000-", fmt.Sprintf("77%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "78000000-0000-0000-", fmt.Sprintf("78%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "79000000-0000-0000-", fmt.Sprintf("79%s00000-0000-0000-", n))
	result = strings.ReplaceAll(result, "80000000-0000-0000-", fmt.Sprintf("8%s000000-0000-0000-", n))
	result = strings.ReplaceAll(result, "90000000-0000-0000-", fmt.Sprintf("9%s000000-0000-0000-", n))

	result = strings.ReplaceAll(result, "INV-2024-", fmt.Sprintf("INV%s-2024-", n))
	result = strings.ReplaceAll(result, "INV-2025-", fmt.Sprintf("INV%s-2025-", n))
	result = strings.ReplaceAll(result, "PAY-2024-", fmt.Sprintf("PAY%s-2024-", n))
	result = strings.ReplaceAll(result, "JE-2024-", fmt.Sprintf("JE%s-2024-", n))

	return result
}

// SeedTemplate returns the base SQL template for one demo user.
func SeedTemplate() string {
	return seedTemplateSQL
}
