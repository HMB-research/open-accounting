package registry

import (
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers"
	genericmapper "github.com/HMB-research/open-accounting/internal/banking/mappers/generic"
	lhvmapper "github.com/HMB-research/open-accounting/internal/banking/mappers/lhv"
)

// ParseTransactions normalizes bank statement content using the requested mapper.
func ParseTransactions(content, format string) ([]banking.CSVTransactionRow, error) {
	switch mappers.Format(strings.ToLower(strings.TrimSpace(format))) {
	case "", mappers.FormatAuto:
		if lhvmapper.DetectTransactions(content) {
			return lhvmapper.ParseTransactions(content)
		}
		return genericmapper.ParseTransactions(content)
	case mappers.FormatGeneric:
		return genericmapper.ParseTransactions(content)
	case mappers.FormatLHV:
		return lhvmapper.ParseCSVTransactions(content)
	case mappers.FormatLHVCAMT:
		return lhvmapper.ParseCAMTTransactions(content)
	default:
		return nil, fmt.Errorf("unsupported bank transaction import format %q", format)
	}
}
