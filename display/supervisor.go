package monteverdi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	Ms "github.com/maroda/monteverdi/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// PollSupervisor is a wrapper around the View that manages polling goroutines
// They are strongly coupled, one knows about the other
type PollSupervisor struct {
	View    *View
	Pollers []*EndpointPoller
	WG      sync.WaitGroup
}

// EndpointPoller manages per-endpoint polling
type EndpointPoller struct {
	QNet     *Ms.QNet
	Index    int          // Index in QNet.Network
	Ticker   *time.Ticker // Frequency
	StopChan chan struct{}
}

// NewPollSupervisor creates a new supervisor for all endpoints
func (v *View) NewPollSupervisor() *PollSupervisor {
	ps := &PollSupervisor{
		View:    v,
		Pollers: make([]*EndpointPoller, len(v.QNet.Network)),
	}

	// Create a poller for each endpoint
	for i := range v.QNet.Network {
		ps.Pollers[i] = &EndpointPoller{
			QNet:  v.QNet,
			Index: i,
		}
	}

	return ps
}

// ReloadConfig performs an automatic restart after filling QNet with the new config
func (v *View) ReloadConfig(ctx context.Context, c []Ms.ConfigFile) {
	ctx, span := otel.Tracer("monteverdi/supervisor").Start(ctx, "ReloadConfig")
	defer span.End()

	// Lock the View
	v.MU.Lock()
	defer v.MU.Unlock()

	// Stop current polling
	if v.Supervisor != nil {
		v.Supervisor.Stop()
	}

	// Build new endpoints from config
	// and replace the existing QNet
	eps := Ms.NewEndpointsFromConfig(c)
	v.QNet = Ms.NewQNet(*eps)

	// Create and start new supervisor
	v.Supervisor = v.NewPollSupervisor()
	v.Supervisor.Start()

	slog.Info("Config reloaded and polling restarted!")
}

// ConfHandler receives the new JSON config, validates, and reloads
func (v *View) ConfHandler(w http.ResponseWriter, r *http.Request) {
	configPath := v.ConfigPath

	switch r.Method {
	case "GET":
		ctx := r.Context()
		ctx, span := otel.Tracer("monteverdi/conf").Start(ctx, "ConfHandlerGet")
		defer span.End()

		loadConfig, err := Ms.LoadConfigFileName(configPath)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf("Failed to load new config: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loadConfig)
	case "POST":
		ctx := r.Context()
		ctx, span := otel.Tracer("monteverdi/conf").Start(ctx, "ConfHandlerPost")
		defer span.End()

		// Read JSON body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate JSON
		var testConfig []Ms.ConfigFile
		if err = json.Unmarshal(body, &testConfig); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		}

		// Write JSON to disk
		if err = os.WriteFile(configPath, body, 0644); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf("Failed to write new config: %v", err), http.StatusInternalServerError)
			return
		}

		// TODO: consider LoadConfigFileName as a method of an interface to inject for testing this
		// Load config and restart like normal
		loadConfig, err := Ms.LoadConfigFileName(configPath)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf("Failed to load new config: %v", err), http.StatusInternalServerError)
			return
		}

		// Reload with new config
		v.ReloadConfig(ctx, loadConfig)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Configuration reloaded",
		})

		slog.Info("Configuration reloaded", slog.String("path", configPath))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		slog.Error("Method not allowed", slog.String("method", r.Method))
		return
	}
}

// Start the PollSupervisor
func (ps *PollSupervisor) Start() {
	slog.Info("Starting Poll Supervisor", slog.Int("endpoints", len(ps.Pollers)))

	for _, poller := range ps.Pollers {
		ps.startEndpointPoller(poller)
	}
}

func (ps *PollSupervisor) startEndpointPoller(poller *EndpointPoller) {
	interval := poller.QNet.Network[poller.Index].Interval
	if interval == 0 {
		slog.Warn("Poller interval is 0, using default of 15s")
		interval = 15 * time.Second
	}

	poller.Ticker = time.NewTicker(interval)
	poller.StopChan = make(chan struct{})

	ps.WG.Add(1)
	go func() {
		defer ps.WG.Done()
		defer poller.Ticker.Stop()

		epID := poller.QNet.Network[poller.Index].ID
		slog.Info("Endpoint poller started",
			slog.String("endpoint", epID),
			slog.Duration("interval", interval))

		for {
			select {
			case <-poller.Ticker.C:
				start := time.Now()
				ctx := context.Background()
				ctx, span := otel.Tracer("monteverdi/supervisor").Start(ctx, "PollEndpoint")
				defer span.End()

				span.SetAttributes(
					attribute.String("endpoint", epID),
					attribute.Int("interval.sec", int(interval)),
				)

				poller.QNet.PollEndpoint(poller.Index)

				ps.View.Stats.RecPollTimer(time.Since(start).Seconds())
			case <-poller.StopChan:
				slog.Info("Endpoint poller stopped", slog.String("endpoint", epID))
				return
			}
		}
	}()
}

// Stop the PollSupervisor
// This is idempotent and will run even if stopped
func (ps *PollSupervisor) Stop() {
	slog.Info("Stopping Poll Supervisor")

	for _, poller := range ps.Pollers {
		if poller.StopChan != nil {
			select {
			case <-poller.StopChan: // Already closed, noop
			default:
				close(poller.StopChan)
			}
		}
	}

	ps.WG.Wait()
	slog.Info("All endpoint pollers stopped", slog.Int("endpoints", len(ps.Pollers)))
}

// Restart the PollSupervisor
func (ps *PollSupervisor) Restart() {
	slog.Info("Restarting Poll Supervisor")
	ps.Stop()
	ps.Start()
}
