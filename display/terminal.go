package monteverdi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/mux"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

type ScreenViewer interface {
	exit()
	handleKeyBoardEvent()
	resizeScreen()
	updateScreen()
}

// View is updated by whatever is in the QNet
type View struct {
	mu      sync.Mutex
	QNet    *Ms.QNet
	screen  tcell.Screen
	display []string
	stats   *Mo.StatsInternal
	server  *http.Server
}

// Display a single rune, probably a hex value?
// Like: '' - this is a private char, should use something else?
// for now just using it
// But for this to "stay on the screen", i need a running histogram
// to do that, i need a histogram type to use a cache in Accent
// this way, a timeseries of runes is beside a metric's accents
func (v *View) drawRune(x, y, m int) {
	color := tcell.NewRGBColor(int32(150+x), int32(150+y), int32(255-m))
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(color)
	v.screen.SetContent(x, y, '', nil, style)
}

func (v *View) drawTimeseries(x, y, i int, m string) {
	runes := v.QNet.Network[i].GetDisplay(m)

	for runeIndex, r := range runes { // Use a different variable name
		if r == 0 {
			r = ' '
		}

		// Choose color based on the rune (intensity)
		var style tcell.Style
		switch r {
		case '▁', '▂':
			style = tcell.StyleDefault.Foreground(tcell.ColorGreen)
		case '▃', '▄', '▅':
			style = tcell.StyleDefault.Foreground(tcell.ColorYellow)
		case '▆', '▇', '█':
			style = tcell.StyleDefault.Foreground(tcell.ColorRed)
		default:
			style = tcell.StyleDefault
		}

		v.screen.SetContent(x+runeIndex, y, r, nil, style) // Use runeIndex here
	}
}

// Display text
func (v *View) drawText(x1, y1, x2, y2 int, text string) {
	row := y1
	col := x1
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorLightSteelBlue)
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

// Display the outline of the View
func (v *View) drawViewBorder(width, height int) {
	hvStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
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
}

// drawBar shows a long bar for the amount entered
// x1 = starting X axis (from left), x2 = ending X axis (from left)
// y1 = starting Y axis (from top), y2 = ending Y axis (from top)
func (v *View) drawBar(x1, y1, x2, y2 int) {
	// barStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	for row := y1; row < y2; row++ {
		for col := x1; col < x2; col++ {
			// color := tcell.NewRGBColor(int32(50+row), int32(50+col), int32(50+row))
			color := tcell.NewRGBColor(int32(80+row), int32(80+col), int32(250+row))
			barStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(color)
			v.screen.SetContent(col, row, '', nil, barStyle)
		}
	}
}

// Draw the HarmonyView itself
func (v *View) drawHarmonyViewMulti() {
	// This is the border of the box
	width, height := 80, 20

	// Draw basic elements
	v.drawViewBorder(width, height)
	v.drawText(1, height-1, width, height+10, "Press ESC or Ctrl+C to quit")

	// step through all Network endpoints
	for ni := range v.QNet.Network {
		// read lock
		v.QNet.Network[ni].MU.RLock()

		// step through metrics listed in View.display
		// ranging on the global display variable is INCORRECT here!
		// i should be ranging on THIS endpoint's metrics
		// for di, dm := range v.display {
		for di, dm := range v.QNet.Network[ni].Metric {
			// look up the key in this Network's Endpoint Metric data.
			// For now, we're pulling raw data,
			// but future this will be only Accents
			ddm := v.QNet.Network[ni].Mdata[dm]

			// Calculate unique y position for each endpoint/metric combination
			yTS := 6 + (ni * len(v.display)) + di

			// draw timeseries - each endpoint gets its own line
			v.drawTimeseries(1, yTS, ni, dm)

			// it will take some experimentation to align...
			v.drawText(2, yTS+3, width+di, height+10+di, fmt.Sprintf("%s:%d", dm, ddm))

			// draw the bar
			v.drawBar(1, 1+di, int(ddm), 2+di)

			// Can we see an Accent happen?
			dda := v.QNet.Network[ni].Accent[dm]
			if dda != nil {
				// now get the second from the Timestamp. this is the X position on the display
				newTime := time.Unix(dda.Timestamp/1e9, dda.Timestamp%1e9)
				s := newTime.Second()

				// draw a rune
				v.drawRune(s, di+1, int(ddm))
			}
		}

		v.QNet.Network[ni].MU.RUnlock()
	}

	v.drawText(width-20, height-1, width, height+10, "MONTEVERDI")
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
		}
	}
}

// PollQNetAll is for reading the multi metric config in Endpoint
func (v *View) PollQNetAll() error {
	start := time.Now()

	err := v.QNet.PollMulti()
	if err != nil {
		slog.Error("Failed to PollMulti", slog.Any("Error", err))
	}

	duration := time.Since(start).Seconds()
	v.stats.RecPollTimer(duration)

	return err
}

// Resize for terminal changes
func (v *View) resizeScreen() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.screen.Sync()
}

// run runs a loop and updates periodically
// each iteration polls the configured Metric[]
// and fills the related Mdata[Metric] in QNet,
// which is then read by drawHarmonyViewMulti
// TODO: parameterize run loop time
func (v *View) run() {
	slog.Info("Starting HarmonyView")
	for {
		time.Sleep(1 * time.Second)
		if err := v.PollQNetAll(); err != nil {
			slog.Error("Failed to poll QNet", slog.Any("Error", err))
			return
		}
		v.updateScreen()
	}
}

func (v *View) setupMux() *mux.Router {
	r := mux.NewRouter()

	r.Handle("/metrics", v.stats.Handler())

	api := r.PathPrefix("/api").Subrouter()
	api.Use(v.statsMiddleware)

	return r
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// optional for non header
func (w *responseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func (v *View) statsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         200,
		}
		next.ServeHTTP(wrapped, r)

		// duration := time.Since(start).Seconds()
		v.stats.RecWWW(strconv.Itoa(wrapped.status), r.Method)
	})
}

func (v *View) updateScreen() {
	v.screen.Clear()
	// v.drawHarmonyView()
	v.drawHarmonyViewMulti()
	v.screen.Show()
}

// NewView creates the tcell screen that displays HarmonyView
func NewView(q *Ms.QNet) (*View, error) {
	if q == nil || q.Network == nil {
		slog.Error("Could not get a QNet for display")
		return nil, errors.New("quality network not found")
	}

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

	// Get all configured metrics from all Endpoints
	display := make([]string, 0)
	for _, ep := range q.Network {
		for _, mv := range ep.Metric {
			display = append(display, mv)
		}
	}

	// create an attached prometheus registry
	stats := Mo.NewStatsInternal()

	view := &View{
		QNet:    q,
		screen:  screen,
		display: display, // something is overranging this display slice!
		stats:   stats,
	}

	view.updateScreen()

	return view, err
}

// StartHarmonyViewWithConfig is called by main to run the program.
// This also starts up the /metrics endpoint that is populated by prometheus.
func StartHarmonyViewWithConfig(c []Ms.ConfigFile) error {
	// with the new config c, we can make other stuff
	// var eps *Ms.Endpoints
	eps, err := Ms.NewEndpointsFromConfig(c)
	if eps == nil || err != nil {
		slog.Error("Failed to init endpoints", slog.Any("Error", err))
		return err
	}

	qn := Ms.NewQNet(*eps)
	if qn == nil {
		slog.Error("Failed to init QNet", slog.Any("Error", err))
	}

	view, err := NewView(qn)
	if err != nil {
		slog.Error("Could not start HarmonyView", slog.Any("Error", err))
		return err
	}

	// Server for stats endpoint
	view.server = &http.Server{
		Addr:    ":8080",
		Handler: view.setupMux(),
	}

	// Run Monteverdi
	go view.run()

	// Run stats endpoint
	go func() {
		addr := ":8080"
		slog.Info("Starting Monteverdi stats endpoint...", slog.String("Port", addr))
		if err := view.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Could not start stats endpoint", slog.Any("Error", err))
		}
	}()

	view.handleKeyBoardEvent()

	return err
}
