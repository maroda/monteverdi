package monteverdi

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
	Ms "github.com/maroda/monteverdi/server"
)

type ScreenViewer interface {
	drawHarmonyView()
	drawText()
	exit()
	handleKeyBoardEvent()
	resizeScreen()
	updateScreen()
}

// View is updated by whatever is in the QNet
type View struct {
	mu     sync.Mutex
	QNet   *Ms.QNet
	screen tcell.Screen
}

// Display text
func (v *View) drawText(x1, y1, x2, y2 int, text string) {
	row := y1
	col := x1
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	for _, r := range text {
		v.screen.SetContent(col, row, r, nil, style)
		col++
		if col >= x2 {
			row++
			col = x1
		}
		if row > y2 {
			break
		}
	}
}

// Draw the HarmonyView itself
func (v *View) drawHarmonyView() {
	width, height := 50, 10
	hvStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	/*
		v.screen.SetContent(0, 0, tcell.RuneVLine, nil, hvStyle)

		v.drawText(1, height+3, width, height+10, "Press ESC or Ctrl+C to quit")
	*/
	v.screen.SetContent(0, 0, tcell.RuneULCorner, nil, hvStyle)
	for i := 1; i < width; i++ {
		v.screen.SetContent(i, 0, tcell.RuneHLine, nil, hvStyle)
	}
	v.screen.SetContent(width, 0, tcell.RuneURCorner, nil, hvStyle)

	for i := 1; i < height; i++ {
		v.screen.SetContent(0, i, tcell.RuneVLine, nil, hvStyle)
	}

	v.screen.SetContent(0, height, tcell.RuneLLCorner, nil, hvStyle)

	for i := 1; i < height; i++ {
		v.screen.SetContent(width, i, tcell.RuneVLine, nil, hvStyle)
	}

	v.screen.SetContent(width, height, tcell.RuneLRCorner, nil, hvStyle)

	for i := 1; i < width; i++ {
		v.screen.SetContent(i, height, tcell.RuneHLine, nil, hvStyle)
	}

	v.drawText(20, height-8, width, height+10, fmt.Sprintf("CPU:%d", v.QNet.Network[0].Mdata["NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL"]))
	v.drawText(30, height-5, width, height+10, "CRAQUEMATTIC")
	v.drawText(1, height-1, width, height+10, "Press ESC or Ctrl+C to quit")
}

// Exit cleanly
func (v *View) exit() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.screen.Fini()
	os.Exit(0)
}

// Running Loop to handle events
func (v *View) handleKeyBoardEvent() {
	for {
		ev := v.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			v.resizeScreen()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				v.exit()
			}
		case *tcell.EventMouse:
			// i can't tell if this is working right...
			if err := v.pollQNet(); err != nil {
				slog.Error("Failed to poll QNet", slog.Any("Error", err))
				return
			}
			v.updateScreen()
		}
	}
}

// Resize for terminal changes
func (v *View) resizeScreen() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.screen.Sync()
}

func (v *View) pollQNet() error {
	// this poll will update the data for QNet
	// this will eventually be ALL the metrics
	_, err := v.QNet.Poll("NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL")
	if err != nil {
		slog.Error("Failed to poll QNet user", slog.Any("Error", err))
		return err
	}
	return nil
}

func (v *View) run() {
	if err := v.pollQNet(); err != nil {
		slog.Error("Failed to poll QNet", slog.Any("Error", err))
		return
	}
	v.updateScreen()
}

func (v *View) updateScreen() {
	v.screen.Clear()
	v.drawHarmonyView()
	v.screen.Show()
}

// NewView we'll see what this becomes
// **probably** QNet is sent here
func NewView(q *Ms.QNet) (*View, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		slog.Error("Could not get new screen", slog.Any("Error", err))
		return nil, err
	}
	if err := screen.Init(); err != nil {
		slog.Error("Could not initialize screen", slog.Any("Error", err))
		return nil, err
	}

	defStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	screen.SetStyle(defStyle)

	view := &View{
		QNet:   q,
		screen: screen,
	}

	view.updateScreen()

	return view, err
}

// StartHarmonyView is called by main to run the program.
func StartHarmonyView(q *Ms.QNet) error {
	view, err := NewView(q)
	if err != nil {
		slog.Error("Could not start HarmonyView", slog.Any("Error", err))
		return err
	}

	go view.run()
	view.handleKeyBoardEvent()
	return err
}
