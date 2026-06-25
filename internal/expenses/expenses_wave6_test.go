package expenses

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/accounting"
)

func TestExpensesWave6ImportExpensesCSVEarlyErrors(t *testing.T) {
	ctx := context.Background()
	validIDsCSV := "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount\n" +
		"EXP-1,2026-06-01,Vendor,99999999-9999-4999-8999-999999999999,aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,10\n"

	tests := []struct {
		name    string
		service *Service
		req     *ImportExpensesRequest
		want    string
	}{
		{
			name:    "missing user id",
			service: NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil),
			req: &ImportExpensesRequest{
				UserID:     " ",
				CSVContent: validIDsCSV,
			},
			want: "user id is required",
		},
		{
			name:    "parse csv row error",
			service: NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil),
			req: &ImportExpensesRequest{
				UserID:     "user-1",
				CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount\n\"unterminated\n",
			},
			want: "parse csv row 2",
		},
		{
			name:    "no expense rows",
			service: NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil),
			req: &ImportExpensesRequest{
				UserID:     "user-1",
				CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount\n",
			},
			want: "no expenses found in CSV",
		},
		{
			name:    "missing accounting dependency for account codes",
			service: NewServiceWithRepository(newMemoryRepository(), nil, nil),
			req: &ImportExpensesRequest{
				UserID: "user-1",
				CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount\n" +
					"EXP-1,2026-06-01,Vendor,5500,1000,10\n",
			},
			want: "accounting service is required to resolve expense account codes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.service.ImportExpensesCSV(ctx, "tenant_schema", "tenant-1", tt.req)

			if result != nil {
				t.Fatalf("expected nil result, got %#v", result)
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestExpensesWave6ImportExpensesCSVDependencyErrors(t *testing.T) {
	ctx := context.Background()
	validIDsCSV := "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount"
	accountingErr := errors.New("account list failed")
	contactErr := errors.New("contact list failed")
	employeeErr := errors.New("employee list failed")

	t.Run("account list error", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), &expensesWave6AccountingLister{
			fakeAccountingPoster: newFakeAccountingPoster(),
			err:                  accountingErr,
		}, nil)

		result, err := service.ImportExpensesCSV(ctx, "tenant_schema", "tenant-1", &ImportExpensesRequest{
			UserID: "user-1",
			CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount\n" +
				"EXP-1,2026-06-01,Vendor,5500,1000,10\n",
		})

		if result != nil {
			t.Fatalf("expected nil result, got %#v", result)
		}
		if !errors.Is(err, accountingErr) || !strings.Contains(err.Error(), "list accounts for expense import") {
			t.Fatalf("expected wrapped account list error, got %v", err)
		}
	})

	t.Run("contact list error", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		service.contacts = &fakeExpenseContactLister{err: contactErr}

		result, err := service.ImportExpensesCSV(ctx, "tenant_schema", "tenant-1", &ImportExpensesRequest{
			UserID: "user-1",
			CSVContent: validIDsCSV + ",contact_code\n" +
				"EXP-1,2026-06-01,Vendor,99999999-9999-4999-8999-999999999999,aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,10,SUP-1\n",
		})

		if result != nil {
			t.Fatalf("expected nil result, got %#v", result)
		}
		if !errors.Is(err, contactErr) || !strings.Contains(err.Error(), "list contacts for expense import") {
			t.Fatalf("expected wrapped contact list error, got %v", err)
		}
	})

	t.Run("employee list error", func(t *testing.T) {
		service := NewServiceWithRepository(newMemoryRepository(), newFakeAccountingPoster(), nil)
		service.employees = &fakeExpenseEmployeeLister{err: employeeErr}

		result, err := service.ImportExpensesCSV(ctx, "tenant_schema", "tenant-1", &ImportExpensesRequest{
			UserID: "user-1",
			CSVContent: validIDsCSV + ",employee_id\n" +
				"EXP-1,2026-06-01,Vendor,99999999-9999-4999-8999-999999999999,aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa,10,11111111-1111-4111-8111-111111111111\n",
		})

		if result != nil {
			t.Fatalf("expected nil result, got %#v", result)
		}
		if !errors.Is(err, employeeErr) || !strings.Contains(err.Error(), "list employees for expense import") {
			t.Fatalf("expected wrapped employee list error, got %v", err)
		}
	})
}

type expensesWave6AccountingLister struct {
	*fakeAccountingPoster
	err error
}

func (f *expensesWave6AccountingLister) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.fakeAccountingPoster.ListAccounts(context.Background(), "", "", false)
}
