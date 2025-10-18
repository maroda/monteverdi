package monteverdi

import (
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
