package monteverdi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	webTimeout = 10 * time.Second
)

var url = "http://localhost:19999/api/v3/allmetrics"

// LightUp takes the output location as an io.Writer
// and uses this to display the results of the fetches
func LightUp(w io.Writer, m string) error {
	err := showBanner(w, m)
	if err != nil {
		slog.Error("Banner write error", slog.Any("Error", err))
		return err
	}

	Code, _, err := SingleFetch(url)
	if err != nil {
		slog.Error("Fetch Error", slog.Any("Error", err))
		return err
	}

	fmt.Printf("Status %d returned for Endpoint: %s\n", Code, url)
	return nil
}

func showBanner(w io.Writer, banner string) error {
	_, err := fmt.Fprintf(w, "%s\n", banner)
	return err
}

func SingleFetch(url string) (int, string, error) {
	client := &http.Client{Timeout: webTimeout}

	resp, err := client.Get(url)
	if err != nil {
		slog.Error("Fetch Error", slog.Any("Error", err))
		return 0, "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Close Error", slog.Any("Error", err))
			return
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Could not read body", slog.Any("Error", err))
		return 0, "", err
	}

	return resp.StatusCode, string(body), nil
}
