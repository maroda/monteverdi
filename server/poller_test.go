package monteverdi

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShowBanner(t *testing.T) {
	t.Run("Prints configured string", func(t *testing.T) {
		welcome := "fantastic"
		buf := &bytes.Buffer{}

		err := showBanner(buf, welcome)
		got := buf.String()
		want := welcome + "\n"

		assertError(t, err, nil)
		assertString(t, got, want)
	})
}

// TestSingleFetch should handle single URLs
func TestSingleFetch(t *testing.T) {
	mockWWW := makeMockWebServBody(0*time.Millisecond, "craquemattic")
	urlWWW := mockWWW.URL

	t.Run("Fetches a single URL", func(t *testing.T) {
		want := "craquemattic"
		_, get, err := SingleFetch(urlWWW)

		got := string(get)
		assertError(t, err, nil)
		assertString(t, got, want)
	})

	t.Run("Returns Status 200", func(t *testing.T) {
		got, _, _ := SingleFetch(urlWWW)
		assertStatus(t, got, 200)
	})

	mockWWW.Close()

	t.Run("Returns Error after Server Close", func(t *testing.T) {
		_, _, got := SingleFetch(urlWWW)
		assertGotError(t, got)
		fmt.Println(got)
	})
}

// TestMetricKV should read KV values from a URL endpoint
func TestMetricKV(t *testing.T) {
	kvbody := `VAR1=value
VAR2=valuevalue
VAR3=valuevaluevalue
VAR4=valuevaluevaluevalue

# A comment
VAR5=valuevaluevaluevaluevalue
`
	mockWWW := makeMockWebServBody(0*time.Millisecond, kvbody)
	urlWWW := mockWWW.URL

	// Check that the correct number of values exist
	// This accounts for removal of whitespace and comments
	t.Run("Fetches correct count of all KV", func(t *testing.T) {
		get, err := MetricKV(urlWWW)
		got := len(get)
		want := 5

		assertError(t, err, nil)
		assertInt(t, got, want)
	})

	// Here we look for VAR4
	t.Run("Fetches known KV", func(t *testing.T) {
		get, _ := MetricKV(urlWWW)
		got := get["VAR4"]
		want := "valuevaluevaluevalue"

		assertString(t, got, want)
	})
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
