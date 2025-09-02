package monteverdi

import (
	"bytes"
	"errors"
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

func TestLightUp(t *testing.T) {
	t.Run("Prints configured string", func(t *testing.T) {
		buf := &bytes.Buffer{}

		msg := "Poller"
		err := LightUp(buf, msg)
		got := buf.String()
		want := "Poller\n"

		assertError(t, err, nil)
		assertString(t, got, want)
	})
}

func TestSingleFetch(t *testing.T) {
	mockWWW := makeMockWebServ(0 * time.Millisecond)
	urlWWW := mockWWW.URL

	t.Run("Fetches a single URL", func(t *testing.T) {
		want := "craquemattic"
		_, got, err := SingleFetch(urlWWW)

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
	})
}

// Mock responder for external API calls
func makeMockWebServ(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testAnswer := []byte("craquemattic")
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		_, err := w.Write(testAnswer)
		if err != nil {
			log.Fatalf("ERROR: Could not write to output.")
		}
	}))
}

// Mock responder for broken body testing
func makeMockWebServBody(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "100") // Claim 100 bytes
		w.Write([]byte(`{"incomplete": true`))  // But only write 20
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
