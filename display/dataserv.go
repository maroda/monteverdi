package monteverdi

import (
	"encoding/json"
	"net/http"
)

func (v *View) metricsDataHandler(w http.ResponseWriter, r *http.Request) {
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
				percentUsed = (float64(currentVal) / float64(maxVal)) * 100
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
