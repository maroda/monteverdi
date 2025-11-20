package monteverdi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
	"go.opentelemetry.io/otel"
)

// SetupMux handles all data serving:
// - Prometheus metric endpoint
// - Websocket specialized for D3.js UI
// - Version for programmatic use
// - Metrics Data for UI feedback
func (v *View) SetupMux() *mux.Router {
	r := mux.NewRouter()

	r.Handle("/metrics", v.Stats.Handler())
	r.HandleFunc("/conf", v.ConfHandler)
	r.HandleFunc("/ws", v.WebsocketHandler)
	r.HandleFunc("/api/version", v.VersionHandler)
	r.HandleFunc("/api/metrics-data", v.MetricsDataHandler)

	// Plugin controls
	r.PathPrefix("/api/plugin").HandlerFunc(v.PluginControlHandler)

	// HTML pages
	r.HandleFunc("/editor", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/value-editor.html")
	})

	// Static files for D3 frontend
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))

	api := r.PathPrefix("/api").Subrouter()
	api.Use(v.StatsMiddleware)

	return r
}

var Version = "dev"

// VersionHandler returns the current release version
// This must be compiled with:
//
//	go build -ldflags "-X github.com/maroda/monteverdi/display.Version=$(git describe --tags --always)"
func (v *View) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": Version})
}

// MetricsDataHandler returns a JSON blob with the full set of running metrics and systeminfo
func (v *View) MetricsDataHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := otel.Tracer("monteverdi/api").Start(ctx, "MetricsDataHandler")
	defer span.End()

	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	var allMetrics []MetricData

	for _, ep := range v.QNet.Network {
		ep.MU.RLock()
		for _, metricName := range ep.Metric {
			currentVal := ep.Mdata[metricName]
			maxVal := ep.Maxval[metricName]
			isAccent := currentVal >= maxVal
			percentUsed := 0.0
			if maxVal > 0 {
				percentUsed = Ms.FloatPrecise((float64(currentVal)/float64(maxVal))*100, 2)
			}

			allMetrics = append(allMetrics, MetricData{
				Endpoint:    ep.ID,
				Metric:      metricName,
				CurrentVal:  currentVal,
				MaxVal:      maxVal,
				IsAccent:    isAccent,
				PercentUsed: percentUsed,
			})
		}
		ep.MU.RUnlock()
	}

	systemInfo := &SystemInfo{}

	// Check for what value OutputType can be set
	if v.QNet.Output == nil {
		systemInfo.OutputType = "None"
	} else {
		systemInfo.OutputType = v.QNet.Output.Type()
		switch systemInfo.OutputType {
		case "MIDI":
			v.getMIDISystemInfo(systemInfo)
		}
	}

	// Smush the two structs together for a big JSON blob
	response := map[string]interface{}{
		"metrics": allMetrics,
		"system":  systemInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (v *View) PluginControlHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := otel.Tracer("monteverdi/api").Start(ctx, "PluginControlHandler")
	defer span.End()

	// This endpoint only allows POST
	if r.Method != http.MethodPost {
		span.RecordError(fmt.Errorf("invalid method: %s", r.Method))
		slog.Error("invalid method", slog.String("method", r.Method))
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}

	// If no output is configured, this endpoint throws an error
	var output Mp.OutputAdapter
	if output = v.QNet.Output; output == nil {
		span.RecordError(fmt.Errorf("no output configured"))
		slog.Error("no output configured")
		http.Error(w, "no output configured", http.StatusInternalServerError)
		return
	}

	// Parse control action from URI
	parts := strings.Split(r.URL.Path, "/")
	slog.Debug("parts: ", slog.Any("parts: ", parts))
	if len(parts) < 4 {
		span.RecordError(fmt.Errorf("invalid path: %s", r.URL.Path))
		slog.Error("Invalid plugin control URL: " + r.URL.Path)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Get API control from third part of URI
	control := parts[3]
	switch control {
	case "close":
		if c, ok := output.(interface{ Close() error }); ok {
			c.Close()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "OUTPUT CLOSED"})
		}
	case "flush":
		if f, ok := output.(interface{ Flush() error }); ok {
			f.Flush()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "WRITE CACHE FLUSHED"})
		}
	case "type":
		if t, ok := output.(interface{ Type() string }); ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(t.Type())
		}
	default:
		span.RecordError(fmt.Errorf("invalid control: %s", control))
		http.Error(w, "invalid control", http.StatusBadRequest)
	}
}

type MetricData struct {
	Endpoint    string  `json:"endpoint"`
	Metric      string  `json:"metric"`
	CurrentVal  int64   `json:"currentVal"`
	MaxVal      int64   `json:"maxVal"`
	IsAccent    bool    `json:"isAccent"`
	PercentUsed float64 `json:"percentUsed"`
}

type SystemInfo struct {
	OutputType  string `json:"outputType"`
	MIDIPort    string `json:"midiPort,omitempty"`
	MIDIChannel int    `json:"midiChannel"`
	MIDIRoot    int    `json:"midiRoot"`
	MIDIScale   string `json:"midiScale,omitempty"`
	MIDINotes   string `json:"midiNotes,omitempty"`
}
