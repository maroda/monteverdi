package monteverdi_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	Md "github.com/maroda/monteverdi/display"
	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
)

func TestView_MetricsDataHandlerMIDI(t *testing.T) {
	t.Run("Metrics Data Endpoint", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/metrics-data", nil)
		w := httptest.NewRecorder()
		view := makeViewWithMIDI(t)
		view.MetricsDataHandler(w, r)
		assertStatus(t, w.Code, http.StatusOK)
	})
}

func TestView_PluginControlHandlerMIDI(t *testing.T) {
	view := makeViewWithMIDI(t)
	defer view.QNet.Output.Close()

	tests := []struct {
		name     string
		method   string
		target   string
		assert   int
		contains string
	}{
		{
			name:     "Plugin Control Endpoint: Type",
			method:   "POST",
			target:   "/api/plugin/type",
			assert:   http.StatusOK, // 200
			contains: "MIDI",
		},
		{
			name:     "Plugin Control Endpoint: Flush",
			method:   "POST",
			target:   "/api/plugin/flush",
			assert:   http.StatusOK, // 200
			contains: "FLUSHED",
		},
		{
			name:     "Plugin Control Endpoint: Bad Request (too few elements)",
			method:   "POST",
			target:   "/api/plugin",
			assert:   http.StatusBadRequest, // 400
			contains: "invalid",
		},
		{
			name:     "Plugin Control Endpoint: Bad Request (invalid control)",
			method:   "POST",
			target:   "/api/plugin/cornhole",
			assert:   http.StatusBadRequest, // 400
			contains: "invalid",
		},
		{
			name:     "Plugin Control Endpoint: Close",
			method:   "POST",
			target:   "/api/plugin/close",
			assert:   http.StatusOK, // 200
			contains: "CLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.target, nil)
			w := httptest.NewRecorder()
			view.PluginControlHandler(w, r)
			assertStatus(t, w.Code, tt.assert)
			assertStringContains(t, w.Body.String(), tt.contains)
		})
	}
}

// Helpers //

// ViewWithMIDI opens a MIDI driver
func makeViewWithMIDI(t *testing.T) *Md.View {
	t.Helper()
	midiOut, err := Mp.NewMIDIOutput(0)
	assertError(t, err, nil)

	// Set up data: New QNet, add Endpoint, create View.
	qn := makeNewTestQNet(t)
	qn.Output = midiOut
	qn.Network[0] = &Ms.Endpoint{
		MU:     sync.RWMutex{},
		ID:     "TEST",
		Metric: map[int]string{0: "TMETRICT"},
		Mdata:  map[string]int64{"TMETRICT": 110},
		Maxval: map[string]int64{"TMETRICT": 100},
	}

	return &Md.View{QNet: qn}
}
