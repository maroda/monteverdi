package monteverdi_test

import (
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
	Mt "github.com/maroda/monteverdi/types"
)

func TestQNet_PollMulti_JSONKeyPlugin(t *testing.T) {
	testMetric := "bitcoin.usd"
	remotebody := `{"bitcoin":{"usd":111580},"ethereum":{"usd":3955.02}}`
	mockWWW := makeMockWebServBody(0*time.Millisecond, remotebody)
	ep := &Ms.Endpoint{
		ID:     "test",
		URL:    mockWWW.URL,
		Delim:  "",
		Metric: map[int]string{1: testMetric},
		Mdata:  make(map[string]int64),
		Maxval: map[string]int64{testMetric: 111600},
		Accent: make(map[string]*Mt.Accent),
		Layer: map[string]*Mt.Timeseries{
			testMetric: {
				Runes:   make([]rune, 80),
				MaxSize: 80,
				Current: 0,
			},
		},
		Sequence: make(map[string]*Ms.IctusSequence),
		Pulses: &Ms.TemporalGrouper{
			WindowSize: 60 * time.Second,
			Buffer:     make([]Mt.PulseEvent, 0),
			Groups:     make([]*Mt.PulseTree, 0),
		},
		Hysteresis:   make(map[string]*Ms.CycBuffer),
		Transformers: map[string]Mp.MetricTransformer{testMetric: &Mp.JSONKeyPlugin{testMetric}},
	}
	eps := Ms.Endpoints{ep}
	qn := Ms.NewQNet(eps)

	t.Run("Correctly fetches value from JSON", func(t *testing.T) {
		qn.PollMulti()
		got := qn.Network[0].Mdata["bitcoin.usd"]
		want := int64(111580)
		assertInt64(t, got, want)
	})

	// Close mock server for error tests
	mockWWW.Close()

	t.Run("Continues when fetch fails", func(t *testing.T) {
		qn.PollMulti()
		// This passes if we get this far without panic
	})
}

func TestNewEndpointsFromConfig_JSONKeyPlugin(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
  "id": "COINGECKO",
  "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
  "delim": "",
  "metrics": {
    "bitcoin.usd": {
      "type": "gauge",
      "transformer": "json_key",
      "max": 111000
    }
  }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	eps := Ms.NewEndpointsFromConfig(loadConfig)

	t.Run("Transformer is returned", func(t *testing.T) {
		ep := (*eps)[0]
		if ep.Transformers["bitcoin.usd"] == nil {
			t.Error("No Transformer returned for metric")
		}
	})
}
