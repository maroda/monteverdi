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

const (
	screenWidth  = 80
	screenHeight = 20
	screenGutter = 4
)

type ScreenViewer interface {
	exit()
	handleKeyBoardEvent()
	resizeScreen()
	updateScreen()
}

// View is updated by whatever is in the QNet
type View struct {
	mu          sync.Mutex
	QNet        *Ms.QNet          // Quality Network
	screen      tcell.Screen      // the screen itself
	display     []string          // rune display sequence
	stats       *Mo.StatsInternal // Internal status for prometheus
	server      *http.Server      // Prometheus metrics server
	selectEP    int               // Selected Endpoint with MouseClick
	showEP      bool              // Display Endpoint ID
	selectMe    string            // Selected Metric with MouseClick
	showMe      bool              // Display Metric ID
	showPulse   bool              // Display pulse view overlay
	pulseFilter *Ms.PulsePattern  // For filtering the display
}

// Figure out where to draw the next Timeseries entry on the graph
func (v *View) calcTimeseriesY(endpointIndex, metricIndex, gutter int) int {
	metricCount := len(v.QNet.Network[endpointIndex].Metric)
	return gutter + (endpointIndex * metricCount) + metricIndex
}

////////// PULSE VIS

func (v *View) calcTimePos(pulseStartTime time.Time) int {
	// Convert pulse timestamp to position on 60-character timeline
	// Assuming timeline shows last 60 seconds, rightmost = most recent
	now := time.Now()
	secondsAgo := int(now.Sub(pulseStartTime).Seconds())

	// Timeline position (0-59, where 59 is most recent)
	position := 59 - secondsAgo
	if position < 0 {
		position = 0 // Pulse started before visible window
	}
	return position
}

func (v *View) calcDurationWidth(duration time.Duration) int {
	// Convert pulse duration to character width on timeline
	durationSeconds := int(duration.Seconds())

	// Cap at reasonable width to prevent overflow
	if durationSeconds > 59 {
		durationSeconds = 59
	}
	return durationSeconds
}

func (v *View) getAccentStateAtPos(pulse Ms.PulseEvent, pos int) bool {
	// Determine if this timeline position represents accent or non-accent
	// Based on the pulse pattern and position within the pulse span

	switch pulse.Pattern {
	case Ms.Iamb:
		// Iamb: non-accent → accent
		// First part is non-accent, second part is accent
		midPoint := v.calcTimePos(pulse.StartTime) + (v.calcDurationWidth(pulse.Duration) / 2)
		return pos >= midPoint

	case Ms.Trochee:
		// Trochee: accent → non-accent
		// First part is accent, second part is non-accent
		midPoint := v.calcTimePos(pulse.StartTime) + (v.calcDurationWidth(pulse.Duration) / 2)
		return pos < midPoint
	}

	return false
}

func (v *View) getPulseRune(pattern Ms.PulsePattern, isAccent bool) (rune, tcell.Style) {
	var baseColor tcell.Color
	var symbol rune

	// Pattern type determines baseColor
	switch pattern {
	case Ms.Iamb:
		baseColor = tcell.ColorMaroon
		symbol = '⚍'
	case Ms.Trochee:
		baseColor = tcell.ColorDarkOrange
		symbol = '⚎'
	}

	// Shade based on accent state
	var style tcell.Style
	if isAccent {
		// Saturated color for accents
		style = tcell.StyleDefault.Foreground(baseColor)
	} else {
		// Desaturated color for non-accents
		style = tcell.StyleDefault.Foreground(baseColor).Dim(true)
	}

	return symbol, style
}

func (v *View) renderPulseViz(x, y int, tld []Ms.PulseVizPoint) {
	for _, point := range tld {
		symbol, style := v.getPulseRune(point.Pattern, point.IsAccent)
		v.screen.SetContent(x+point.Position, y, symbol, nil, style)
	}
}

func (v *View) drawPulseView() {
	// Clear or dim the background first
	// v.drawPulseBackground()

	// Show current filter mode
	filterText := "All Patterns"
	if v.pulseFilter != nil {
		switch *v.pulseFilter {
		case Ms.Iamb:
			filterText = "Iamb Only"
		case Ms.Trochee:
			filterText = "Trochee Only"
		}
	}

	v.drawText(1, 1, 70, 2, fmt.Sprintf("PULSE VIEW - %s (triple ictus analysis)", filterText))
	v.drawText(1, 2, 70, 3, "i=Iamb | t=Trochee | x=All | ► stacked long pulses ◄ ")

	// Draw pulse visualization for each endpoint/metric
	for ni := range v.QNet.Network {
		for di, dm := range v.QNet.Network[ni].Metric {
			yTS := v.calcTimeseriesY(ni, di, screenGutter)

			// Pass the filter for display
			timelineData := v.QNet.Network[ni].GetPulseVizData(dm, v.pulseFilter)
			v.renderPulseViz(1, yTS, timelineData)

			// Track long pulse boundaries
			var longPulseStart, longPulseEnd int = -1, -1
			now := time.Now()

			for _, point := range timelineData {
				pulseAge := now.Sub(point.StartTime).Seconds()

				// Find pulses that started before our 60-second window
				if pulseAge > 60 {
					if longPulseStart == -1 || point.Position < longPulseStart {
						longPulseStart = point.Position
					}
					if longPulseEnd == -1 || point.Position > longPulseEnd {
						longPulseEnd = point.Position
					}
				}
			}

			// Mark start and end of frozen section
			if longPulseStart >= 0 {
				leftStyle := tcell.StyleDefault.Foreground(tcell.ColorDodgerBlue)
				v.screen.SetContent(1+longPulseStart, yTS, '►', nil, leftStyle)
			}

			if longPulseEnd >= 0 && longPulseEnd != longPulseStart {
				rightStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkTurquoise)
				v.screen.SetContent(1+longPulseEnd, yTS, '◄', nil, rightStyle)
			}
		}
	}

	// Add pulse view indicator
	v.drawText(1, 1, 20, 2, "PULSE VIEW")
}

func (v *View) drawPulseBackground() {
	width, height := 80, 20

	// Dim the existing content
	for y := 2; y < height-2; y++ {
		for x := 1; x < width-1; x++ {
			// Get current content and dim it
			mainc, combc, style, _ := v.screen.GetContent(x, y)
			dimmedStyle := style.Background(tcell.ColorBlack).Foreground(tcell.ColorDarkGray)
			v.screen.SetContent(x, y, mainc, combc, dimmedStyle)
		}
	}
}

////////// PULSE VIS ^^^^^

// place a single '' on the screen
// used to draw the accents/second indicator
func (v *View) drawRune(x, y, m int) {
	color := tcell.NewRGBColor(int32(150+x), int32(150+y), int32(255-m))
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(color)
	v.screen.SetContent(x, y, '', nil, style)
}

// Display the current Timeseries data for a metric
func (v *View) drawTimeseries(x, y, i int, m string) {
	runes := v.QNet.Network[i].GetDisplay(m)

	for runeIndex, r := range runes { // Use a different variable name
		if r == 0 {
			r = ' '
		}

		// Choose color based on the rune (intensity)
		var style tcell.Style
		switch r {
		case '▁':
			style = tcell.StyleDefault.Foreground(tcell.ColorSeaGreen)
		case '▂':
			style = tcell.StyleDefault.Foreground(tcell.ColorMediumSeaGreen)
		case '▃':
			style = tcell.StyleDefault.Foreground(tcell.ColorLightSeaGreen)
		case '▄':
			style = tcell.StyleDefault.Foreground(tcell.ColorDarkTurquoise)
		case '▅':
			style = tcell.StyleDefault.Foreground(tcell.ColorMediumTurquoise)
		case '▆':
			style = tcell.StyleDefault.Foreground(tcell.ColorTurquoise)
		case '▇':
			style = tcell.StyleDefault.Foreground(tcell.ColorLightGreen)
		case '█':
			style = tcell.StyleDefault.Foreground(tcell.ColorAquaMarine)
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
// Includes a toggle for view mode
func (v *View) drawHarmonyViewMulti() {
	// This is the border of the box
	width, height := screenWidth, screenHeight

	// Draw basic elements
	v.drawViewBorder(width, height)

	// Support toggle to pulse view by wrapping in a boolean
	if v.showPulse {
		v.drawPulseView()

		// A MouseClick has happened on a graph
		// - show the Endpoint ID at the bottom
		// - show the metric name and value to the side
		if v.showEP {
			v.showEndpoint(40, 1)
		}
		if v.showMe {
			v.showMetric(2, screenGutter, 0, 4)
		}

		v.drawText(1, height-1, width, height+10, "/p/ to exit | /ESC/ to quit")
	} else {
		// step through all Network endpoints
		for ni := range v.QNet.Network {
			// read lock
			v.QNet.Network[ni].MU.RLock()

			// step through metrics listed in View.display
			for di, dm := range v.QNet.Network[ni].Metric {
				// look up the key in this Network's Endpoint Metric data.
				ddm := v.QNet.Network[ni].Mdata[dm]

				// Calculate unique y position for each endpoint/metric combination
				yTS := v.calcTimeseriesY(ni, di, screenGutter)

				// draw timeseries - each endpoint gets its own line
				v.drawTimeseries(1, yTS, ni, dm)

				// See an Accent happen
				dda := v.QNet.Network[ni].Accent[dm]
				if dda != nil {
					// now get the second from the Timestamp. this is the X position on the display
					newTime := time.Unix(dda.Timestamp/1e9, dda.Timestamp%1e9)
					s := newTime.Second()

					// draw a rune across the top
					// v.drawRune(s, di+1, int(ddm))
					v.drawRune(s, 1, int(ddm))
				}
			}

			// A MouseClick has happened on a graph, show the metric name and value
			if v.showMe {
				for ni := range v.QNet.Network {
					if ni == v.selectEP {
						for di, dm := range v.QNet.Network[ni].Metric {
							if dm == v.selectMe {
								yTS := v.calcTimeseriesY(ni, di, screenGutter)

								mdata := v.QNet.Network[ni].Mdata[dm]
								label := fmt.Sprintf("... %s ...", dm) // The Metric
								data := fmt.Sprintf("%d", mdata)       // The raw data
								v.drawText(62, yTS, width, yTS, data)
								v.drawText(4, height-2, width, height-2, label)
							}
						}
					}
				}
			}

			// A MouseClick has happened on a graph, show the Endpoint ID at the bottom
			if v.showEP {
				v.showEndpoint(40, 1)
			}

			v.QNet.Network[ni].MU.RUnlock()
		}

		v.drawText(1, height-1, width, height+10, "/p/ for pulses | /ESC/ to quit")
	}

	v.drawText(width-12, height-1, width, height+10, "MONTEVERDI")
}

// g = gutter, typically screenGutter
// dx = start metric (was 62)
// lx = start label (was 4)
// by = y offset from the bottom (was 2)
func (v *View) showMetric(by, g, dx, lx int) {
	width, height := screenWidth, screenHeight
	for ni := range v.QNet.Network {
		if ni == v.selectEP {
			for di, dm := range v.QNet.Network[ni].Metric {
				if dm == v.selectMe {
					yTS := v.calcTimeseriesY(ni, di, g)
					mdata := v.QNet.Network[ni].Mdata[dm]
					data := fmt.Sprintf("%d", mdata)       // The raw data
					label := fmt.Sprintf("... %s ...", dm) // The Metric

					// Turn off drawing raw metrics by using dx=0
					if dx != 0 {
						v.drawText(dx, yTS, width, yTS, data)
					}
					v.drawText(lx, height-by, width, height-by, label)
				}
			}
		}
	}
}

// x = normal x
// by = y offset from the bottom
func (v *View) showEndpoint(x, by int) {
	width, height := screenWidth, screenHeight
	epName := v.QNet.Network[v.selectEP].ID
	v.drawText(x, height-by, width, height, fmt.Sprintf("|  Polling: %s  |", epName))
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
			// Catch quit and exit
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				v.exit()
			}

			// Toggle pulse view with 'p'
			if ev.Rune() == 'p' {
				v.mu.Lock()
				v.showPulse = !v.showPulse
				/*
					if !v.showPulse {
						v.pulseFilter = nil // reset filter when exiting pulse view
					}
				*/
				v.mu.Unlock()
			}

			// Pattern filtering (only when in pulse view)
			if v.showPulse {
				switch ev.Rune() {
				case 'i':
					v.mu.Lock()
					iamb := Ms.Iamb
					v.pulseFilter = &iamb
					v.mu.Unlock()
				case 't':
					v.mu.Lock()
					trochee := Ms.Trochee
					v.pulseFilter = &trochee
					v.mu.Unlock()
				case 'x':
					v.mu.Lock()
					v.pulseFilter = nil // Show all patterns
					v.mu.Unlock()
				}
			}

		case *tcell.EventMouse:
			// Button1 is Left Mouse Button
			if ev.Buttons() == tcell.Button1 {
				v.handleMouseClick(ev.Position())
			}
		}
	}
}

func (v *View) handleMouseClick(x, y int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Assume there is no label so the last one is cleared.
	v.showEP = false
	v.showMe = false

	// Check for a click on any timeseries graph
	for ni := range v.QNet.Network {
		for di, dm := range v.QNet.Network[ni].Metric {
			// yTS is the same as drawHarmonyViewMulti
			yTS := v.calcTimeseriesY(ni, di, screenGutter)

			// Check if click is on this timeseries line
			// Timeseries spans x=1 to x=60
			// Exit if a match is found
			if y == yTS && x >= 1 && x <= 60 {
				v.selectEP = ni
				v.selectMe = dm
				v.showEP = true
				v.showMe = true
				return
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

	// Define and configure the default screen
	defStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	screen.SetStyle(defStyle)
	screen.EnableMouse()

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
		Addr:    ":8090",
		Handler: view.setupMux(),
	}

	// Run Monteverdi
	go view.run()

	// Run stats endpoint
	go func() {
		addr := ":8090"
		slog.Info("Starting Monteverdi stats endpoint...", slog.String("Port", addr))
		if err := view.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Could not start stats endpoint", slog.Any("Error", err))
		}
	}()

	view.handleKeyBoardEvent()

	return err
}
