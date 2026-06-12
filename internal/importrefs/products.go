package importrefs

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/HMB-research/open-accounting/internal/inventory"
)

// ProductLookup resolves optional migration product references to product IDs.
type ProductLookup struct {
	idsByCode      map[string]string
	duplicateCodes map[string]struct{}
}

// NewProductLookup builds a product-code lookup from tenant-scoped products.
func NewProductLookup(products []inventory.Product) ProductLookup {
	lookup := ProductLookup{
		idsByCode:      make(map[string]string, len(products)),
		duplicateCodes: make(map[string]struct{}),
	}
	for _, product := range products {
		code := normalizeProductCode(product.Code)
		if code == "" {
			continue
		}
		if existingID, ok := lookup.idsByCode[code]; ok && existingID != product.ID {
			lookup.duplicateCodes[code] = struct{}{}
			continue
		}
		lookup.idsByCode[code] = product.ID
	}
	return lookup
}

// ResolveID returns an explicit product ID, or resolves productCode when productID is empty.
func (l ProductLookup) ResolveID(productID, productCode string) (*string, error) {
	if id := strings.TrimSpace(productID); id != "" {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("product_id must be a valid UUID")
		}
		canonicalID := parsedID.String()
		return &canonicalID, nil
	}

	code := strings.TrimSpace(productCode)
	if code == "" {
		return nil, nil
	}

	normalized := normalizeProductCode(code)
	if _, duplicate := l.duplicateCodes[normalized]; duplicate {
		return nil, fmt.Errorf("product_code %q matched multiple products", code)
	}

	id, ok := l.idsByCode[normalized]
	if !ok {
		return nil, fmt.Errorf("product_code %q was not found", code)
	}
	return &id, nil
}

func normalizeProductCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
