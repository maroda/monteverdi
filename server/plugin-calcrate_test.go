package monteverdi_test

/*
	Plugin tests that are separate from monteverdi core
	As more plugins are developed, testing should go in separate files.
*/

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

func TestQNet_PollEndpointPlugin(t *testing.T) {
	metric := "CPU1"

	t.Run("Returns correct rate from plugin", func(t *testing.T) {
		// Create mock server that counts
		mockMetricsServ := makeRemoteGauge(400)
		defer mockMetricsServ.Close()

		// Create full endpoint with transformer
		ep := makeEndpointMTrans(metric, mockMetricsServ.URL, &MockCalcRatePlugin{})
		qn := Ms.NewQNet(Ms.Endpoints{ep})

		// First poll - no rate yet
		qn.PollEndpoint(0)
		firstVal := qn.Network[0].Mdata[metric]
		t.Logf("First val: %d", firstVal)

		// Second poll - should have a rate
		time.Sleep(1 * time.Second)
		qn.PollEndpoint(0)
		secondVal := qn.Network[0].Mdata[metric]
		t.Logf("Second val: %d", secondVal)

		// Rate should be ~50/sec (the increment in makeRemoteGauge)
		if secondVal < 45 || secondVal > 55 {
			t.Errorf("Expected rate around 50/sec, got %d", secondVal)
		}
	})

	t.Run("No error returned from transformer (code continues)", func(t *testing.T) {
		// Create mock server
		mockMetricsServ := makeRemoteGauge(100)
		defer mockMetricsServ.Close()

		// Create endpoint with a transformer that errors
		ep := makeEndpointMTrans(metric, mockMetricsServ.URL, &MockErrorTransformer{})
		qn := Ms.NewQNet(Ms.Endpoints{ep})
		qn.PollEndpoint(0)
	})
}

// create a remote metric server with one value,
// helpful for creating the illusion of a changing
// value on a remote server.
func makeRemoteGauge(start int) *httptest.Server {
	counterVal := start
	mu := sync.Mutex{}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		metric := fmt.Sprintf("CPU1=%d\n", counterVal)
		counterVal += 50 // Increment for next request
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(metric))
	}))
}

// MockCalcRatePlugin simulates the transformer plugin with just one previous entry
type MockCalcRatePlugin struct {
	PrevVal  int64
	PrevTime time.Time
}

func (p *MockCalcRatePlugin) Transform(metric string, current int64, historical []int64, timestamp time.Time) (int64, error) {
	// No rate, first time reading
	if p.PrevTime.IsZero() {
		p.PrevVal = current
		p.PrevTime = timestamp
		return 0, nil
	}

	// calc rate here
	delta := current - p.PrevVal
	timeDelta := timestamp.Sub(p.PrevTime).Seconds()
	if timeDelta == 0 {
		return 0, nil
	}
	rate := int64(float64(delta) / timeDelta)

	// Update for next call
	p.PrevVal = current
	p.PrevTime = timestamp

	return rate, nil
}

func (p *MockCalcRatePlugin) HysteresisReq() int { return 1 }
func (p *MockCalcRatePlugin) Type() string       { return "mock_rate" }

// MockErrorTransformer always returns an error //
type MockErrorTransformer struct{}

func (m *MockErrorTransformer) Transform(metric string, current int64, historical []int64, timestamp time.Time) (int64, error) {
	return 0, errors.New("mock transformer error")
}

func (m *MockErrorTransformer) HysteresisReq() int { return 1 }
func (m *MockErrorTransformer) Type() string       { return "mock_error" }
