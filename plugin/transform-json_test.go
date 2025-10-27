package plugin_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
)

func TestNewJSONTransformer(t *testing.T) {
	t.Run("Returns JSON transformer", func(t *testing.T) {
		key := "NETWORK"

		newJSON := Mp.NewJSONTransformer(key)
		assertStringContains(t, newJSON.MetricKey, key)
	})
}

func TestJSONKeyPlugin(t *testing.T) {
	t.Run("HysteresisReq returns the correct value", func(t *testing.T) {
		// Hysteresis in JSONKey is meaningless so set it to -1
		plugin := Mp.JSONKeyPlugin{}
		want := -1
		got := plugin.HysteresisReq()
		assertInt(t, got, want)
	})

	t.Run("Type returns the correct value", func(t *testing.T) {
		plugin := Mp.JSONKeyPlugin{}
		want := "json_key"
		got := plugin.Type()
		assertStringContains(t, got, want)
	})

	now := time.Now()
	plugin := Mp.JSONKeyPlugin{
		MetricKey: "bitcoin.usd",
	}

	t.Run("Metric returns the correct value", func(t *testing.T) {
		// This interface expects that /metric/ is filled with JSON
		metric := `{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}`
		want := int64(111580)

		got, err := plugin.Transform(metric, 0, []int64{0}, now)
		assertError(t, err, nil)
		assertInt64(t, got, want)
	})

	// Check for errors
	errTests := []struct {
		name   string
		metric string
		key    string
	}{
		{
			name:   "Errors on JSON array (not yet implemented)",
			metric: `[{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}]`,
			key:    "bitcoin.usd",
		},
		{
			name:   "Errors on unmarshalling JSON",
			metric: `^M`,
			key:    "bitcoin.usd",
		},
		{
			name:   "Errors on missing key",
			metric: `{"rubicoin":{"usd":111580},"ethereum":{"usd":3955.02}}`,
			key:    "bitcoin.usd",
		},
		{
			name:   "Errors on non-matching JSON structure",
			metric: `{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}`,
			key:    "bitcoin.usd.price",
		},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.MetricKey = tt.key
			_, err := plugin.Transform(tt.metric, 0, []int64{0}, now)
			assertGotError(t, err)
		})
	}

	t.Run("Errors on non-number", func(t *testing.T) {
		plugin.MetricKey = "bitcoin.usd"
		metric := `{"bitcoin":{"usd":"price"},"ethereum":{"usd":3955.02}}`
		_, err := plugin.Transform(metric, 0, []int64{0}, now)
		assertGotError(t, err)
	})

	// Check for number conversions
	testNum := []struct {
		name   string
		metric string
		key    string
		want   int64
	}{
		{
			name:   "float64",
			metric: `{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}`,
			key:    "ethereum.usd",
			want:   int64(3955),
		},
		{
			name:   "int",
			metric: `{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}`,
			key:    "bitcoin.usd",
			want:   int64(111580),
		},
	}

	for _, tt := range testNum {
		t.Run(tt.name, func(t *testing.T) {
			plugin.MetricKey = tt.key
			got, err := plugin.Transform(tt.metric, 0, []int64{0}, now)
			assertError(t, err, nil)
			assertInt64(t, got, tt.want)
		})
	}
}

func TestJSONKeyPlugin_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	t.Run("INTEGRATION: coingecko response in real-time", func(t *testing.T) {
		now := time.Now()
		webclient := http.Client{}
		url := "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd"
		resp, err := webclient.Get(url)
		assertError(t, err, nil)
		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		// Get the value here in the test first
		var response CryptoResponse
		err = json.Unmarshal(body, &response)
		assertError(t, err, nil)

		// Now create a new interface with the value we want
		plugin := Mp.JSONKeyPlugin{
			MetricKey: "ethereum.usd",
		}

		// Plug that into the Transform method and the values should match
		got, err := plugin.Transform(string(body), 0, []int64{0}, now)
		assertError(t, err, nil)
		assertInt64(t, got, int64(response.Ethereum.USD))
	})
}

// Helpers //

type CryptoResponse struct {
	Bitcoin  Currency `json:"bitcoin"`
	Ethereum Currency `json:"ethereum"`
}

type Currency struct {
	USD float64 `json:"usd"`
}
