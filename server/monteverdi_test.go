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

// TODO: Add a test to check for multiple endpoints
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

func TestEndpoint_ValToRune(t *testing.T) {
	// create KV data
	kvbody := `VAR1=1`
	// create a mock web server
	mockWWW := makeMockWebServBody(0*time.Millisecond, kvbody)
	urlWWW := mockWWW.URL
	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name, urlWWW)

	runes := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	numset := []int64{10 - 1, 20 - 1, 30 - 1, 50 - 1, 80 - 1, 130 - 1, 210 - 1, 340}

	t.Run("Returns the correct rune for each metric value", func(t *testing.T) {
		for i, n := range numset {
			r := ep.ValToRune(n)
			if r != runes[i] {
				t.Errorf("ValToRune returned incorrect value, got: %q, want: %q", r, runes[i])
			}
		}
	})
}

func TestEndpoint_ValToRuneWithCheck(t *testing.T) {
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

	runes := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	numset := []int64{10 - 1, 20 - 1, 30 - 1, 50 - 1, 80 - 1, 130 - 1, 210 - 1, 340}

	t.Run("Returns the correct rune for each metric value", func(t *testing.T) {
		for _, ep := range *eps {
			for i, n := range numset {
				r := ep.ValToRuneWithCheck(n, true)
				if r != runes[i] {
					t.Errorf("ValToRune returned incorrect value, got: %q, want: %q", r, runes[i])
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

	// create a new QNet and poll
	qn := Ms.NewQNet(eps)

	t.Run("Adds a metric/second and retrieves the correct rune", func(t *testing.T) {
		for _, ep := range qn.Network {
			// Get the first metric that actually exists
			for _, m := range ep.Metric {
				ep.AddSecondWithCheck(m, true)

				got := ep.Layer[m].Runes[1]
				want := '▂'
				if got != want {
					t.Errorf("AddSecond returned incorrect value, got: %q, want: %q", got, want)
				}
				break
			}
		}
	})
}

func TestEndpoint_AddSecond(t *testing.T) {
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

	t.Run("Adds a metric/second and retrieves the correct rune", func(t *testing.T) {
		for _, ep := range *eps {
			m := ep.Metric[0]
			ep.AddSecond(m)

			// Check the rune. This should be < 13
			got := ep.Layer[m].Runes[1]
			want := '▂'
			if got != want {
				t.Errorf("AddSecond returned incorrect value, got: %q, want: %q", got, want)
			}
		}
	})
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
				ep.AddSecond(m)
				testrunes = append(testrunes, '▂')
			}

			got := ep.GetDisplay(ep.Metric[0])
			want := testrunes

			if !reflect.DeepEqual(got, want) {
				t.Errorf("GetDisplay returned incorrect value, got: %q, want: %q", got, want)
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

	// Struct matches the Endpoint type
	return &Ms.Endpoint{
		ID:     id,
		URL:    url,
		Delim:  "=",
		Metric: c,
		Mdata:  d,
		Maxval: x,
		Accent: nil,
		Layer:  l,
	}
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
