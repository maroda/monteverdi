package monteverdi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
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

		if len(ps.Pollers) == 0 {
			t.Errorf("Pollers = %v, want at least 1", len(ps.Pollers))
		}
		for i, poller := range ps.Pollers {
			if poller.StopChan == nil {
				t.Errorf("Poller[%d] StopChan() should be initialized, not nil", i)
			}
			if poller.Ticker == nil {
				t.Errorf("Poller[%d] Ticker() should be initialized, not nil", i)
			}
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

	t.Run("Sets interval to default when 0", func(t *testing.T) {
		// The only response here is a log entry,
		// the configuration value is not changed from 0
		var logBuffer bytes.Buffer
		logHandler := slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		logger := slog.New(logHandler)
		slog.SetDefault(logger)

		// Set the interval to 0
		view.QNet.Network[0].MU.Lock()
		view.QNet.Network[0].Interval = 0
		view.QNet.Network[0].MU.Unlock()

		// Start the Supervisor
		ps.Start()
		defer ps.Stop()

		// Check if the log message made it through
		logOut := logBuffer.String()
		assertStringContains(t, logOut, "Poller interval is 0, using default of 15s")
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
		view.Supervisor = ps
		view.Supervisor.Start()
		defer view.Supervisor.Stop()

		time.Sleep(2 * time.Second)

		metric1 := view.QNet.Network[0].Mdata["CPU"]
		assertInt64(t, metric1, 44)

		// make new config
		data := `[{"id": "test2",
  "url": "` + metricsServ2.URL + `",
  "delim": "=",
  "interval": 1,
  "metrics": {
      "NETIN": { "type": "rate", "max": 10 },
      "NETOUT": { "type": "rate", "max": 3000 }
  }}]`

		configFile, delConfig := createTempFile(t, data)
		defer delConfig()
		fileName := configFile.Name()
		loadConfig, err := Ms.LoadConfigFileName(fileName)
		assertError(t, err, nil)

		// This will stop the old supervisor
		// and create a new one with the new config
		// In addition to the config file,
		// it expects a context for otel.
		ctx := context.Background()
		view.ReloadConfig(ctx, loadConfig)
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

	// Setup view with config path
	loadConfig, _ := Ms.LoadConfigFileName(configFile.Name())
	eps := Ms.NewEndpointsFromConfig(loadConfig)
	view := &Md.View{
		QNet:       Ms.NewQNet(*eps),
		Stats:      Mo.NewStatsInternal(),
		ConfigPath: configFile.Name(),
	}
	view.Supervisor = view.NewPollSupervisor()
	view.Supervisor.Start()
	defer view.Supervisor.Stop()

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

	t.Run("Errors on invalid JSON POST", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/conf", errorReader{})
		w := httptest.NewRecorder()
		view.ConfHandler(w, r)
		assertStatus(t, w.Code, http.StatusBadRequest)
	})

	// This test must be last, it removes the config to check for error
	t.Run("Errors when loading config file", func(t *testing.T) {
		// Make GET request to retrieve current config
		r := httptest.NewRequest(http.MethodGet, "/conf", nil)
		w := httptest.NewRecorder()

		// remove config file
		delConfig()
		view.ConfHandler(w, r)
		assertStatus(t, w.Code, http.StatusInternalServerError)
	})
}

// Helpers //

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

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
