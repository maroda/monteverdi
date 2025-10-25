package monteverdi_test

/*
	Monteverdi core tests
*/

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
	Mt "github.com/maroda/monteverdi/types"
)

func TestNewQNet(t *testing.T) {
	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name, "https://popg.xyz")

	t.Run("Endpoint ID matches", func(t *testing.T) {
		// create a slice of Endpoint (also type:Endpoints)
		// using the Endpoint created above, just one element
		var eps Ms.Endpoints
		eps = append(eps, ep)

		// create a new QNet
		// check that the ID was created OK
		qn := Ms.NewQNet(eps)
		got := qn.Network[0].ID
		want := eps[0].ID
		assertString(t, got, want)
	})
}

func TestQNet_PollMultiNetworkError(t *testing.T) {
	qn := NewTestQNet(t)

	t.Run("No error returned on bad URL (code continues)", func(t *testing.T) {
		qn.Network[0].URL = "http://unreachable-craquemattic:2345/metrics"
		qn.PollMulti()
	})

	t.Run("No error returned when endpoint times out (code continues)", func(t *testing.T) {
		slowServ := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 + webTimeout)
		}))
		defer slowServ.Close()

		qn.Network[0].URL = slowServ.URL
		qn.PollMulti()
	})
}

func TestQNet_PollMultiDataError(t *testing.T) {
	t.Run("No error on bad server data (code continues)", func(t *testing.T) {
		partialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "CPU1=notanumber")
			fmt.Fprintln(w, "CPU2=22.222")
			fmt.Fprintln(w, "CPU3=123")
			fmt.Fprintln(w, "CPU4=")
		}))
		defer partialServer.Close()

		qn := NewTestQNet(t)
		qn.Network[0].URL = partialServer.URL
		qn.Network[0].Metric = map[int]string{
			0: "CPU1",
			1: "CPU2",
			2: "CPU3",
			3: "CPU4",
		}

		qn.PollMulti()
	})
}

func TestNewEndpointsFromConfig(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
  "id": "MONTEVERDI_INTERNAL",
  "url": "http://localhost:8090/metrics",
  "delim": " ",
  "metrics": {
    "CPU_USAGE": {
      "type": "gauge",
      "max": 90
    },
    "HTTP_REQUESTS_TOTAL": {
      "type": "counter",
      "transformer": "calc_rate",
      "max": 3000
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
		if ep.Transformers["HTTP_REQUESTS_TOTAL"] == nil {
			t.Error("Transformer calc_rate was not returned for HTTP_REQUESTS_TOTAL")
			t.Logf("transformers: %+v", ep.Transformers)
		}
	})

	t.Run("Endpoint contains expected TSDB configuration", func(t *testing.T) {
		for _, c := range *eps {
			m := c.Metric
			got := c.Layer[m[0]].MaxSize
			want := 80

			assertInt(t, got, want)
		}
	})

	t.Run("Returns correct metadata", func(t *testing.T) {
		// there's only one member of the slice
		var got string
		for _, c := range *eps {
			got = c.ID
		}
		want := "MONTEVERDI_INTERNAL"
		assertString(t, got, want)
	})
}

func TestQNet_FindAccent(t *testing.T) {
	var eps Ms.Endpoints

	// Create remote server
	_, u := makeRemoteMetricsServer(1)
	for i := range u {
		name := "SAAS_" + strconv.Itoa(i)
		ep := makeEndpoint(name, *u[i])
		eps = append(eps, ep)
	}

	// create a new QNet and poll
	qn := Ms.NewQNet(eps)
	qn.PollMulti()

	t.Run("No accent with value below Maxval", func(t *testing.T) {
		// Using CPU1:10 from NewTestQNet
		k := "CPU1"
		qn.Network[0].Maxval[k] = 10
		qn.Network[0].Mdata[k] = 2
		accent := qn.FindAccent(k, 0)
		if accent.SourceID != "" {
			t.Errorf("Accent.SourceID expected to be blank, but got %s", accent.SourceID)
		}
	})

	t.Run("Accent with value above Maxval", func(t *testing.T) {
		// Using CPU1:10 from NewTestQNet
		k := "CPU1"
		qn.Network[0].Mdata[k] = 16
		accent := qn.FindAccent(k, 0)
		fmt.Println(accent)
		if accent.SourceID != k {
			t.Errorf("Accent.SourceID expected to be %s, but got %s", k, accent.SourceID)
		}
	})

	// create fake data for each
	/*
		for _, ep := range qn.Network {
			for mi, mv := range ep.Metric {
				ep.Mdata[mv] = 10 + int64(mi)
			}
		}
	*/

	t.Run("Fetches Correct Timestamp in Accent", func(t *testing.T) {
		for i := range qn.Network {
			get := qn.FindAccent("CPU1", i)
			got := truncateToDigits(get.Timestamp, 4)
			want := truncateToDigits(time.Now().UnixNano(), 4)
			assertInt64(t, got, want)

		}
	})
}

func TestConcurrentAccentDetection(t *testing.T) {
	qn := NewTestQNet(t)
	k := "CPU1"

	// create fake data for each
	for _, ep := range qn.Network {
		ep.MU.Lock()
		for mi, mv := range ep.Metric {
			ep.Mdata[mv] = 10 + int64(mi)
		}
		ep.MU.Unlock()
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Simulate varying data
				qn.Network[0].MU.Lock()
				if goroutineID%2 == 0 {
					qn.Network[0].Mdata[k] = 18 // Above threshold
				} else {
					qn.Network[0].Mdata[k] = 6 // Below threshold
				}

				qn.Network[0].MU.Unlock()
				qn.FindAccent(k, 0)
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panic, concurrent access is safe.
}

func TestQNet_PollMulti(t *testing.T) {
	t.Run("Continues polling other endpoints after one fails", func(t *testing.T) {
		// First endpoint - will fail (bad URL)
		badEndpoint := makeEndpoint("bad-endpoint", "http://localhost:99999")
		badEndpoint.Metric = map[int]string{0: "CPU1"}

		// Second endpoint - will succeed
		goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "CPU2=200")
			fmt.Fprintln(w, "CPU3=300")
		}))
		defer goodServer.Close()

		goodEndpoint := makeEndpoint("good-endpoint", goodServer.URL)
		goodEndpoint.Metric = map[int]string{0: "CPU2", 1: "CPU3"}

		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{badEndpoint, goodEndpoint},
		}

		qnet.PollMulti()

		// Verify the good endpoint was still polled successfully
		if goodEndpoint.Mdata["CPU2"] != 200 {
			t.Errorf("Expected CPU2=200, got %d", goodEndpoint.Mdata["CPU2"])
		}
		if goodEndpoint.Mdata["CPU3"] != 300 {
			t.Errorf("Expected CPU3=300, got %d", goodEndpoint.Mdata["CPU3"])
		}

		// Bad endpoint should have no data
		if len(badEndpoint.Mdata) > 0 {
			t.Errorf("Bad endpoint should have no metrics, got %v", badEndpoint.Mdata)
		}
	})

	var eps Ms.Endpoints

	// Create remote server
	_, u := makeRemoteMetricsServer(2)
	for i := range u {
		name := "SAAS_" + strconv.Itoa(i)
		ep := makeEndpoint(name, *u[i])
		eps = append(eps, ep)
	}

	// create a new QNet and poll
	qn := Ms.NewQNet(eps)
	qn.PollMulti()

	t.Run("Fetches correct IDs", func(t *testing.T) {
		got := qn.Network[0].ID
		want := "SAAS_0"
		assertString(t, got, want)

		got = qn.Network[1].ID
		want = "SAAS_1"
		assertString(t, got, want)
	})

	/*
		t.Run("Reads Accent", func(t *testing.T) {
			fmt.Println(qn.Network[0].Accent["CPU1"])
			fmt.Println(qn.Network[1].Accent["CPU2"])
		})
	*/
}

func TestEndpoint_ValToRuneWithCheckMax(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
  "id": "MONTEVERDI_INTERNAL",
  "url": "http://localhost:8090/metrics",
  "delim": " ",
  "metrics": {
    "CPU1": {
      "type": "gauge",
      "max": 10
    },
    "CPU2": {
      "type": "gauge",
      "max": 3
    },
    "CPU3": {
      "type": "gauge",
      "max": 10
    }
  }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	eps := Ms.NewEndpointsFromConfig(loadConfig)

	// create fake data for each
	for _, ep := range *eps {
		for mi, mv := range ep.Metric {
			ep.Mdata[mv] = 10 + int64(mi)
		}
	}

	// We know the function divides by 8,
	// so use a single maxval and a series
	// of numbers that will draw each rune
	m := int64(80)
	numset := []int64{81, 91, 101, 111, 121, 131, 141, 151}
	runes := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	t.Run("Returns the correct rune for each metric value", func(t *testing.T) {
		for _, ep := range *eps {
			for i, n := range numset {
				r := ep.ValToRuneWithCheckMax(n, m, true)
				if r != runes[i] {
					t.Errorf("ValToRune returned incorrect value for %d, got: %q, want: %q", n, r, runes[i])
				}
			}

		}
	})
}

func TestEndpoint_AddSecondWithCheck(t *testing.T) {
	var eps Ms.Endpoints

	// Create remote server
	_, u := makeRemoteMetricsServer(2)
	for i := range u {
		name := "SAAS_" + strconv.Itoa(i)
		ep := makeEndpoint(name, *u[i])
		eps = append(eps, ep)
	}

	// create a new QNet
	qn := Ms.NewQNet(eps)

	t.Run("Adds a metric/second and retrieves the correct rune", func(t *testing.T) {
		for _, ep := range qn.Network {
			// Get the first metric that actually exists
			for _, m := range ep.Metric {
				ep.AddSecondWithCheck(m, true)

				got := ep.Layer[m].Runes[1]
				want := '█'
				if got != want {
					t.Errorf("AddSecond returned incorrect value, got: %q, want: %q", got, want)
				}
				break
			}
		}
	})
}

func TestEndpoint_RecordIctus(t *testing.T) {
	qn := makeQNet(2)

	t.Run("Records an ictus", func(t *testing.T) {
		for _, ep := range qn.Network {
			// Get the first metric that actually exists
			for _, m := range ep.Metric {
				d := ep.Mdata[m]
				ep.AddSecondWithCheck(m, true)
				ep.RecordIctus(m, true, d)

				got := ep.Sequence[m].Metric
				want := m
				assertString(t, got, want)
				break
			}
		}
	})

	/*
		t.Run("Updates an ictus duration", func(t *testing.T) {
			for _, ep := range qn.Network {
				for _, m := range ep.Metric {
					// create a new ictus
					// for the subsequent one, update the previous one
					// then check the previous one for the update
				}
			}
		})

	*/
}

func TestEndpoint_GetDisplay(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
  "id": "MONTEVERDI_INTERNAL",
  "url": "http://localhost:8090/metrics",
  "delim": " ",
  "metrics": {
    "CPU1": {
      "type": "gauge",
      "max": 10
    },
    "CPU2": {
      "type": "gauge",
      "max": 3
    },
    "CPU3": {
      "type": "gauge",
      "max": 10
    }
  }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	eps := Ms.NewEndpointsFromConfig(loadConfig)

	// create fake data for each
	for _, ep := range *eps {
		for mi, mv := range ep.Metric {
			ep.Mdata[mv] = 10 + int64(mi)
		}
	}

	t.Run("Retrieves the correct runes for fake data accents", func(t *testing.T) {
		for _, ep := range *eps {
			m := ep.Metric[0]
			expectedRune := ep.ValToRuneWithCheckMax(ep.Mdata[m], ep.Maxval[m], true)

			var testrunes []rune
			for i := 0; i < ep.Layer[m].MaxSize; i++ {
				ep.AddSecondWithCheck(m, true)
				testrunes = append(testrunes, expectedRune)
			}

			got := ep.GetDisplay(ep.Metric[0])
			want := testrunes

			if !reflect.DeepEqual(got, want) {
				t.Errorf("GetDisplay returned incorrect value, got: %q, want: %q", got, want)
			}
		}
	})
}

func TestEndpoint_GetPulseVizData(t *testing.T) {
	qn := makeQNet(1)
	_, grouper := makePulsesWithGrouper()
	qn.Network[0].Pulses = grouper

	testMetric := "CPU1"
	now := time.Now()

	t.Run("Returns empty data when no pulses exist", func(t *testing.T) {
		got := qn.Network[0].GetPulseVizData(testMetric, nil)
		var want []Mt.PulseVizPoint
		want = []Mt.PulseVizPoint{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetPulseVizData returned incorrect value, got: %v, want: %v", got, want)
		}
	})

	t.Run("Filters by metric name", func(t *testing.T) {
		// Clear previous test data
		qn.Network[0].Pulses.Buffer = []Mt.PulseEvent{}

		// Add pulses for different metrics
		pulse1 := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: now.Add(-20 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}
		pulse2 := Mt.PulseEvent{
			Pattern:   Mt.Trochee,
			StartTime: now.Add(-15 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{"CPU4"},
		}

		qn.Network[0].Pulses.Buffer = append(qn.Network[0].Pulses.Buffer, pulse1, pulse2)

		got := qn.Network[0].GetPulseVizData(testMetric, nil)
		want := Mt.Iamb

		// Should only return points for the requested metric
		for _, point := range got {
			if point.Pattern != want {
				t.Errorf("Incorrect pattern for point, got: %q, want: %q", point.Pattern, want)
			}
		}
	})

	t.Run("Filters by pattern", func(t *testing.T) {
		// Setup endpoint with mixed pulse patterns
		ep := makeEndpoint("TEST", "http://test")
		ep.Pulses = &Ms.TemporalGrouper{
			Buffer: []Mt.PulseEvent{
				{Pattern: Mt.Iamb, StartTime: time.Now().Add(-10 * time.Second), Duration: 5 * time.Second, Metric: []string{testMetric}},
				{Pattern: Mt.Trochee, StartTime: time.Now().Add(-5 * time.Second), Duration: 3 * time.Second, Metric: []string{testMetric}},
			},
			Groups: []*Mt.PulseTree{
				{OGEvents: []Mt.PulseEvent{
					{Pattern: Mt.Iamb, StartTime: time.Now().Add(-20 * time.Second), Duration: 4 * time.Second, Metric: []string{testMetric}},
				}},
			},
		}

		patterns := []Mt.PulsePattern{Mt.Iamb, Mt.Trochee}
		for _, pattern := range patterns {
			points := ep.GetPulseVizData(testMetric, &pattern)
			for _, point := range points {
				if point.Pattern != pattern {
					t.Errorf("Expected only %q, got %q", pattern, point.Pattern)
				}
			}
		}

		// Use nil directly for no filter
		// This should get both pattern types
		points := ep.GetPulseVizData(testMetric, nil)
		hasIamb := false
		hasTrochee := false
		for _, point := range points {
			if point.Pattern == Mt.Iamb {
				hasIamb = true
			}
			if point.Pattern == Mt.Trochee {
				hasTrochee = true
			}
		}

		if !hasIamb {
			t.Error("Expected to find Iamb patterns when no filter applied")
		}
		if !hasTrochee {
			t.Error("Expected to find Trochee patterns when no filter applied")
		}
	})

	t.Run("Filters completed groups by pattern", func(t *testing.T) {
		ep := makeEndpoint("TEST", "http://test")

		// Setup completed groups with different patterns (no buffer pulses)
		ep.Pulses = &Ms.TemporalGrouper{
			Buffer: []Mt.PulseEvent{}, // Empty buffer to isolate groups testing
			Groups: []*Mt.PulseTree{
				{
					StartTime: time.Now().Add(-30 * time.Second),
					OGEvents: []Mt.PulseEvent{
						{Pattern: Mt.Iamb, StartTime: time.Now().Add(-25 * time.Second), Duration: 4 * time.Second, Metric: []string{"CPU1"}},
						{Pattern: Mt.Trochee, StartTime: time.Now().Add(-20 * time.Second), Duration: 3 * time.Second, Metric: []string{"CPU1"}},
					},
				},
				{
					StartTime: time.Now().Add(-15 * time.Second),
					OGEvents: []Mt.PulseEvent{
						{Pattern: Mt.Iamb, StartTime: time.Now().Add(-12 * time.Second), Duration: 2 * time.Second, Metric: []string{"CPU1"}},
					},
				},
			},
		}

		patterns := []Mt.PulsePattern{Mt.Iamb, Mt.Trochee}
		for _, pattern := range patterns {
			points := ep.GetPulseVizData(testMetric, &pattern)
			for _, point := range points {
				if point.Pattern != pattern {
					t.Errorf("Expected only %q from completed groups, got %d", pattern, point.Pattern)
				}
			}
			if len(points) == 0 {
				t.Errorf("Expected to find %q in completed groups", pattern)
			}
		}
	})

	t.Run("Converts buffer pulses to viz points", func(t *testing.T) {
		// Use a fixed time for consistent calculations
		testTime := time.Now()

		testPulse := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: testTime.Add(-30 * time.Second),
			Duration:  10 * time.Second,
			Metric:    []string{testMetric},
		}

		qn.Network[0].Pulses.Buffer = append(qn.Network[0].Pulses.Buffer, testPulse)

		// Manually call PulseToPoints with the same test time
		points := qn.Network[0].PulseToPoints(testPulse, testTime)

		// Check the position calculation directly
		expectedPos := 49 // 79 - 30 sec ago
		found := false
		for _, point := range points {
			t.Logf("Got point at position: %d", point.Position)
			if point.Position == expectedPos {
				found = true
				// ... rest of checks
			}
		}

		if !found {
			t.Errorf("Expected position %d not found. Available positions: %v",
				expectedPos, getPositions(points))
		}
	})

	t.Run("Returns empty when Pulses is nil", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU:     sync.RWMutex{},
			Pulses: nil,
		}

		got := ep.GetPulseVizData(testMetric, nil)
		if len(got) != 0 {
			t.Errorf("Pulses data should be zero")
		}
	})

	t.Run("Processes completed groups with original events", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU: sync.RWMutex{},
			Pulses: &Ms.TemporalGrouper{
				WindowSize: 60 * time.Second,
				Buffer:     []Mt.PulseEvent{},
				Groups:     []*Mt.PulseTree{},
			},
		}

		testPulse := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: now.Add(-30 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}

		// Create a group with original events
		group := &Mt.PulseTree{
			StartTime: now.Add(-35 * time.Second),
			OGEvents:  []Mt.PulseEvent{testPulse}, // Note: using your abbreviated field name
		}
		ep.Pulses.Groups = append(ep.Pulses.Groups, group)

		got := ep.GetPulseVizData(testMetric, nil)
		if len(got) > 0 {
		} else {
			t.Errorf("Expected pulses data")
		}
	})

	t.Run("Skips groups outside time window", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU: sync.RWMutex{},
			Pulses: &Ms.TemporalGrouper{
				WindowSize: 60 * time.Second,
				Buffer:     []Mt.PulseEvent{},
				Groups:     []*Mt.PulseTree{},
			},
		}

		now := time.Now()
		oldGroup := &Mt.PulseTree{
			StartTime: now.Add(-120 * time.Second), // Outside 60-second window
			OGEvents:  []Mt.PulseEvent{{Pattern: Mt.Iamb, Metric: []string{testMetric}}},
		}
		ep.Pulses.Groups = append(ep.Pulses.Groups, oldGroup)

		result := ep.GetPulseVizData(testMetric, nil)
		if len(result) != 0 {
			t.Errorf("Pulses data should be zero")
		}
	})

	t.Run("Trochee midpoint calculation", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Mt.PulseEvent{Pattern: Mt.Trochee}
		startPos, endPos := 10, 20
		midPoint := 15

		// Before midpoint should be accent
		before := ep.CalcAccentStateForPos(pulse, midPoint-1, startPos, endPos)
		if !before {
			t.Errorf("Expected Accent, got %v", before)
		}
		// At/after midpoint should be non-accent
		after := ep.CalcAccentStateForPos(pulse, midPoint, startPos, endPos)
		if after {
			t.Errorf("Expected NO Accent, got %v", after)
		}
	})

	t.Run("Amphibrach calculation", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Mt.PulseEvent{Pattern: Mt.Amphibrach}
		startPos, firstPt, secondPt, endPos := 10, 20, 30, 40

		firstThird := ep.CalcAccentStateForPos(pulse, firstPt-1, startPos, endPos)
		if firstThird {
			t.Errorf("Expected NO Accent, got %v", firstThird)
		}

		secondThird := ep.CalcAccentStateForPos(pulse, (firstPt+secondPt)/2, startPos, endPos)
		if !secondThird {
			t.Errorf("Expected Accent, got %v", secondThird)
		}

		thirdThird := ep.CalcAccentStateForPos(pulse, secondPt+1, startPos, endPos)
		if thirdThird {
			t.Errorf("Expected NO Accent, got %v", thirdThird)
		}

	})

	t.Run("Calculation returns false for no result", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Mt.PulseEvent{Pattern: 9}
		startPos, endPos := 10, 20
		midPoint := 15

		// Return false if the pattern isn't recognized
		got := ep.CalcAccentStateForPos(pulse, midPoint-1, startPos, endPos)
		if got {
			t.Errorf("Expected false, got %v", got)
		}
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

	t.Run("Initializes Hysteresis map if nil", func(t *testing.T) {
		// Create an endpoint WITHOUT initializing Hysteresis
		ep := &Ms.Endpoint{
			ID:         "test",
			Mdata:      make(map[string]int64),
			Hysteresis: nil,
		}

		// This should not panic - it should initialize the map
		ep.ValueToHysteresis("CPU1", 100)

		// Verify the map was created
		if ep.Hysteresis == nil {
			t.Error("Hysteresis map should have been initialized")
		}

		// Verify the buffer was created and value stored
		if ep.Hysteresis["CPU1"] == nil {
			t.Error("CycBuffer for CPU1 should have been created")
		}

		if ep.Hysteresis["CPU1"].Values[0] != 100 {
			t.Errorf("Expected value 100, got %d", ep.Hysteresis["CPU1"].Values[0])
		}
	})

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

func TestPulseToPoints_Clamping(t *testing.T) {
	// These tests assume the default char width of 80
	ep := &Ms.Endpoint{}
	now := time.Now()

	t.Run("Clamps start position to 0", func(t *testing.T) {
		// Pulse that started before visible window
		pulse := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: now.Add(-90 * time.Second), // Before 60-second window
			Duration:  10 * time.Second,
		}

		points := ep.PulseToPoints(pulse, now)
		for _, point := range points {
			if point.Position >= 0 {
			} else {
				t.Errorf("Point position should be zero or greater")
			}
		}
	})

	t.Run("Clamps end position to default with 79", func(t *testing.T) {
		// Long duration pulse that would extend beyond visible range
		pulse := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: now.Add(-10 * time.Second),
			Duration:  80 * time.Second, // Very long duration
		}

		points := ep.PulseToPoints(pulse, now)
		for _, point := range points {
			if point.Position <= 79 {
			} else {
				t.Errorf("Point position should be 79 or smaller")
			}
		}
	})
}

func TestConcurrentPollAndDisplay(t *testing.T) {
	var eps Ms.Endpoints
	_, u := makeRemoteMetricsServer(1)
	for i := range u {
		name := "SAAS_" + strconv.Itoa(i)
		ep := makeEndpoint(name, *u[i])
		eps = append(eps, ep)
	}

	//qn := NewTestQNet(t)
	qn := Ms.NewQNet(eps)
	var wg sync.WaitGroup

	// Simulate polling goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			qn.PollMulti()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Simulate display reading goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			// Simulate what your display code does
			for ni := range qn.Network {
				qn.Network[ni].MU.RLock()
				_ = qn.Network[ni].Mdata
				_ = qn.Network[ni].Accent
				qn.Network[ni].MU.RUnlock()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestPulseDetect_AddPulse(t *testing.T) {
	qn := makeQNet(1)
	testMetric := "CPU1"

	// Create a sequence that should generate pulses
	qn.Network[0].Sequence[testMetric] = &Ms.IctusSequence{
		Metric: testMetric,
		Events: []Mt.Ictus{
			{Timestamp: time.Now().Add(-10 * time.Second), IsAccent: false, Value: 5},
			{Timestamp: time.Now().Add(-7 * time.Second), IsAccent: true, Value: 15},
			{Timestamp: time.Now().Add(-3 * time.Second), IsAccent: false, Value: 8},
		},
	}

	t.Run("PulseDetect adds pulses to temporal grouper", func(t *testing.T) {
		// Make sure we have a fresh sequence that will generate pulses
		qn.Network[0].Sequence[testMetric] = &Ms.IctusSequence{
			Metric: testMetric,
			Events: []Mt.Ictus{
				{Timestamp: time.Now().Add(-15 * time.Second), IsAccent: false, Value: 5},
				{Timestamp: time.Now().Add(-10 * time.Second), IsAccent: true, Value: 15},
				{Timestamp: time.Now().Add(-5 * time.Second), IsAccent: false, Value: 8},
				{Timestamp: time.Now(), IsAccent: true, Value: 12},
			},
		}

		initialBufferSize := len(qn.Network[0].Pulses.Buffer)

		// This should trigger pulse detection and addition
		qn.PulseDetect(testMetric, 0)

		finalBufferSize := len(qn.Network[0].Pulses.Buffer)

		if finalBufferSize <= initialBufferSize {
			t.Errorf("Expected pulses to be added to buffer via PulseDetect. Initial: %d, Final: %d",
				initialBufferSize, finalBufferSize)
		}
	})

	t.Run("Debug boundary calculations", func(t *testing.T) {
		seq := qn.Network[0].Sequence[testMetric]

		// Print the raw events
		for i, event := range seq.Events {
			t.Logf("Event %d: IsAccent=%v, Time=%v", i, event.IsAccent, event.Timestamp.Format("15:04:05"))
		}

		// Check what your config is
		config := Ms.NewPulseConfig(0.0, 1.0, 0.0, 1.0)
		t.Logf("Config: IambStart=%.1f, IambEnd=%.1f, TrocheeStart=%.1f, TrocheeEnd=%.1f",
			config.IambStartPeriod, config.IambEndPeriod, config.TrocheeStartPeriod, config.TrocheeEndPeriod)
	})

	t.Run("Test with middle-to-middle config", func(t *testing.T) {
		seq := qn.Network[0].Sequence[testMetric]
		ictusSeq := &Ms.IctusSequence{
			Metric: testMetric,
			Events: make([]Mt.Ictus, len(seq.Events)),
		}

		for j, e := range seq.Events {
			ictusSeq.Events[j] = Mt.Ictus{
				Timestamp: e.Timestamp,
				IsAccent:  e.IsAccent,
				Value:     e.Value,
				Duration:  e.Duration,
			}
		}

		// Try middle-to-middle instead of 0.0,1.0,0.0,1.0
		config := Ms.NewPulseConfig(0.5, 0.5, 0.5, 0.5)
		pulses := ictusSeq.DetectPulsesWithConfig(*config)

		t.Logf("Middle-to-middle config returned %d pulses", len(pulses))
	})

	t.Run("Direct pulse addition to buffer", func(t *testing.T) {
		initialBufferSize := len(qn.Network[0].Pulses.Buffer)

		// Create a pulse directly and add it
		testPulse := Mt.PulseEvent{
			Pattern:   Mt.Iamb,
			StartTime: time.Now().Add(-10 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}

		qn.Network[0].Pulses.AddPulse(testPulse)

		finalBufferSize := len(qn.Network[0].Pulses.Buffer)

		if finalBufferSize <= initialBufferSize {
			t.Errorf("Expected pulse to be added to buffer. Initial: %d, Final: %d",
				initialBufferSize, finalBufferSize)
		}
	})

	t.Run("No pulses added with insufficient events", func(t *testing.T) {
		// Create sequence with only 1 event (insufficient for pulse detection)
		qn.Network[0].Sequence["CPU2"] = &Ms.IctusSequence{
			Metric: "CPU2",
			Events: []Mt.Ictus{
				{Timestamp: time.Now(), IsAccent: true, Value: 10},
			},
		}

		initialBufferSize := len(qn.Network[0].Pulses.Buffer)
		qn.PulseDetect("CPU2", 0)
		finalBufferSize := len(qn.Network[0].Pulses.Buffer)

		if finalBufferSize != initialBufferSize {
			t.Errorf("Expected no pulses added with insufficient events. Buffer size changed from %d to %d",
				initialBufferSize, finalBufferSize)
		}
	})

	t.Run("Empty pulse detection result", func(t *testing.T) {
		// Create sequence that won't generate pulses (no state transitions)
		qn.Network[0].Sequence["CPU3"] = &Ms.IctusSequence{
			Metric: "CPU3",
			Events: []Mt.Ictus{
				{Timestamp: time.Now().Add(-10 * time.Second), IsAccent: true, Value: 15},
				{Timestamp: time.Now().Add(-5 * time.Second), IsAccent: true, Value: 16},
				{Timestamp: time.Now(), IsAccent: true, Value: 17},
			},
		}

		initialBufferSize := len(qn.Network[0].Pulses.Buffer)
		qn.PulseDetect("CPU3", 0)
		finalBufferSize := len(qn.Network[0].Pulses.Buffer)

		// Should not add any pulses since there are no state transitions
		if finalBufferSize != initialBufferSize {
			t.Errorf("Expected no pulses added when no state transitions. Buffer size changed from %d to %d",
				initialBufferSize, finalBufferSize)
		}
	})
}

func TestMax(t *testing.T) {
	a1 := 420
	a2 := 42

	got := Ms.Max(a1, a2)
	assertInt(t, got, a1)

	get := Ms.Max(a2, a1)
	assertInt(t, get, a1)
}

func TestQNet_PulseDetect_SequenceTrimming(t *testing.T) {
	qn := makeQNet(1)
	testMetric := "CPU1"

	// Create a sequence with more than maxRecognitionEvents (10)
	events := make([]Mt.Ictus, 15) // More than the 10 limit
	for i := 0; i < 15; i++ {
		events[i] = Mt.Ictus{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			IsAccent:  i%2 == 0, // Alternate accent/non-accent
			Value:     int64(50 + i),
		}
	}

	qn.Network[0].Sequence[testMetric] = &Ms.IctusSequence{
		Metric:                  testMetric,
		Events:                  events,
		LastProcessedEventCount: 5, // Set to non-zero to verify reset
	}

	initialLength := len(qn.Network[0].Sequence[testMetric].Events)
	initialProcessedCount := qn.Network[0].Sequence[testMetric].LastProcessedEventCount

	// This should trigger the trimming
	qn.PulseDetect(testMetric, 0)

	// Verify trimming occurred
	finalLength := len(qn.Network[0].Sequence[testMetric].Events)
	finalProcessedCount := qn.Network[0].Sequence[testMetric].LastProcessedEventCount

	if initialLength != 15 {
		t.Errorf("Expected initial length 15, got %d", initialLength)
	}

	if finalLength != 10 {
		t.Errorf("Expected trimmed length 10, got %d", finalLength)
	}

	if finalProcessedCount != 10 {
		t.Errorf("Expected LastProcessedEventCount at 10, got %d", finalProcessedCount)
	}

	if initialProcessedCount == 0 {
		t.Errorf("Test setup issue: initialProcessedCount should have been non-zero")
	}
}

// Helpers //

// reverse []int64 in place
func reverse64(s []int64) []int64 {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// See what we actually got
func getPositions(points []Mt.PulseVizPoint) []int {
	positions := make([]int, len(points))
	for i, p := range points {
		positions[i] = p.Position
	}
	return positions
}

// Create an endpoint with a customizable ID and URL
// It contains three metrics and a data value for each metric
func makeEndpoint(i, u string) *Ms.Endpoint {
	// Fake ID
	id := i

	// Fake URL
	url := u

	// Collection map literal for Metric
	// What metrics we want to keep from the Poll
	c := make(map[int]string)
	c[1] = "CPU1"
	c[2] = "CPU2"
	c[3] = "CPU3"

	// Collection data map literal for Mdata
	// What data each of these metrics has from a PollMulti()
	// Normally no metrics come with a new Endpoint
	d := make(map[string]int64)
	//d[c[1]] = 11
	//d[c[2]] = 12
	//d[c[3]] = 13

	// Accent trigger data map literal for Maxval
	// Greater than or equal to, an accent happens
	x := make(map[string]int64)
	//x[c[1]] = 4
	//x[c[2]] = 5
	//x[c[3]] = 6

	// Initialize the Timeseries structure
	l := make(map[string]*Mt.Timeseries)
	for _, mName := range c {
		l[mName] = &Mt.Timeseries{
			Runes:   make([]rune, 60),
			MaxSize: 60,
			Current: 0,
		}
	}

	is := make(map[string]*Ms.IctusSequence)
	tg := &Ms.TemporalGrouper{
		WindowSize: 60 * time.Second,
		Buffer:     make([]Mt.PulseEvent, 0),
		Groups:     make([]*Mt.PulseTree, 0),
	}
	hb := make(map[string]*Ms.CycBuffer)

	// Struct matches the Endpoint type
	return &Ms.Endpoint{
		MU:         sync.RWMutex{},
		ID:         id,
		URL:        url,
		Delim:      "=",
		Metric:     c,
		Hysteresis: hb,
		Mdata:      d,
		Maxval:     x,
		Accent:     nil,
		Layer:      l,
		Sequence:   is,
		Pulses:     tg,
	}
}

func makeEndpointMTrans(metric string, url string, transformer Mp.MetricTransformer) *Ms.Endpoint {
	return &Ms.Endpoint{
		MU:     sync.RWMutex{},
		ID:     "test",
		URL:    url,
		Delim:  "=",
		Metric: map[int]string{0: metric},
		Mdata:  make(map[string]int64),
		Maxval: map[string]int64{metric: 1000},
		Accent: make(map[string]*Mt.Accent),
		Layer: map[string]*Mt.Timeseries{
			metric: {
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
		Transformers: map[string]Mp.MetricTransformer{metric: transformer},
	}
}

// Initialize a QNet for use with testing
func makeQNet(n int) *Ms.QNet {
	var eps Ms.Endpoints
	_, u := makeRemoteMetricsServer(n)
	for i := range u {
		name := "SAAS_" + strconv.Itoa(i)
		ep := makeEndpoint(name, *u[i])
		eps = append(eps, ep)
	}
	return Ms.NewQNet(eps)
}

func truncateToDigits(n int64, digits int) int64 {
	return int64(math.Pow10(digits)) % n
}

// NewTestQNet is a special use func for tests that manually set up data and don't need network calls
func NewTestQNet(t *testing.T) *Ms.QNet {
	t.Helper()
	endpoint := makeEndpoint("TESTING", "http://testing")
	return &Ms.QNet{
		MU:      sync.RWMutex{},
		Network: Ms.Endpoints{endpoint},
	}
}

// makeRemoteMetricsServer is for tests that need working endpoints with realistic data
// this data should match the metric values created by makeEndpoint
func makeRemoteMetricsServer(num int) ([]*httptest.Server, []*string) {
	// create KV data to look like prometheus
	kvbody := `CPU1=9
CPU2=23
CPU3=420
CPU4=1234`

	var WWW []*httptest.Server
	var URL []*string

	for i := 0; i < (num + 1); i++ {
		mockWWW := makeMockWebServBody(0*time.Millisecond, kvbody)
		WWW = append(WWW, mockWWW)
		URL = append(URL, &WWW[i].URL)
	}

	return WWW, URL
}
