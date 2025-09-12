package monteverdi_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

const (
	webTimeout = 10 * time.Second
)

// TestSingleFetch should handle single URLs
func TestSingleFetch(t *testing.T) {
	mockWWW := makeMockWebServBody(0*time.Millisecond, "craquemattic")
	urlWWW := mockWWW.URL

	t.Run("Fetches a single URL", func(t *testing.T) {
		want := "craquemattic"
		_, get, err := Ms.SingleFetch(urlWWW)

		got := string(get)
		assertError(t, err, nil)
		assertString(t, got, want)
	})

	t.Run("Returns Status 200", func(t *testing.T) {
		got, _, _ := Ms.SingleFetch(urlWWW)
		assertStatus(t, got, 200)
	})

	// Close this mock server to run additional tests
	mockWWW.Close()

	t.Run("Returns Error after Server Close", func(t *testing.T) {
		_, _, got := Ms.SingleFetch(urlWWW)
		assertGotError(t, got)
		fmt.Println(got)
	})

	t.Run("Returns Error after Host Unreachable", func(t *testing.T) {
		_, _, err := Ms.SingleFetch("http://badhost:4420")
		assertGotError(t, err)
		assertStringContains(t, err.Error(), "no such host")
	})

	t.Run("Returns 500 Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", 500)
		}))
		defer server.Close()

		statusCode, _, err := Ms.SingleFetch(server.URL)
		assertStatus(t, statusCode, 500)
		assertError(t, err, nil)
	})

	t.Run("Returns Timeout Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 + webTimeout)
			w.Write([]byte("timeout"))
		}))
		defer server.Close()

		_, _, err := Ms.SingleFetch(server.URL)
		assertGotError(t, err)
		assertStringContains(t, err.Error(), "Timeout")
	})

	/* io.ReadAll is extremely robust, it is very difficult to get it to break

	t.Run("Returns Body Read Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("body"))
			hj, ok := w.(http.Hijacker)
			if !ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		}))
		defer server.Close()

		_, _, err := Ms.SingleFetch(server.URL)
		assertGotError(t, err)
	})
	*/
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
	delimiter := "="

	// Check that the correct number of values exist
	// This accounts for removal of whitespace and comments
	t.Run("Fetches correct count of all KV", func(t *testing.T) {
		get, err := Ms.MetricKV(delimiter, urlWWW)
		got := len(get)
		want := 5

		assertError(t, err, nil)
		assertInt(t, got, want)
	})

	// Here we look for VAR4
	t.Run("Fetches known KV", func(t *testing.T) {
		get, _ := Ms.MetricKV(delimiter, urlWWW)
		got := get["VAR4"]
		want := "valuevaluevaluevalue"

		assertString(t, got, want)
	})

	// We can use `=` or ` `
	t.Run("Works with alternative delimiter", func(t *testing.T) {
		kvb := `VAR1 value`
		mWWW := makeMockWebServBody(0*time.Millisecond, kvb)
		uWWW := mWWW.URL
		delimiter = " "
		get, _ := Ms.MetricKV(delimiter, uWWW)
		got := get["VAR1"]
		want := "value"

		assertString(t, got, want)
	})
}

func TestParseMetricKV(t *testing.T) {
	t.Run("Error on invalid delimiter", func(t *testing.T) {
		tests := []string{
			"CPU1_NO_DELIMITER\n", // No delimiter at all
			"CPU1\n",              // No delimiter
			"   \n",               // Malformed data
			"\n",                  // Malformed data
		}

		for _, testData := range tests {
			reader := strings.NewReader(testData)
			parse, err := Ms.ParseMetricKV(reader, "=")

			// none should be used
			assertInt(t, len(parse), 0)
			assertError(t, err, nil)
		}
	})

	t.Run("Trailing quotes and comments are removed", func(t *testing.T) {
		testData := `CPU1="value1"trailing"
CPU2='value2' # comment with spaces
CPU3=value3"embed_quote
CPU4=value4'embed_single_quote
CPU5=value5#comment_no_spaces
`
		reader := strings.NewReader(testData)
		parse, err := Ms.ParseMetricKV(reader, "=")
		assertError(t, err, nil)

		assertString(t, parse["CPU1"], "value1")
		assertString(t, parse["CPU2"], "value2")
		assertString(t, parse["CPU3"], "value3")
		assertString(t, parse["CPU4"], "value4")
		assertString(t, parse["CPU5"], "value5")
	})

	t.Run("Metrics in exponential notation handled", func(t *testing.T) {
		testData := `memory_bytes=4.21909151744e+11
cpu_usage=1.5e+2
disk_io=3.14159e-3
network_packets=2.5E+6
`

		reader := strings.NewReader(testData)
		parse, err := Ms.ParseMetricKV(reader, "=")

		assertError(t, err, nil)
		assertInt(t, len(parse), 4)

		assertString(t, parse["memory_bytes"], "4.21909151744e+11")
		assertString(t, parse["cpu_usage"], "1.5e+2")
		assertString(t, parse["disk_io"], "3.14159e-3")
		assertString(t, parse["network_packets"], "2.5E+6")
	})
}

type FailingReader struct {
	data      []byte
	position  int
	failAfter int
}

func (fr *FailingReader) Read(p []byte) (n int, err error) {
	if fr.position >= fr.failAfter {
		return 0, fmt.Errorf("simulated I/O error")
	}

	remaining := len(fr.data) - fr.position
	if remaining == 0 {
		return 0, io.EOF
	}

	toCopy := len(p)
	if toCopy > remaining {
		toCopy = remaining
	}

	copy(p, fr.data[fr.position:fr.position+toCopy])
	fr.position += toCopy
	return toCopy, nil
}

func TestParseMetricKV_ScannerError(t *testing.T) {
	failingReader := &FailingReader{
		data:      []byte("CPU1=100\nCPU2=200\n"),
		failAfter: 5,
	}

	_, err := Ms.ParseMetricKV(failingReader, "=")
	assertGotError(t, err)
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

func assertStringContains(t *testing.T, full, want string) {
	t.Helper()
	if !strings.Contains(full, want) {
		t.Errorf("Did not find %q, expected string contains %q", want, full)
	}
}
