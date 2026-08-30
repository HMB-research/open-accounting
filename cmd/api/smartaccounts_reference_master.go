package main

import (
	"context"
	"fmt"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/shopspring/decimal"
)

// smartAccountsReferenceWriter composes existing native OA services. It has no
// path to a journal, invoice, or payment writer. Its Ensure operations only
// accept a retry after proving the deterministic target has the exact
// projected reference fields; they never turn a code/name collision into a
// successful import.
type smartAccountsReferenceWriter struct {
	accounts  *accounting.Service
	contacts  *contacts.Service
	inventory *inventory.Service
}

func (w smartAccountsReferenceWriter) EnsureAccount(ctx context.Context, schema, tenant string, req *accounting.CreateAccountRequest) error {
	if _, err := w.accounts.CreateAccount(ctx, schema, tenant, req); err == nil {
		return nil
	} else if existing, getErr := w.accounts.GetAccount(ctx, schema, tenant, req.ID); getErr == nil && existing.Code == req.Code && existing.Name == req.Name && existing.AccountType == req.AccountType {
		return nil
	} else {
		return fmt.Errorf("reference account target was not created with the expected deterministic identity: %w", err)
	}
}
func (w smartAccountsReferenceWriter) EnsureContact(ctx context.Context, tenant, schema string, req *contacts.CreateContactRequest) error {
	if _, err := w.contacts.Create(ctx, tenant, schema, req); err == nil {
		return nil
	} else if existing, getErr := w.contacts.GetByID(ctx, tenant, schema, req.ID); getErr == nil && sameReferenceContact(existing, req) {
		return nil
	} else {
		return fmt.Errorf("reference contact target was not created with the expected deterministic identity: %w", err)
	}
}
func (w smartAccountsReferenceWriter) EnsureProduct(ctx context.Context, tenant, schema string, req *inventory.CreateProductRequest) error {
	if _, err := w.inventory.CreateProduct(ctx, tenant, schema, req); err == nil {
		return nil
	} else if existing, getErr := w.inventory.GetProductByID(ctx, tenant, schema, req.ID); getErr == nil && sameReferenceProduct(existing, req) {
		return nil
	} else {
		return fmt.Errorf("reference product target was not created with the expected deterministic identity: %w", err)
	}
}

func sameReferenceContact(existing *contacts.Contact, req *contacts.CreateContactRequest) bool {
	return existing != nil && existing.Code == req.Code && existing.Name == req.Name && existing.ContactType == req.ContactType && existing.RegCode == req.RegCode && existing.VATNumber == req.VATNumber && existing.Email == req.Email && existing.Phone == req.Phone && existing.AddressLine1 == req.AddressLine1 && existing.AddressLine2 == req.AddressLine2 && existing.City == req.City && existing.PostalCode == req.PostalCode && existing.CountryCode == req.CountryCode
}

func sameReferenceProduct(existing *inventory.Product, req *inventory.CreateProductRequest) bool {
	if existing == nil || existing.Code != req.Code || existing.Name != req.Name || existing.Description != req.Description || existing.ProductType != inventory.ProductType(req.ProductType) || existing.Unit != req.Unit || existing.TrackInventory != req.TrackInventory {
		return false
	}
	salesPrice, salesErr := decimal.NewFromString(req.SalesPrice)
	vatRate, vatErr := decimal.NewFromString(req.VATRate)
	return salesErr == nil && vatErr == nil && existing.SalesPrice.Equal(salesPrice) && existing.VATRate.Equal(vatRate)
}

type smartAccountsReferenceCatalog struct {
	accounts  *accounting.Service
	contacts  *contacts.Service
	inventory *inventory.Service
}

func (c smartAccountsReferenceCatalog) ListAccounts(ctx context.Context, schema, tenant string) ([]accounting.Account, error) {
	return c.accounts.ListAccounts(ctx, schema, tenant, false)
}
func (c smartAccountsReferenceCatalog) ListContacts(ctx context.Context, schema, tenant string) ([]contacts.Contact, error) {
	return c.contacts.List(ctx, tenant, schema, nil)
}
func (c smartAccountsReferenceCatalog) ListProducts(ctx context.Context, schema, tenant string) ([]inventory.Product, error) {
	return c.inventory.ListProducts(ctx, tenant, schema, nil)
}
