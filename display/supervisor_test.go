package monteverdi_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
