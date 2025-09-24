package monteverdi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	webTimeout = 10 * time.Second
)

type HTTPClient interface {
	Get(string) (*http.Response, error)
}

// Shared HTTP Client
var sharedHTTPClient = &http.Client{
	Timeout: webTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
	},
}

// SingleFetchWithClient handles the messy business of the HTTP connection
// and is testable with dependency injection, called by SingleFetch
func SingleFetchWithClient(url string, c HTTPClient) (int, []byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		slog.Error("Fetch Error", slog.Any("Error", err))
		return 0, nil, err
	}

	// This io.ReadAll block does not have test coverage
	// Accepting this because of how difficult it is to mock
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Could not read body", slog.Any("Error", err))
		return 0, nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Close Error", slog.Any("Error", err))
			return
		}
	}()

	return resp.StatusCode, body, err
}

// SingleFetch returns the Response Code, raw byte stream body, and error
// This uses a Shared HTTP Client:
// - to reuse existing endpoint connections
// - to avoid stale connections that eat up OS FDs
func SingleFetch(url string) (int, []byte, error) {
	return SingleFetchWithClient(url, sharedHTTPClient)
}

// MetricKV streams input from the endpoint body and populates
// a map for all key/values, removing whitespace and comments
func MetricKV(d, url string) (map[string]string, error) {
	_, body, err := SingleFetch(url)
	if err != nil {
		return nil, err
	}
	return ParseMetricKV(bytes.NewReader(body), d)
}

func ParseMetricKV(reader io.Reader, d string) (map[string]string, error) {
	envMap := make(map[string]string)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// ignore whitespace and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on the delimiter /d/
		parts := strings.SplitN(line, d, 2)
		if len(parts) != 2 {
			slog.Error("WARNING: Invalid line", slog.String("line", line))
			continue
		}

		// Extract Key, Clean up Value, Add to Map
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes
		value = strings.Trim(value, `"'`)
		// Take care of any trailing quotes and comments
		if pos := strings.IndexAny(value, `"'#`); pos != -1 {
			value = value[:pos]
		}
		envMap[key] = value
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Problem scanning input", slog.Any("Error", err))
		return nil, fmt.Errorf("scanning error: %w", err)
	}

	return envMap, nil
}
