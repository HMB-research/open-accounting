package reports

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCashFlowMappingWave8SettingsParseEdges(t *testing.T) {
	t.Run("invalid mapping value is wrapped", func(t *testing.T) {
		mapping, err := cashFlowMappingFromSettings(json.RawMessage(`{"cash_flow_mapping":"not-an-object"}`))

		assert.Empty(t, mapping)
		require.ErrorContains(t, err, "parse cash flow mapping")
	})

	t.Run("settings update preserves unrelated keys", func(t *testing.T) {
		updated, err := settingsWithCashFlowMapping(
			json.RawMessage(`{"locale":"et","cash_flow_mapping":null}`),
			CashFlowMappingOverrides{OperatingAccountCodes: []string{"1000", "1010"}},
		)

		require.NoError(t, err)
		var settings map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(updated, &settings))
		assert.JSONEq(t, `"et"`, string(settings["locale"]))
		assert.JSONEq(t, `{"operating_account_codes":["1000","1010"]}`, string(settings["cash_flow_mapping"]))
	})
}
