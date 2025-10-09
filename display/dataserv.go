package monteverdi

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	Ms "github.com/maroda/monteverdi/server"
)

// SetupMux handles all data serving:
// - Prometheus metric endpoint
// - Websocket specialized for D3.js UI
// - Version for programmatic use
// - Metrics Data for UI feedback
func (v *View) SetupMux() *mux.Router {
	r := mux.NewRouter()

	r.Handle("/metrics", v.Stats.Handler())
	r.HandleFunc("/ws", v.WebsocketHandler)
	r.HandleFunc("/api/version", v.VersionHandler)
	r.HandleFunc("/api/metrics-data", v.MetricsDataHandler)

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allMetrics)
}
