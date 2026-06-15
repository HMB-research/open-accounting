package tenant

import (
	"fmt"
	"strings"
)

const (
	// InventoryValuationMethodStandardCost values stock using each product purchase price.
	InventoryValuationMethodStandardCost = "STANDARD_COST"
	// InventoryValuationMethodWeightedAverage values stock using weighted average inbound movement cost.
	InventoryValuationMethodWeightedAverage = "WEIGHTED_AVERAGE"
	// InventoryValuationMethodFIFO values ending stock from the newest remaining inbound layers.
	InventoryValuationMethodFIFO = "FIFO"
)

const (
	// InventoryIssueCostingMethodLot uses the consumed lot/serial/expiry layer costs.
	InventoryIssueCostingMethodLot = "LOT"
	// InventoryIssueCostingMethodStandardCost uses each product purchase price for issue cost.
	InventoryIssueCostingMethodStandardCost = InventoryValuationMethodStandardCost
	// InventoryIssueCostingMethodWeightedAverage uses weighted average inbound movement cost for issue cost.
	InventoryIssueCostingMethodWeightedAverage = InventoryValuationMethodWeightedAverage
)

// NormalizeInventoryValuationMethod returns the canonical tenant inventory valuation method.
func NormalizeInventoryValuationMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(method), "-", "_"))
	switch normalized {
	case "", "STANDARD", InventoryValuationMethodStandardCost:
		return InventoryValuationMethodStandardCost, nil
	case "WEIGHTED", "AVERAGE_COST", InventoryValuationMethodWeightedAverage:
		return InventoryValuationMethodWeightedAverage, nil
	case InventoryValuationMethodFIFO, "FIFO_LAYERED":
		return InventoryValuationMethodFIFO, nil
	default:
		return "", fmt.Errorf("invalid inventory valuation method: %s", method)
	}
}

// NormalizeInventoryIssueCostingMethod returns the canonical tenant inventory issue costing method.
func NormalizeInventoryIssueCostingMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(method), "-", "_"))
	switch normalized {
	case "", InventoryIssueCostingMethodLot, "LOT_COST", "SPECIFIC_LOT", "ACTUAL_LOT", "FIFO", "FEFO":
		return InventoryIssueCostingMethodLot, nil
	case "STANDARD", InventoryIssueCostingMethodStandardCost:
		return InventoryIssueCostingMethodStandardCost, nil
	case "WEIGHTED", "AVERAGE", "AVERAGE_COST", InventoryIssueCostingMethodWeightedAverage:
		return InventoryIssueCostingMethodWeightedAverage, nil
	default:
		return "", fmt.Errorf("invalid inventory issue costing method: %s", method)
	}
}

// EffectiveInventoryValuationMethod returns a usable valuation method even for older settings rows.
func EffectiveInventoryValuationMethod(method string) string {
	normalized, err := NormalizeInventoryValuationMethod(method)
	if err != nil {
		return InventoryValuationMethodStandardCost
	}
	return normalized
}

// EffectiveInventoryIssueCostingMethod returns a usable issue costing method even for older settings rows.
func EffectiveInventoryIssueCostingMethod(method string) string {
	normalized, err := NormalizeInventoryIssueCostingMethod(method)
	if err != nil {
		return InventoryIssueCostingMethodLot
	}
	return normalized
}

func normalizeInventoryPolicySettings(settings *TenantSettings) error {
	if settings == nil {
		return nil
	}

	issueMethod, err := NormalizeInventoryIssueCostingMethod(settings.InventoryIssueCostingMethod)
	if err != nil {
		return err
	}
	valuationMethod, err := NormalizeInventoryValuationMethod(settings.InventoryValuationMethod)
	if err != nil {
		return err
	}

	settings.InventoryIssueCostingMethod = issueMethod
	settings.InventoryValuationMethod = valuationMethod
	return nil
}
