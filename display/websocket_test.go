package monteverdi_test

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	Md "github.com/maroda/monteverdi/display"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

func TestPulsePatternToString(t *testing.T) {
	tests := []struct {
		name    string
		pattern Ms.PulsePattern
	}{
		{"iamb", Ms.Iamb},
		{"trochee", Ms.Trochee},
		{"amphibrach", Ms.Amphibrach},
		{"anapest", Ms.Anapest},
		{"dactyl", Ms.Dactyl},
		{"unknown", 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Md.PulsePatternToString(tt.pattern)
			assertStringContains(t, got, tt.name)
		})
	}
}

func TestCalcRing(t *testing.T) {
	now := time.Now()

	t.Run("Returns 0 for recent pulses", func(t *testing.T) {
		recent := now.Add(-30 * time.Second)
		got := Md.CalcRing(recent)
		want := 0
		assertInt(t, got, want)
	})

	t.Run("Returns 1 for medium age pulses", func(t *testing.T) {
		medium := now.Add(-5 * time.Minute)
		got := Md.CalcRing(medium)
		want := 1
		assertInt(t, got, want)
	})

	t.Run("Returns -1 for old pulses", func(t *testing.T) {
		old := now.Add(-2 * time.Hour)
		got := Md.CalcRing(old)
		want := -1
		assertInt(t, got, want)
	})
}

func TestCalcAngle(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		age        time.Duration
		wantRing   int
		checkAngle func(float64) bool
	}{
		{
			name:     "Inner ring start (just created)",
			age:      0 * time.Second,
			wantRing: 0,
			checkAngle: func(angle float64) bool {
				return math.Abs(angle-270.0) < 1.0 // Should be ~270° (12 o'clock)
			},
		},
		{
			name:     "Inner ring middle (30s old)",
			age:      30 * time.Second,
			wantRing: 0,
			checkAngle: func(angle float64) bool {
				return math.Abs(angle-90.0) < 5.0 // Should be ~90° (halfway around)
			},
		},
		{
			name:     "Inner ring boundary (59s old)",
			age:      59 * time.Second,
			wantRing: 0,
			checkAngle: func(angle float64) bool {
				return angle > 270.0 && angle < 280.0 // Almost full circle
			},
		},
		{
			name:     "Middle ring start (61s old)",
			age:      61 * time.Second,
			wantRing: 1,
			checkAngle: func(angle float64) bool {
				return math.Abs(angle-270.0) < 5.0 // Should be near 270°
			},
		},
		{
			name:     "Middle ring middle (5min old)",
			age:      5 * time.Minute,
			wantRing: 1,
			checkAngle: func(angle float64) bool {
				return angle > 100.0 && angle < 110.0 // Roughly 70°
			},
		},
		{
			name:     "Outer ring start (11min old)",
			age:      11 * time.Minute,
			wantRing: 2,
			checkAngle: func(angle float64) bool {
				return math.Abs(angle-270.0) < 10.0
			},
		},
		{
			name:     "Outer ring middle (30min old)",
			age:      30 * time.Minute,
			wantRing: 2,
			checkAngle: func(angle float64) bool {
				return angle > 145.0 && angle < 155.0 // Roughly 54°
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			past := now.Add(-tt.age)

			// Verify ring calculation
			ring := Md.CalcRing(past)
			if ring != tt.wantRing {
				t.Errorf("calcRing() = %d, want %d", ring, tt.wantRing)
			}

			// Verify angle calculation
			angle := Md.CalcAngle(past)
			if !tt.checkAngle(angle) {
				t.Errorf("calcAngle() = %f, failed validation for %s", angle, tt.name)
			}
		})
	}
}

func TestCalcAngle_Normalization(t *testing.T) {
	// Property tests
	ages := []time.Duration{
		1 * time.Second,
		30 * time.Second,
		59 * time.Second, // Near ring boundary
		61 * time.Second, // Just into ring 1
		5 * time.Minute,
		9 * time.Minute,
		30 * time.Minute,
		59 * time.Minute,
	}

	for _, age := range ages {
		t.Run(fmt.Sprintf("%v old", age), func(t *testing.T) {
			pulseTime := time.Now().Add(-age)
			angle := Md.CalcAngle(pulseTime)

			// Should always be normalized to 0-360 range
			if angle < 0 || angle >= 360 {
				t.Errorf("Angle %f not properly normalized for age %v", angle, age)
			}
		})
	}
}

func TestCalcSpeed(t *testing.T) {
	now := time.Now()
	config := Md.SpeedConfig{
		InnerBase:  2.0,
		MiddleBase: 1.0,
		OuterBase:  0.5,
		GlobalBase: 1.0,
	}

	tests := []struct {
		name  string
		start time.Time
		speed float64
	}{
		{"20s Ago", now.Add(-20 * time.Second), 2},
		{"200s Ago", now.Add(-200 * time.Second), 1},
		{"2000s Ago", now.Add(-2000 * time.Second), 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Md.CalcSpeed(tt.start, config)
			if got != tt.speed {
				t.Errorf("CalcSpeed() = %f, want %f", got, tt.speed)
			}
		})
	}

}

func TestCalcIntensity(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *Ms.Endpoint
		want     float64
	}{
		{
			name: "Fallback with no accents",
			endpoint: &Ms.Endpoint{
				Accent: make(map[string]*Ms.Accent),
				Mdata:  make(map[string]int64),
				Maxval: make(map[string]int64),
			},
			want: 0.5,
		},
		{
			name: "Fallback when nil",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": nil},
				Mdata:  make(map[string]int64),
				Maxval: make(map[string]int64),
			},
			want: 0.5,
		},
		{
			name: "Fallback when Mdata missing",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 5}},
				Mdata:  make(map[string]int64), // Empty, no CPU1
				Maxval: map[string]int64{"CPU1": 100},
			},
			want: 0.5,
		},
		{
			name: "Fallback when Maxval missing",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 5}},
				Mdata:  map[string]int64{"CPU1": 50},
				Maxval: make(map[string]int64), // Empty, no CPU1
			},
			want: 0.5,
		},
		{
			name: "Fallback when Maxval is 0",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 5}},
				Mdata:  map[string]int64{"CPU1": 50},
				Maxval: map[string]int64{"CPU1": 0}, // Zero
			},
			want: 0.5,
		},
		{
			name: "Calculates intensity correctly",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 5}}, // baseIntensity = 0.5
				Mdata:  map[string]int64{"CPU1": 50},
				Maxval: map[string]int64{"CPU1": 100}, // valueRatio = 0.5
			},
			want: 0.25, // baseIntensity (0.5) * valueRatio (0.5) = 0.25
		},
		{
			name: "Calculates intensity to minimum 0.2",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 1}}, // baseIntensity = 0.1
				Mdata:  map[string]int64{"CPU1": 10},
				Maxval: map[string]int64{"CPU1": 100}, // valueRatio = 0.1
			},
			want: 0.2, // 0.1 * 0.1 = 0.01, should clamp to 0.2
		},
		{
			name: "Calculates intensity to maximum 1.0",
			endpoint: &Ms.Endpoint{
				Accent: map[string]*Ms.Accent{"CPU1": {Intensity: 20}}, // baseIntensity = 2.0
				Mdata:  map[string]int64{"CPU1": 150},
				Maxval: map[string]int64{"CPU1": 100}, // valueRatio = 1.5
			},
			want: 1.0, // 2.0 * 1.5 = 3.0, should clamp to 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Md.CalcIntensity(tt.endpoint)
			if got != tt.want {
				t.Errorf("CalcIntensity() = %f, want %f", got, tt.want)
			}
		})
	}

	t.Run("Returns first accent when multiple exist", func(t *testing.T) {
		ep := &Ms.Endpoint{
			Accent: map[string]*Ms.Accent{
				"CPU1": {Intensity: 5},
				"CPU2": {Intensity: 8},
			},
			Mdata: map[string]int64{
				"CPU1": 50,
				"CPU2": 80,
			},
			Maxval: map[string]int64{
				"CPU1": 100,
				"CPU2": 100,
			},
		}

		// Should return on first accent found
		// (map iteration order is random, but it should return early)
		got := Md.CalcIntensity(ep)
		if got < 0.2 || got > 1.0 {
			t.Errorf("Intensity %f outside valid range", got)
		}
	})
}

func TestAmphibrachFlowToD3(t *testing.T) {
	// Create a QNet with controlled amphibrach data
	qn := makeQNetWithAmphibrachs(t)
	view := &Md.View{QNet: qn}

	t.Run("Amphibrachs appear in D3 data", func(t *testing.T) {
		pulses := view.GetPulseDataD3()

		amphibrachCount := 0
		for _, pulse := range pulses {
			if pulse.Dimension == 2 && pulse.Type == "amphibrach" {
				amphibrachCount++

				// Should be in middle or outer ring
				if pulse.Ring != 1 && pulse.Ring != 2 {
					t.Errorf("Amphibrach in wrong ring: %d", pulse.Ring)
				}
			}
		}

		if amphibrachCount == 0 {
			t.Error("No amphibrachs found in D3 data")
		}
	})

	t.Run("Amphibrachs persist through multiple calls", func(t *testing.T) {
		// Call GetPulseDataD3 multiple times, should get consistent results
		first := view.GetPulseDataD3()
		time.Sleep(100 * time.Millisecond)
		second := view.GetPulseDataD3()

		firstCount := countAmphibrachs(first)
		secondCount := countAmphibrachs(second)

		// Should have same or more amphibrachs (as they age)
		if secondCount < firstCount {
			t.Errorf("Amphibrachs disappeared: %d -> %d", firstCount, secondCount)
		}
	})
}

func TestCalcAngleForAmphibrach(t *testing.T) {
	now := time.Now()

	// Test amphibrach at different ages in middle ring
	ages := []time.Duration{
		61 * time.Second, // Just entered middle ring
		3 * time.Minute,  // Should be at ~60° (1/6th rotation)
		6 * time.Minute,  // Should be at ~180° (halfway)
		9 * time.Minute,  // Should be at ~300° (5/6th rotation)
	}

	for _, age := range ages {
		pulseTime := now.Add(-age)
		angle := Md.CalcAngle(pulseTime)
		t.Logf("Age: %v, Ring: %d, Angle: %.1f°",
			age, Md.CalcRing(pulseTime), angle)
	}
}

func TestView_GetPulseDataD3(t *testing.T) {
	t.Run("PulseData is nil if qnet or endpoint network is nil", func(t *testing.T) {
		qn := &Ms.QNet{}
		view := &Md.View{
			QNet: qn,
		}

		got := view.GetPulseDataD3()
		want := []Md.PulseDataD3{}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("View.GetPulseDataD3() = %+v, want %+v", got, want)
		}
	})
}

func TestWebsocketHandler(t *testing.T) {
	// uses gorilla/websocket to perform testing
	qn := makeNewTestQNet(t)

	// Add pulse data to the endpoint
	now := time.Now()
	testMetric := "CPU1"

	// Add D1 pulses to buffer
	qn.Network[0].Pulses.Buffer = []Ms.PulseEvent{
		{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-30 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		},
		{
			Dimension: 1,
			Pattern:   Ms.Trochee,
			StartTime: now.Add(-20 * time.Second),
			Duration:  4 * time.Second,
			Metric:    []string{testMetric},
		},
	}

	// Add accent data for intensity calculation
	qn.Network[0].Accent[testMetric] = &Ms.Accent{Intensity: 5}
	qn.Network[0].Mdata[testMetric] = 50
	qn.Network[0].Maxval[testMetric] = 100

	// Create test server
	view := &Md.View{
		QNet:  qn,
		Stats: Mo.NewStatsInternal(),
	}

	server := httptest.NewServer(http.HandlerFunc(view.WebsocketHandler))
	defer server.Close()

	// Connect as WebSocket client
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Could not connect: %v", err)
	}
	defer ws.Close()

	// Read a message
	var pulseData []Md.PulseDataD3
	err = ws.ReadJSON(&pulseData)
	if err != nil {
		t.Fatalf("Could not read JSON: %v", err)
	}

	// Assert on the data
	if pulseData == nil {
		t.Error("Expected pulse data, got nil")
	}

	if len(pulseData) == 0 {
		t.Error("Expected pulse data, got empty slice")
	}

	// Verify data structure
	for _, pulse := range pulseData {
		if pulse.Ring < 0 || pulse.Ring > 2 {
			t.Errorf("Invalid ring: %d", pulse.Ring)
		}
		if pulse.Angle < 0 || pulse.Angle >= 360 {
			t.Errorf("Invalid angle: %f", pulse.Angle)
		}
		if pulse.Type == "" {
			t.Error("Pulse type should not be empty")
		}
	}
}

func TestWebsocketHandler_ConnectionClosed(t *testing.T) {
	qn := makeNewTestQNet(t)

	// Add pulse data
	now := time.Now()
	testMetric := "CPU1"
	qn.Network[0].Pulses.Buffer = []Ms.PulseEvent{
		{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-30 * time.Second),
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		},
	}
	qn.Network[0].Accent[testMetric] = &Ms.Accent{Intensity: 5}
	qn.Network[0].Mdata[testMetric] = 50
	qn.Network[0].Maxval[testMetric] = 100

	view := &Md.View{
		QNet:  qn,
		Stats: Mo.NewStatsInternal(),
	}

	server := httptest.NewServer(http.HandlerFunc(view.WebsocketHandler))
	defer server.Close()

	// Connect as WebSocket client
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Could not connect: %v", err)
	}

	// Read first message successfully
	var pulseData []Md.PulseDataD3
	err = ws.ReadJSON(&pulseData)
	if err != nil {
		t.Fatalf("Could not read first message: %v", err)
	}

	// Close connection immediately (while handler loop is still trying to write)
	ws.Close()

	// Give handler time to attempt next write and hit the error path
	time.Sleep(200 * time.Millisecond)

	// Handler should have exited gracefully (no panic)
	// If we get here without panic, the error handling worked
}

func TestWebsocketHandler_DataFormat(t *testing.T) {
	view := &Md.View{
		QNet:  makeQNetWithAmphibrachs(t),
		Stats: Mo.NewStatsInternal(),
	}

	// Test data generation directly
	pulseData := view.GetPulseDataD3()

	// Verify structure
	if len(pulseData) == 0 {
		t.Errorf("No pulse data")
	}

	for _, pulse := range pulseData {
		// Validate fields
		if pulse.Ring < 0 || pulse.Ring > 1 {
			t.Errorf("Invalid pulse ring %d", pulse.Ring)
		}
		if pulse.Angle < 0 || pulse.Angle >= 360 {
			t.Errorf("Invalid pulse angle %f", pulse.Angle)
		}
		if pulse.Intensity < 0 || pulse.Intensity > 1 {
			t.Errorf("Invalid pulse intensity %f", pulse.Intensity)
		}
	}
}

// Helpers //

func makeQNetWithAmphibrachs(t *testing.T) *Ms.QNet {
	t.Helper()
	// Create endpoint with known amphibrachs in TemporalGrouper
	endpoint := makeEndpoint("TEST", "http://test")

	// Add some D2 pulses directly to the buffer
	now := time.Now()
	amphibrach := Ms.PulseEvent{
		Dimension: 2,
		Pattern:   Ms.Amphibrach,
		StartTime: now.Add(-2 * time.Minute), // 2 minutes old
		Duration:  30 * time.Second,
		Metric:    []string{"CPU1"},
	}

	endpoint.Pulses.Buffer = append(endpoint.Pulses.Buffer, amphibrach)

	return &Ms.QNet{Network: Ms.Endpoints{endpoint}}
}

func countAmphibrachs(pulses []Md.PulseDataD3) int {
	count := 0
	for _, pulse := range pulses {
		if pulse.Dimension == 2 && pulse.Type == "amphibrach" {
			count++
		}
	}
	return count
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

	nap := make(map[string]*Ms.Accent)

	// Struct matches the Endpoint type
	return &Ms.Endpoint{
		MU:       sync.RWMutex{},
		ID:       id,
		URL:      url,
		Delim:    "=",
		Metric:   c,
		Mdata:    d,
		Maxval:   x,
		Accent:   nap,
		Layer:    l,
		Sequence: is,
		Pulses:   tg,
	}
}

func assertError(t testing.TB, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("got error %q want %q", got, want)
	}
}

func assertGotError(t testing.TB, got error) {
	t.Helper()
	if got == nil {
		t.Errorf("Expected an error but got %q", got)
	}
}

func assertStatus(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("did not get correct status, got %d, want %d", got, want)
	}
}

func assertInt(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("did not get correct value, got %d, want %d", got, want)
	}
}

func assertInt64(t *testing.T, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("did not get correct value, got %d, want %d", got, want)
	}
}

func assertStringContains(t *testing.T, full, want string) {
	t.Helper()
	if !strings.Contains(full, want) {
		t.Errorf("Did not find %q, expected string contains %q", want, full)
	}
}
