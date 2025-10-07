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
	"reflect"
	"sync"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

func TestQNet_PollMultiPlugin(t *testing.T) {
	metric := "CPU1"

	t.Run("Returns correct rate from plugin", func(t *testing.T) {
		// Create mock server that counts
		mockMetricsServ := makeRemoteGauge(400)
		defer mockMetricsServ.Close()

		// Create full endpoint with transformer
		ep := makeEndpointMTrans(metric, mockMetricsServ.URL, &MockCalcRatePlugin{})
		qn := Ms.NewQNet(Ms.Endpoints{ep})

		// First poll - no rate yet
		err := qn.PollMulti()
		assertError(t, err, nil)
		firstVal := qn.Network[0].Mdata[metric]
		t.Logf("First val: %d", firstVal)

		// Second poll - should have a rate
		time.Sleep(1 * time.Second)
		err = qn.PollMulti()
		assertError(t, err, nil)
		secondVal := qn.Network[0].Mdata[metric]
		t.Logf("Second val: %d", secondVal)

		// Rate should be ~50/sec (the increment in makeRemoteGauge)
		if secondVal < 45 || secondVal > 55 {
			t.Errorf("Expected rate around 50/sec, got %d", secondVal)
		}
	})

	t.Run("Returns transformer error", func(t *testing.T) {
		// Create mock server
		mockMetricsServ := makeRemoteGauge(100)
		defer mockMetricsServ.Close()

		// Create endpoint with a transformer that errors
		ep := makeEndpointMTrans(metric, mockMetricsServ.URL, &MockErrorTransformer{})
		qn := Ms.NewQNet(Ms.Endpoints{ep})

		// should return a transformer error
		err := qn.PollMulti()

		assertGotError(t, err)
		assertStringContains(t, err.Error(), "error transforming CPU1")
		assertStringContains(t, err.Error(), "mock transformer error")
	})
}

func TestEndpoint_GetHysteresis(t *testing.T) {
	ep := makeEndpoint("test", "http://test")
	metric := "CPU1"

	t.Run("No hysteresis exists", func(t *testing.T) {
		got := ep.GetHysteresis(metric, 5)
		want := []int64{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetHysteresis = %v, want %v", got, want)
		}
	})

	t.Run("Retrieves chronological hysteresis metrics", func(t *testing.T) {
		valuesForMetric := []int64{10, 11, 12, 13, 14, 15, 16}
		for _, mv := range valuesForMetric {
			// Write a new data value, as if we have gotten a new read
			ep.MU.Lock()
			ep.Mdata[metric] = mv
			ep.MU.Unlock()

			// Write that value to the buffer
			ep.ValueToHysteresis(metric, mv)
		}

		// This should return the last five entries
		// in reverse chronological order
		got := ep.GetHysteresis(metric, 5)
		want := reverse64(valuesForMetric[2:])
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetHysteresis() = %v, want %v", got, want)
		}
	})

	t.Run("Clamps retrieval depth to MaxSize", func(t *testing.T) {
		// Collect and write 20 values to the buffer
		var valuesForMetric []int64
		for i := 0; i < 20; i++ {
			valuesForMetric = append(valuesForMetric, int64(i))
			ep.MU.Lock()
			ep.Mdata[metric] = int64(i)
			ep.MU.Unlock()
			ep.ValueToHysteresis(metric, int64(i))
		}

		// Attempt to get a larger depth than 20
		got := ep.GetHysteresis(metric, 30)
		want := reverse64(valuesForMetric)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetHysteresis() = %v, want %v", got, want)
		}
	})
}

func TestEndpoint_ValueToHysteresis(t *testing.T) {
	ep := makeEndpoint("test", "http://test")
	metric := "CPU1"
	mdata := int64(11)

	t.Run("Writes at least one value to hysteresis", func(t *testing.T) {
		ep.ValueToHysteresis(metric, mdata)

		got := ep.Hysteresis[metric].Values[0]
		assertInt64(t, got, mdata)
	})

	t.Run("Writes multiple values to hysteresis", func(t *testing.T) {
		// Because we're checking for the entire Values, we have 20 values to set
		valuesForMetric := []int64{10, 11, 12, 13, 14, 15, 16}
		for _, mv := range valuesForMetric {
			// Write a new data value, as if we have gotten a new read
			ep.MU.Lock()
			ep.Mdata[metric] = mv
			ep.MU.Unlock()

			// Write that value to the buffer
			ep.ValueToHysteresis(metric, mv)
		}

		// Use GetHysteresis() to retrieve only the values recorded
		got := ep.GetHysteresis(metric, len(valuesForMetric))
		want := reverse64(valuesForMetric)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Hysteresis Values = %v, want %v", got, want)
		}
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
