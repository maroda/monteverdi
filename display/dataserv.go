package monteverdi

import (
	"encoding/json"
	"fmt"
	"net/http"

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

func (v *View) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": Version})
}

func (v *View) MetricsDataHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := otel.Tracer("monteverdi/api").Start(ctx, "MetricsDataHandler")
	defer span.End()

	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	type MetricData struct {
		Endpoint    string  `json:"endpoint"`
		Metric      string  `json:"metric"`
		CurrentVal  int64   `json:"currentVal"`
		MaxVal      int64   `json:"maxVal"`
		IsAccent    bool    `json:"isAccent"`
		PercentUsed float64 `json:"percentUsed"`
	}

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

	type SystemInfo struct {
		OutputType  string `json:"outputType"`
		MIDIPort    string `json:"midiPort,omitempty"`
		MIDIChannel int    `json:"midiChannel"`
		MIDIRoot    int    `json:"midiRoot"`
		MIDIScale   string `json:"midiScale,omitempty"`
		MIDINotes   string `json:"midiNotes,omitempty"`
	}

	systemInfo := SystemInfo{
		OutputType: "None",
	}

	if v.QNet.Output != nil {
		systemInfo.OutputType = v.QNet.Output.Type()

		// If the output type is MIDI, fill in the details
		if midiOut, ok := v.QNet.Output.(*Mp.MIDIOutput); ok {
			systemInfo.MIDIPort = midiOut.Port.String()
			systemInfo.MIDIChannel = int(midiOut.Channel)
			systemInfo.MIDIRoot = int(midiOut.Root)
			systemInfo.MIDIScale = fmt.Sprint(midiOut.Scale)
			systemInfo.MIDINotes = fmt.Sprint(midiOut.ScNotes)
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
