package monteverdi

import (
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type StatsInternal struct {
	WWWRegistry *prometheus.Registry
	WWWStats    *prometheus.CounterVec
	PollSingle  prometheus.Counter
	PollTimer   prometheus.Histogram
}

func NewStatsInternal() *StatsInternal {
	si := &StatsInternal{
		WWWRegistry: prometheus.NewRegistry(),
	}

	// Custom Go Runtime collector
	goCollector := collectors.NewGoCollector(
		collectors.WithGoCollectorRuntimeMetrics(
			collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")},
		),
	)
	si.WWWRegistry.MustRegister(goCollector)
	si.WWWRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Create metrics and references
	si.WWWStats = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_inbound_total"},
		[]string{"code", "method"},
	)
	si.WWWRegistry.MustRegister(si.WWWStats)

	si.PollSingle = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "poll_requests_total"})
	si.WWWRegistry.MustRegister(si.PollSingle)

	si.PollTimer = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "poll_requests_seconds"})
	si.WWWRegistry.MustRegister(si.PollTimer)

	return si
}

func (si *StatsInternal) RecWWW(statusCode, method string) {
	si.WWWStats.WithLabelValues(statusCode, method).Inc()
}

func (si *StatsInternal) RecPollSingle() {
	si.PollSingle.Inc()
}

func (si *StatsInternal) RecPollTimer(duration float64) {
	si.PollTimer.Observe(duration)
	si.PollSingle.Inc()
}

func (si *StatsInternal) Handler() http.Handler {
	return promhttp.HandlerFor(si.WWWRegistry, promhttp.HandlerOpts{})
}
