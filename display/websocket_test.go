package monteverdi_test

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	Md "github.com/maroda/monteverdi/display"
	Ms "github.com/maroda/monteverdi/server"
)

func TestPulsePatternToString(t *testing.T) {
	tests := []struct {
		name    string
		pattern Ms.PulsePattern
	}{
		{"iamb", Ms.Iamb},
		{"trochee", Ms.Trochee},
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
		{
			name:     "Too old (2 hours)",
			age:      2 * time.Hour,
			wantRing: -1,
			checkAngle: func(angle float64) bool {
				return angle == 0.0 // Should return 0 for invalid ring
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
	now := time.Now()
	ep := makeEndpoint(now.String(), "http://now")
	//testMetric := "CPU1"

	t.Run("Accent intensity is set", func(t *testing.T) {
		got := Md.CalcIntensity(ep)
		want := 0.5 // Calculated value from makeEndpoint with Accent
		if got != want {
			t.Errorf("CalcIntensity() = %f, want %f", got, want)
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

/// Helpers

func makeQNetWithAmphibrachs(t *testing.T) *Ms.QNet {
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
