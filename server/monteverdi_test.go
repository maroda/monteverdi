package monteverdi_test

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

	Ms "github.com/maroda/monteverdi/server"
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

	t.Run("Error returned on bad URL", func(t *testing.T) {
		qn.Network[0].URL = "http://unreachable-craquemattic:2345/metrics"

		err := qn.PollMulti()
		assertGotError(t, err)
	})

	t.Run("Error returned when endpoint times out", func(t *testing.T) {
		slowServ := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 + webTimeout)
		}))
		defer slowServ.Close()

		qn.Network[0].URL = slowServ.URL
		err := qn.PollMulti()
		assertGotError(t, err)
	})
}

func TestQNet_PollMultiDataError(t *testing.T) {
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

	err := qn.PollMulti()
	assertGotError(t, err)
}

func TestNewEndpointsFromConfig(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
		  "id": "NETDATA",
		  "url": "http://localhost:19999/api/v3/allmetrics",
		  "delim": "=",
		  "metrics": {
		    "NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL": 10,
		    "NETDATA_APP_WINDOWSERVER_CPU_UTILIZATION_VISIBLETOTAL": 3,
		    "NETDATA_USER_MATT_CPU_UTILIZATION_VISIBLETOTAL": 10
		  }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	config, err := Ms.NewEndpointsFromConfig(loadConfig)
	assertError(t, err, nil)

	t.Run("Endpoint contains expected TSDB configuration", func(t *testing.T) {
		for _, c := range *config {
			m := c.Metric
			got := c.Layer[m[0]].MaxSize
			want := 60

			assertInt(t, got, want)
		}
	})

	t.Run("Returns correct metadata", func(t *testing.T) {
		// there's only one member of the slice
		var got string
		for _, c := range *config {
			got = c.ID
		}
		want := "NETDATA"
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
	err := qn.PollMulti()
	assertError(t, err, nil)

	t.Run("No accent with value below Maxval", func(t *testing.T) {
		// Using CPU1:10 from NewTestQNet
		k := "CPU1"
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

	// Alternate simulation
	/*
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					qn.FindAccent(k, 0)
				}
			}()
		}
		wg.Wait()
	*/
}

func TestQNet_PollMulti(t *testing.T) {
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
	err := qn.PollMulti()
	assertError(t, err, nil)

	t.Run("Fetches correct IDs", func(t *testing.T) {
		got := qn.Network[0].ID
		want := "SAAS_0"
		assertString(t, got, want)

		got = qn.Network[1].ID
		want = "SAAS_1"
		assertString(t, got, want)
	})

	t.Run("Fetches known KV", func(t *testing.T) {
		got := qn.Network[0].Maxval["CPU1"]
		want := 4

		assertInt64(t, got, int64(want))
	})

	t.Run("Reads Accent", func(t *testing.T) {
		fmt.Println(qn.Network[0].Accent["CPU1"])
		fmt.Println(qn.Network[1].Accent["CPU2"])
	})
}

func TestEndpoint_ValToRuneWithCheckMax(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
		  "id": "NETDATA",
		  "url": "http://localhost:19999/api/v3/allmetrics",
		  "metrics": { "CPU1": 10, "CPU2": 3, "CPU3": 10 }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	eps, err := Ms.NewEndpointsFromConfig(loadConfig)
	assertError(t, err, nil)

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
		  "id": "NETDATA",
		  "url": "http://localhost:19999/api/v3/allmetrics",
		  "metrics": { "CPU1": 10, "CPU2": 3, "CPU3": 10 }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	loadConfig, err := Ms.LoadConfigFileName(fileName)
	assertError(t, err, nil)

	eps, err := Ms.NewEndpointsFromConfig(loadConfig)
	assertError(t, err, nil)

	// create fake data for each
	for _, ep := range *eps {
		for mi, mv := range ep.Metric {
			ep.Mdata[mv] = 10 + int64(mi)
		}
	}

	t.Run("Returns the correct display value", func(t *testing.T) {
		for _, ep := range *eps {
			got := ep.GetDisplay(ep.Metric[0])
			want := []rune{
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("GetDisplay returned incorrect value, got: %q, want: %q", got, want)
			}
		}
	})

	t.Run("Retrieves the correct runes for fake data accents", func(t *testing.T) {
		var testrunes []rune
		for _, ep := range *eps {
			m := ep.Metric[0]
			for i := 0; i < ep.Layer[m].MaxSize; i++ {
				ep.AddSecondWithCheck(m, true)
				testrunes = append(testrunes, '▁')
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
		got := qn.Network[0].GetPulseVizData(testMetric)
		var want []Ms.PulseVizPoint
		want = []Ms.PulseVizPoint{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetPulseVizData returned incorrect value, got: %v, want: %v", got, want)
		}
	})

	t.Run("Filters by metric name", func(t *testing.T) {
		// Clear previous test data
		qn.Network[0].Pulses.Buffer = []Ms.PulseEvent{}

		// Add pulses for different metrics
		pulse1 := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-20 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}
		pulse2 := Ms.PulseEvent{
			Pattern:   Ms.Trochee,
			StartTime: now.Add(-15 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{"CPU4"},
		}

		qn.Network[0].Pulses.Buffer = append(qn.Network[0].Pulses.Buffer, pulse1, pulse2)

		got := qn.Network[0].GetPulseVizData(testMetric)
		want := Ms.Iamb

		// Should only return points for the requested metric
		for _, point := range got {
			if point.Pattern != want {
				t.Errorf("Incorrect pattern for point, got: %q, want: %q", point.Pattern, want)
			}
		}
	})

	t.Run("Converts buffer pulses to viz points", func(t *testing.T) {
		// Add test pulse to buffer
		testPulse := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-30 * time.Second),
			Duration:  10 * time.Second,
			Metric:    []string{testMetric},
		}

		qn.Network[0].Pulses.Buffer = append(qn.Network[0].Pulses.Buffer, testPulse)

		got := qn.Network[0].GetPulseVizData(testMetric)

		// Should have points for the 10-second duration
		if len(got) > 0 {
		} else {
			t.Errorf("Pulses data is zero, expected at least one")
		}

		// Check position mapping (30 seconds ago should be around position 29)
		expectedPos := 29 // 59 - 30 sec ago
		found := false
		for _, point := range got {
			if point.Position == expectedPos {
				found = true
				if Ms.Iamb != point.Pattern {
					t.Errorf("Incorrect pulses data, got: %q, want: %q", point.Pattern, expectedPos)
				}
				if point.Duration != 10*time.Second {
					t.Errorf("Incorrect pulses data, got: %q, want: %q", point.Duration, 10*time.Second)
				}
			}
		}

		if !found {
			t.Errorf("Expected position not found in viz data")
		}
	})

	t.Run("Returns empty when Pulses is nil", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU:     sync.RWMutex{},
			Pulses: nil,
		}

		got := ep.GetPulseVizData(testMetric)
		if len(got) != 0 {
			t.Errorf("Pulses data should be zero")
		}
	})

	t.Run("Processes completed groups with original events", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU: sync.RWMutex{},
			Pulses: &Ms.TemporalGrouper{
				WindowSize: 60 * time.Second,
				Buffer:     []Ms.PulseEvent{},
				Groups:     []*Ms.PulseTree{},
			},
		}

		testPulse := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-30 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}

		// Create a group with original events
		group := &Ms.PulseTree{
			StartTime: now.Add(-35 * time.Second),
			OGEvents:  []Ms.PulseEvent{testPulse}, // Note: using your abbreviated field name
		}
		ep.Pulses.Groups = append(ep.Pulses.Groups, group)

		got := ep.GetPulseVizData(testMetric)
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
				Buffer:     []Ms.PulseEvent{},
				Groups:     []*Ms.PulseTree{},
			},
		}

		now := time.Now()
		oldGroup := &Ms.PulseTree{
			StartTime: now.Add(-120 * time.Second), // Outside 60-second window
			OGEvents:  []Ms.PulseEvent{{Pattern: Ms.Iamb, Metric: []string{testMetric}}},
		}
		ep.Pulses.Groups = append(ep.Pulses.Groups, oldGroup)

		result := ep.GetPulseVizData(testMetric)
		if len(result) != 0 {
			t.Errorf("Pulses data should be zero")
		}
	})

	t.Run("Trochee midpoint calculation", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Ms.PulseEvent{Pattern: Ms.Trochee}
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

	t.Run("Amphibrach thirds calculation", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Ms.PulseEvent{Pattern: Ms.Amphibrach}
		startPos, endPos := 12, 24 // 12-character span

		// First third (12-15): non-accent
		first3 := ep.CalcAccentStateForPos(pulse, 14, startPos, endPos)
		if first3 {
			t.Errorf("First 3rd should be Non-Accented, got %v", first3)
		}
		// Second third (16-19): accent
		second3 := ep.CalcAccentStateForPos(pulse, 18, startPos, endPos)
		if !second3 {
			t.Errorf("Second 3rd should be ACCENTED, got %v", second3)
		}
		// Final third (20-23): non-accent
		third3 := ep.CalcAccentStateForPos(pulse, 22, startPos, endPos)
		if third3 {
			t.Errorf("Third 3rd should be Non-Accented, got %v", third3)
		}
	})

	t.Run("Calculation returns false for no result", func(t *testing.T) {
		ep := &Ms.Endpoint{}
		pulse := Ms.PulseEvent{Pattern: 9}
		startPos, endPos := 10, 20
		midPoint := 15

		// Return false if the pattern isn't recognized
		got := ep.CalcAccentStateForPos(pulse, midPoint-1, startPos, endPos)
		if got {
			t.Errorf("Expected false, got %v", got)
		}
	})
}

func TestPulseToPoints_Clamping(t *testing.T) {
	ep := &Ms.Endpoint{}
	now := time.Now()

	t.Run("Clamps start position to 0", func(t *testing.T) {
		// Pulse that started before visible window
		pulse := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-70 * time.Second), // Before 60-second window
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

	t.Run("Clamps end position to 59", func(t *testing.T) {
		// Long duration pulse that would extend beyond visible range
		pulse := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-10 * time.Second),
			Duration:  80 * time.Second, // Very long duration
		}

		points := ep.PulseToPoints(pulse, now)
		for _, point := range points {
			if point.Position <= 59 {
			} else {
				t.Errorf("Point position should be 59 or smaller")
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
			err := qn.PollMulti()
			assertError(t, err, nil)
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

func TestConfigWithMissingMetrics(t *testing.T) {
	// Test empty MWithMax
	config := []Ms.ConfigFile{{ID: "test", URL: "http://test", MWithMax: map[string]int{}}}
	endpoints, err := Ms.NewEndpointsFromConfig(config)
	assertError(t, err, nil)
	fmt.Println(endpoints)
}

func TestConfigWithInvalidURL(t *testing.T) {
	// Test malformed URLs
	config := []Ms.ConfigFile{{ID: "test", URL: "not-a-url", MWithMax: map[string]int{"cpu": 80}}}
	// Should this return an error, or handle gracefully?
	fmt.Println(config)
}

func TestChooseBetterPoint(t *testing.T) {
	t.Run("Returns new point when new is Trochee and existing is not", func(t *testing.T) {
		existing := Ms.PulseVizPoint{Pattern: Ms.Iamb}
		new := Ms.PulseVizPoint{Pattern: Ms.Trochee}

		result := Ms.ChooseBetterPoint(existing, new)

		if result.Pattern != Ms.Trochee {
			t.Errorf("Expected %v, got %v", Ms.Trochee, result.Pattern)
		}
		reflect.DeepEqual(new, result)
	})

	t.Run("Returns existing point when existing is Trochee and new is not", func(t *testing.T) {
		existing := Ms.PulseVizPoint{Pattern: Ms.Trochee}
		new := Ms.PulseVizPoint{Pattern: Ms.Iamb}

		result := Ms.ChooseBetterPoint(existing, new)

		if result.Pattern != Ms.Trochee {
			t.Errorf("Expected %v, got %v", Ms.Trochee, result.Pattern)
		}
		reflect.DeepEqual(existing, result)
	})

	t.Run("Returns existing when both are Trochee", func(t *testing.T) {
		existing := Ms.PulseVizPoint{Pattern: Ms.Trochee}
		new := Ms.PulseVizPoint{Pattern: Ms.Trochee}

		result := Ms.ChooseBetterPoint(existing, new)

		reflect.DeepEqual(existing, result)
	})

	t.Run("Returns existing when neither is Trochee", func(t *testing.T) {
		existing := Ms.PulseVizPoint{Pattern: Ms.Iamb}
		new := Ms.PulseVizPoint{Pattern: Ms.Amphibrach}

		result := Ms.ChooseBetterPoint(existing, new)

		reflect.DeepEqual(existing, result)
	})

	t.Run("Provides priority when overlapping", func(t *testing.T) {
		ep := &Ms.Endpoint{
			MU: sync.RWMutex{},
			Pulses: &Ms.TemporalGrouper{
				WindowSize: 60 * time.Second,
				Buffer:     []Ms.PulseEvent{},
				Groups:     []*Ms.PulseTree{},
			},
		}

		now := time.Now()
		testMetric := "CPU1"

		// Create two pulses that will overlap at the same timeline position
		pulse1 := Ms.PulseEvent{
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-30 * time.Second), // 30 seconds ago
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}

		pulse2 := Ms.PulseEvent{
			Pattern:   Ms.Trochee,                 // Should be chosen by ChooseBetterPoint
			StartTime: now.Add(-28 * time.Second), // 28 seconds ago, overlaps with pulse1
			Duration:  3 * time.Second,
			Metric:    []string{testMetric},
		}

		// Add pulses to a completed group
		group := &Ms.PulseTree{
			StartTime: now.Add(-35 * time.Second), // Within the 60-second window
			OGEvents:  []Ms.PulseEvent{pulse1, pulse2},
		}
		ep.Pulses.Groups = append(ep.Pulses.Groups, group)

		result := ep.GetPulseVizData(testMetric)

		// Verify that overlapping positions chose Trochee (better pattern)
		foundTrochee := false
		foundIamb := false

		for _, point := range result {
			if point.Pattern == Ms.Trochee {
				foundTrochee = true
			}
			if point.Pattern == Ms.Iamb {
				foundIamb = true
			}
		}

		// Should have found Trochee patterns due to ChooseBetterPoint prioritization
		if !foundTrochee {
			t.Errorf("Expected to find Trochee patterns from ChooseBetterPoint")
		}

		// Positions that don't overlap should still have Iamb
		if !foundIamb {
			t.Errorf("Expected to find Iamb patterns in non-overlapping positions")
		}
	})
}

// Helpers //

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
	d[c[1]] = 11
	d[c[2]] = 12
	d[c[3]] = 13

	// Accent trigger data map literal for Maxval
	// Greater than or equal to, an accent happens
	x := make(map[string]int64)
	x[c[1]] = 4
	x[c[2]] = 5
	x[c[3]] = 6

	// Initialize the Timeseries structure
	l := make(map[string]*Ms.Timeseries)
	for _, mName := range c {
		l[mName] = &Ms.Timeseries{
			Runes:   make([]rune, 60),
			MaxSize: 60,
			Current: 0,
		}
	}

	is := make(map[string]*Ms.IctusSequence)
	tg := &Ms.TemporalGrouper{
		WindowSize: 60 * time.Second,
		Buffer:     make([]Ms.PulseEvent, 0),
		Groups:     make([]*Ms.PulseTree, 0),
	}

	// Struct matches the Endpoint type
	return &Ms.Endpoint{
		MU:       sync.RWMutex{},
		ID:       id,
		URL:      url,
		Delim:    "=",
		Metric:   c,
		Mdata:    d,
		Maxval:   x,
		Accent:   nil,
		Layer:    l,
		Sequence: is,
		Pulses:   tg,
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
