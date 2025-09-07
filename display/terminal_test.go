package monteverdi_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	Ms "github.com/maroda/monteverdi/server"
)

const (
	id   = "craquemattic"
	url  = "https://popg.xyz"
	stat = "CPU"
)

var ep *Ms.Endpoint
var eps []Ms.Endpoint

func TestNewView(t *testing.T) {
	ep = Ms.NewEndpoint(id, url, stat)
	eps = append(eps, *ep)
	// qn := Ms.QNet{Network: eps}

	t.Run("Returns correct field count", func(t *testing.T) {
	})
}

// it's probably not right to have a func that creates a screen
// Instead, the screen would be mocked as part of testing an interface
func TestScreen(t *testing.T) {
	s := mkTestScreen(t, "")
	defer s.Fini()
	s.Clear()

	t.Run("Check test screen", func(t *testing.T) {
		b, x, y := s.GetContents()
		if len(b) != x*y || x != 80 || y != 25 {
			t.Fatalf("Contents (%v, %v, %v) wrong", len(b), x, y)
		}
		for i := 0; i < x*y; i++ {
			if len(b[i].Runes) == 1 && b[i].Runes[0] != ' ' {
				t.Errorf("Incorrect contents at %v: %v", i, b[i].Runes)
			}
			if b[i].Style != tcell.StyleDefault {
				t.Errorf("Incorrect style at %v: %v", i, b[i].Style)
			}
		}
	})
}

func TestStartHarmonyView(t *testing.T) {
}

func mkTestScreen(t *testing.T, charset string) tcell.SimulationScreen {
	s := tcell.NewSimulationScreen(charset)
	if s == nil {
		t.Fatalf("Failed to get SimulationScreen")
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init screen: %v", err)
	}
	return s
}
