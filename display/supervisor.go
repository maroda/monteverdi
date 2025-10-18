package monteverdi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

type PollSupervisor struct {
	View     *View
	Ticker   *time.Ticker
	StopChan chan struct{}
	WG       sync.WaitGroup
}

// NewPollSupervisor is a wrapper around the View that manages polling goroutines
// They are strongly coupled, one knows about the other
func (v *View) NewPollSupervisor() *PollSupervisor {
	ps := &PollSupervisor{
		View: v,
	}
	v.Supervisor = ps
	return ps
}

// ReloadConfig performs an automatic restart after filling QNet with the new config
func (v *View) ReloadConfig(c []Ms.ConfigFile) {
	v.Supervisor.Stop()

	// Build new endpoints from config
	// and replace the existing QNet
	eps := Ms.NewEndpointsFromConfig(c)
	v.QNet.MU.Lock()
	v.QNet.Network = *eps
	v.QNet.MU.Unlock()

	v.Supervisor.Start()
}

// ConfHandler receives the new JSON config, validates, and reloads
func (v *View) ConfHandler(w http.ResponseWriter, r *http.Request) {
	configPath := v.ConfigPath

	switch r.Method {
	case "GET":
		loadConfig, err := Ms.LoadConfigFileName(configPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to load new config: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loadConfig)
	case "POST":
		// Read JSON body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate JSON
		var testConfig []Ms.ConfigFile
		if err = json.Unmarshal(body, &testConfig); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		}

		// Write JSON to disk
		if err = os.WriteFile(configPath, body, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write new config: %v", err), http.StatusInternalServerError)
			return
		}

		// Load config and restart like normal
		loadConfig, err := Ms.LoadConfigFileName(configPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to load new config: %v", err), http.StatusInternalServerError)
			return
		}

		// Reload with new config
		v.ReloadConfig(loadConfig)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Configuration reloaded",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

// Start the PollSupervisor
func (p *PollSupervisor) Start() {
	p.StopChan = make(chan struct{})
	p.Ticker = time.NewTicker(1 * time.Second)

	p.WG.Add(1)
	go func() {
		defer p.WG.Done()
		defer p.Ticker.Stop()

		for {
			select {
			case <-p.Ticker.C:
				p.View.PollQNetAll()
			case <-p.StopChan:
				return
			}
		}
	}()
}

// Stop the PollSupervisor
func (p *PollSupervisor) Stop() {
	if p.StopChan != nil {
		close(p.StopChan)
		p.WG.Wait()
	}
}

// Restart the PollSupervisor
func (p *PollSupervisor) Restart() {
	p.Stop()
	p.Start()
}
