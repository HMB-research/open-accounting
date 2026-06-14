package contactrefs

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

// Reference is a named contact reference from an import row.
type Reference struct {
	Field string
	Value string
}

// SupplierLookup resolves supplier references from tenant contacts.
type SupplierLookup struct {
	idsByReference      map[string]string
	duplicateReferences map[string]struct{}
}

// NewSupplierLookup builds a lookup across common supplier identity columns.
func NewSupplierLookup(contactRows []contacts.Contact) SupplierLookup {
	lookup := SupplierLookup{
		idsByReference:      make(map[string]string, len(contactRows)),
		duplicateReferences: make(map[string]struct{}),
	}
	for _, contact := range contactRows {
		lookup.add("supplier_code", contact.Code, contact.ID)
		lookup.add("supplier_reg_code", contact.RegCode, contact.ID)
		lookup.add("supplier_vat_number", contact.VATNumber, contact.ID)
		lookup.add("supplier_email", contact.Email, contact.ID)
		lookup.add("supplier_name", contact.Name, contact.ID)
	}
	return lookup
}

// ResolveID returns an explicit supplier UUID, or resolves the first supplied non-empty reference.
func (l SupplierLookup) ResolveID(supplierID string, refs ...Reference) (*string, error) {
	if id := strings.TrimSpace(supplierID); id != "" {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("supplier_id must be a valid UUID")
		}
		canonicalID := parsedID.String()
		return &canonicalID, nil
	}

	for _, ref := range refs {
		value := strings.TrimSpace(ref.Value)
		if value == "" {
			continue
		}
		key := referenceKey(ref.Field, value)
		if _, duplicate := l.duplicateReferences[key]; duplicate {
			return nil, fmt.Errorf("%s %q matched multiple contacts", ref.Field, value)
		}
		id, ok := l.idsByReference[key]
		if !ok {
			return nil, fmt.Errorf("%s %q was not found", ref.Field, value)
		}
		return &id, nil
	}
	return nil, nil
}

func (l SupplierLookup) add(field, value, id string) {
	key := referenceKey(field, value)
	if key == "" || strings.TrimSpace(id) == "" {
		return
	}
	if existingID, ok := l.idsByReference[key]; ok && existingID != id {
		l.duplicateReferences[key] = struct{}{}
		return
	}
	l.idsByReference[key] = id
}

func referenceKey(field, value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if strings.TrimSpace(field) == "" || normalized == "" {
		return ""
	}
	return strings.TrimSpace(field) + "\x00" + normalized
}
