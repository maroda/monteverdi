package monteverdi

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	webTimeout = 10 * time.Second
)

// SingleFetch returns the Response Code, raw byte stream body, and error
func SingleFetch(url string) (int, []byte, error) {
	client := &http.Client{Timeout: webTimeout}

	resp, err := client.Get(url)
	if err != nil {
		slog.Error("Fetch Error", slog.Any("Error", err))
		return 0, nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Could not read body", slog.Any("Error", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Close Error", slog.Any("Error", err))
			return
		}
	}()

	return resp.StatusCode, body, err
}

// MetricKV streams input from the endpoint body and populates
// a map for all key/values, removing whitespace and comments
// TODO: in order to support prometheus, we need a custom delimiter
func MetricKV(url string) (map[string]string, error) {
	envMap := make(map[string]string)

	// Grab the body of the given URL, which is a known KV output
	// in the format "KEY=VALUE" similar to a .env file
	// This streams the result, which can be large from some sources.
	_, body, err := SingleFetch(url)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// ignore whitespace and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on the assignment operator
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
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
	}

	return envMap, err
}
