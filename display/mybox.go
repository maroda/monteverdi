package monteverdi

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

/*

This snake game does something like I'm thinking about:
https://github.com/liweiyi88/gosnakego/blob/main/snake/game.go

- Create a struct that contains the current harmonic view or whatever it is.
- This will contain the Screen and other configs
- Use interface methods to work on the data

*/

/*

With netdata installed, I can see all 13k MacOS metrics here:
curl -s 'http://localhost:19999/api/v3/allmetrics'

Here's one that changes pretty often:
curl -s 'http://localhost:19999/api/v3/allmetrics' | grep CPU_UTILIZATION_VISIBLE | grep -v "=\"0\""

Use this to set all of them:
eval "$(curl -s 'http://localhost:19999/api/v3/allmetrics')"

*/

func GetTTY() (tcell.Screen, error) {
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)

	// New screen
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Could not get new screen: %+v", err)
	}

	// Initialize screen
	if err := s.Init(); err != nil {
		log.Fatalf("Could not initialize screen: %+v", err)
		os.Exit(1)
	}
	s.SetStyle(defStyle)
	s.EnableMouse()
	s.EnablePaste()
	s.Clear()

	return s, err
}

// WriteBar shows a long bar for the amount entered
// x1 = starting X axis (from left), x2 = ending X axis (from left)
// y1 = starting Y axis (from top), y2 = ending Y axis (from top)
func WriteBar(s tcell.Screen, x1, y1, x2, y2 int, style tcell.Style) {
	for row := y1; row < y2; row++ {
		for col := x1; col < x2; col++ {
			s.SetContent(col, row, ' ', nil, style)
		}
	}
}
