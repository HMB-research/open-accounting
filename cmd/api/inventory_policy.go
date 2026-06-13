package main

import (
	"strings"

	"github.com/HMB-research/open-accounting/internal/tenant"
)

func tenantInventoryValuationMethod(tenantRecord *tenant.Tenant, requested string) string {
	if method := strings.TrimSpace(requested); method != "" {
		return method
	}
	if tenantRecord == nil {
		return ""
	}
	return tenant.EffectiveInventoryValuationMethod(tenantRecord.Settings.InventoryValuationMethod)
}

func tenantInventoryIssueCostingMethod(tenantRecord *tenant.Tenant, requested string) string {
	if method := strings.TrimSpace(requested); method != "" {
		return method
	}
	if tenantRecord == nil {
		return ""
	}
	return tenant.EffectiveInventoryIssueCostingMethod(tenantRecord.Settings.InventoryIssueCostingMethod)
}
