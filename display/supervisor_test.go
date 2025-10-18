package monteverdi_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	Md "github.com/maroda/monteverdi/display"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

func TestPollSupervisor(t *testing.T) {
	t.Run("Creates new struct", func(t *testing.T) {
		ep := makeEndpointWithMetrics(t)
		view := makeTestViewWithScreen(t, []*Ms.Endpoint{ep})
		ps := view.NewPollSupervisor()

		// Check if the view is the same
		if ps.View != view {
			t.Errorf("NewPollSupervisor() view = %v, want %v", ps.View, view)
		}
	})

	// This is the body that is fetched using Endpoint config
	kvbody := `CPU=44
MEM=555`
	metricsServer := makeMockWebServBody(0, kvbody)
	ep := makeEndpointEmptyMetricsURL(t, metricsServer.URL)
	view := makeTestViewWithScreen(t, []*Ms.Endpoint{ep})
	ps := view.NewPollSupervisor()

	t.Run("Starts Polling with Supervisor", func(t *testing.T) {
		ps.Start()
		defer ps.Stop()

		if ps.StopChan == nil {
			t.Errorf("StopChan() should be initialized, not nil")
		}
		if ps.Ticker == nil {
			t.Errorf("Ticker() should be initialized, not nil")
		}

		// Allow for one poll (every 1s) to happen
		time.Sleep(2 * time.Second)

		// Now the stored Endpoint should have filled Mdata
		if len(view.QNet.Network[0].Mdata) == 0 {
			t.Errorf("Expected value from poll, got 0")
		}
	})

	t.Run("Stops Polling with Supervisor", func(t *testing.T) {
		ps.Start()

		time.Sleep(2 * time.Second)

		done := make(chan struct{})
		go func() {
			ps.Stop()
			close(done)
		}()

		select {
		case <-done:
		// Success! Stop() returned
		case <-time.After(2 * time.Second):
			t.Fatalf("Polling did not stop after timeout")
		}
	})

	t.Run("Supervisor ticker stops", func(t *testing.T) {
		ps.Start()
		ps.Stop()
		// If we get this far there's no panic and the ticker stopped
	})

	t.Run("Restarts Polling Supervisor", func(t *testing.T) {
		ps.Start()
		time.Sleep(2 * time.Second)
		ps.Restart()

		time.Sleep(2 * time.Second)
		if len(view.QNet.Network[0].Mdata) == 0 {
			t.Errorf("Expected value from poll, got 0")
		}

		ps.Stop()
	})
}

func TestView_ReloadConfig(t *testing.T) {
	kvbody1 := `CPU=44
MEM=555`
	kvbody2 := `NETIN=6
NETOUT=777`
	metricsServ1 := makeMockWebServBody(0, kvbody1)
	ep := makeEndpointEmptyMetricsURL(t, metricsServ1.URL)
	view := makeTestViewWithScreen(t, []*Ms.Endpoint{ep})
	ps := view.NewPollSupervisor()

	metricsServ2 := makeMockWebServBody(0, kvbody2)

	t.Run("Reloads Config with Supervisor", func(t *testing.T) {
		// now this should create the new config from a test JSON file
		// that is configured to point to kvbody2
		ps.Start()
		time.Sleep(2 * time.Second)

		metric1 := view.QNet.Network[0].Mdata["CPU"]
		assertInt64(t, metric1, 44)

		// make new config
		data := `[{"id": "test2",
  "url": "` + metricsServ2.URL + `",
  "delim": "=",
  "metrics": {
      "NETIN": { "type": "rate", "max": 10 },
      "NETOUT": { "type": "rate", "max": 3000 }
  }}]`

		configFile, delConfig := createTempFile(t, data)
		defer delConfig()
		fileName := configFile.Name()
		loadConfig, err := Ms.LoadConfigFileName(fileName)
		assertError(t, err, nil)

		view.ReloadConfig(loadConfig)
		time.Sleep(2 * time.Second)

		metric2 := view.QNet.Network[0].Mdata["NETIN"]
		assertInt64(t, metric2, 6)

		if _, ok := view.QNet.Network[0].Mdata["CPU"]; ok {
			t.Error("Metric CPU should not exist after reload")
		}
	})
}

func TestView_ConfHandler(t *testing.T) {
	t.Run("Rejects invalid JSON", func(t *testing.T) {
		view := makeTestViewWithScreen(t, []*Ms.Endpoint{})
		r := httptest.NewRequest(http.MethodPost, "/conf", strings.NewReader("invalid json"))
		w := httptest.NewRecorder()

		view.ConfHandler(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	// Config file alef
	alefConfig := `[{"id": "test1", "url": "http://localhost:9999/metrics", "delim": "=", "metrics": {"CPU": {"type": "gauge", "max": 100}}}]`
	configFile, delConfig := createTempFile(t, alefConfig)
	defer delConfig()

	// Setup view with config path
	loadConfig, _ := Ms.LoadConfigFileName(configFile.Name())
	eps := Ms.NewEndpointsFromConfig(loadConfig)
	view := &Md.View{
		QNet:       Ms.NewQNet(*eps),
		Stats:      Mo.NewStatsInternal(),
		ConfigPath: configFile.Name(),
	}
	ps := view.NewPollSupervisor()
	ps.Start()
	defer ps.Stop()

	t.Run("Returns Config JSON", func(t *testing.T) {
		// Make GET request to retrieve current config
		r := httptest.NewRequest(http.MethodGet, "/conf", nil)
		w := httptest.NewRecorder()

		view.ConfHandler(w, r)
		assertStatus(t, w.Code, http.StatusOK)

		// Parse original config to struct
		var expectedConf []Ms.ConfigFile
		err := json.Unmarshal([]byte(alefConfig), &expectedConf)
		assertError(t, err, nil)

		// Parse response to struct
		var actualConf []Ms.ConfigFile
		err = json.Unmarshal(w.Body.Bytes(), &actualConf)
		assertError(t, err, nil)

		// Compare struct fields
		if !reflect.DeepEqual(actualConf, expectedConf) {
			t.Errorf("Config mismatch, expected %v, got %v", expectedConf, actualConf)
		}
	})

	t.Run("Errors on invalid Method", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/conf", nil)
		w := httptest.NewRecorder()
		view.ConfHandler(w, r)
		assertStatus(t, w.Code, http.StatusMethodNotAllowed)
	})

	t.Run("Reloads Config with Supervisor", func(t *testing.T) {
		// Make POST request with new config
		betaConfig := `[{"id": "test2", "url": "http://localhost:8888/metrics", "delim": "=", "metrics": {"MEM": {"type": "gauge", "max": 200}}}]`
		r := httptest.NewRequest(http.MethodPost, "/conf", strings.NewReader(betaConfig))
		w := httptest.NewRecorder()

		view.ConfHandler(w, r)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Wait a bit for the reload
		time.Sleep(100 * time.Millisecond)

		// Verify config was updated
		if view.QNet.Network[0].ID != "test2" {
			t.Errorf("Expected ID %s, got %s", "test2", view.QNet.Network[0].ID)
		}
	})
}

// Helpers //

// Mock responder for external API calls with configurable body content
func makeMockWebServBody(delay time.Duration, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testAnswer := []byte(body)
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		_, err := w.Write(testAnswer)
		if err != nil {
			log.Fatalf("ERROR: Could not write to output.")
		}
	}))
}

// Temporary OS file to use for testing configurations
func createTempFile(t testing.TB, data string) (*os.File, func()) {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "db")
	if err != nil {
		t.Fatalf("could not create temp file %v", err)
	}
	assertError(t, err, nil)

	tmpfile.Write([]byte(data))
	removeFile := func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}
	return tmpfile, removeFile
}
