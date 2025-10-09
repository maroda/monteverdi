package monteverdi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	Md "github.com/maroda/monteverdi/display"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

func TestView_SetupMux(t *testing.T) {
	qn := makeNewTestQNet(t)
	stats := Mo.NewStatsInternal()
	view := &Md.View{
		QNet:  qn,
		Stats: stats,
	}

	mux := view.SetupMux()

	t.Run("Websocket Endpoint answers", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/ws", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		// websocket upgrade will fail in test, but check for the 400
		assertStatus(t, w.Code, http.StatusBadRequest)
	})

	t.Run("Metrics Endpoint answers", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		assertStatus(t, w.Code, http.StatusOK)
	})

	t.Run("Version Endpoint answers with JSON", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/version", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		assertStatus(t, w.Code, http.StatusOK)

		// Does it return JSON?
		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assertError(t, err, nil)

		// Check for the version field
		if _, ok := resp["version"]; !ok {
			t.Errorf("Field 'version' not found in response")
		}
	})

}

func TestView_VersionHandler(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()

	view := &Md.View{}
	view.VersionHandler(w, r)

	// Check status code
	assertStatus(t, w.Code, http.StatusOK)

	// Check response, "dev" is the default
	want := "dev"
	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assertStringContains(t, response["version"], want)
}

func TestView_MetricsDataHandler(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/metrics-data", nil)
	w := httptest.NewRecorder()

	// Set up data: New QNet, add Endpoint, create View.
	qn := makeNewTestQNet(t)
	qn.Network[0] = &Ms.Endpoint{
		MU:     sync.RWMutex{},
		ID:     "TEST",
		Metric: map[int]string{0: "TMETRICT"},
		Mdata:  map[string]int64{"TMETRICT": 110},
		Maxval: map[string]int64{"TMETRICT": 100},
	}
	view := &Md.View{QNet: qn}

	view.MetricsDataHandler(w, r)
	t.Logf("Metrics data: %s", w.Body.String())

}

// NewTestQNet is a special use func for tests that manually set up data and don't need network calls
func makeNewTestQNet(t *testing.T) *Ms.QNet {
	t.Helper()
	endpoint := makeEndpoint("TESTING", "http://testing")
	return &Ms.QNet{
		MU:      sync.RWMutex{},
		Network: Ms.Endpoints{endpoint},
	}
}
