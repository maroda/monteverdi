package plugin_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
)

func TestCalcRate(t *testing.T) {
	curr := int64(420)
	prev := int64(400)
	currtime := time.Now()
	timeago := currtime.Add(-5 * time.Second)

	t.Run("Returns rate calculation", func(t *testing.T) {
		// The rate of 400 -> 420 over 5 seconds is 4 (20/5)
		want := int64(4)
		got := Mp.CalcRate(curr, prev, currtime, timeago)
		assertInt64(t, got, want)
	})

	t.Run("Handles counter reset to 0", func(t *testing.T) {
		curr = 0
		// The rate of 400 -> 420 over 5 seconds is 4 (20/5)
		want := int64(0)
		got := Mp.CalcRate(curr, prev, currtime, timeago)
		assertInt64(t, got, want)
	})
}

func TestCalcRatePlugin(t *testing.T) {
	metric := "CPU1"
	curr := int64(420)
	prev := int64(400)
	prevval := make(map[string]int64)
	prevval[metric] = prev

	history := []int64{prev}
	currtime := time.Now()
	timeago := currtime.Add(-5 * time.Second)
	prevtime := make(map[string]time.Time)
	prevtime[metric] = timeago

	t.Run("HysteresisReq returns the correct value", func(t *testing.T) {
		plugin := Mp.CalcRatePlugin{}
		want := 1
		got := plugin.HysteresisReq()
		assertInt(t, got, want)
	})

	t.Run("Type returns the correct value", func(t *testing.T) {
		plugin := Mp.CalcRatePlugin{}
		want := "calc_rate"
		got := plugin.Type()
		assertStringContains(t, got, want)
	})

	t.Run("Returns transformation for CalcRate", func(t *testing.T) {
		plugin := Mp.CalcRatePlugin{
			prevval,
			prevtime,
		}

		rate, err := plugin.Transform(metric, curr, history, currtime)
		assertError(t, err, nil)

		want := int64(4)
		assertInt64(t, rate, want)
	})

	t.Run("Starts new rate measurement series with no previous metric", func(t *testing.T) {

		// PrevVal[metric] does not exist
		// assume it is nil
		plugin := Mp.CalcRatePlugin{}

		rate, err := plugin.Transform(metric, curr, history, currtime)
		assertError(t, err, nil)

		want := int64(0)
		assertInt64(t, rate, want)
	})

	t.Run("Returns zero when Hysteresis Requirement is not met", func(t *testing.T) {
		plugin := Mp.CalcRatePlugin{}

		rate, err := plugin.Transform(metric, curr, []int64{}, currtime)
		assertError(t, err, nil)

		want := int64(0)
		assertInt64(t, rate, want)
	})
}

/// Helpers

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
