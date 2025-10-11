package monteverdi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

const (
	screenGutter = 4
)

// View is updated by whatever is in the QNet
type View struct {
	MU          sync.Mutex        // State locks to read data
	QNet        *Ms.QNet          // Quality Network
	Screen      tcell.Screen      // the screen itself
	display     []string          // rune display sequence
	Stats       *Mo.StatsInternal // Internal status for prometheus
	server      *http.Server      // Prometheus metrics server
	SelectEP    int               // Selected Endpoint with MouseClick
	ShowEP      bool              // Display Endpoint ID
	SelectMe    string            // Selected Metric with MouseClick
	ShowMe      bool              // Display Metric ID
	ShowPulse   bool              // Display pulse view overlay
	pulseFilter *Ms.PulsePattern  // For filtering the display
}

// CalcTimeseriesY figures out where to draw the next Timeseries entry on the graph
func (v *View) CalcTimeseriesY(endpointIndex, metricIndex, gutter int) int {
	// Calculate cumulative offset from all previous endpoints
	offset := 0
	for i := 0; i < endpointIndex; i++ {
		offset += len(v.QNet.Network[i].Metric)
	}
	return gutter + offset + metricIndex
}

////////// PULSE VIS

func (v *View) CalcTimePos(pulseStartTime time.Time) int {
	width, _ := v.GetScreenSize()
	timelineW := width - 2 // room for border drawing

	now := time.Now()
	secondsAgo := int(now.Sub(pulseStartTime).Seconds())

	// Timeline position (0 to timelineWidth-1, where timelineWidth-1 is most recent)
	position := timelineW - 1 - secondsAgo
	if position < 0 {
		position = 0 // Pulse started before visible window
	}
	return position
}

func (v *View) CalcDurationWidth(duration time.Duration) int {
	width, _ := v.GetScreenSize()
	maxW := width - 2 // room for border drawing

	// Convert pulse duration to character width on timeline
	durationSeconds := int(duration.Seconds())

	// Cap at reasonable width to prevent overflow
	if durationSeconds > maxW {
		durationSeconds = maxW
	}
	return durationSeconds
}

func (v *View) GetPulseRune(pattern Ms.PulsePattern, isAccent bool) (rune, tcell.Style) {
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
	case Ms.Amphibrach:
		baseColor = tcell.ColorAquaMarine
		symbol = '☵'
	case Ms.Anapest:
		baseColor = tcell.ColorAzure
		symbol = '☳'
	case Ms.Dactyl:
		baseColor = tcell.ColorDodgerBlue
		symbol = '☶'
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
		symbol, style := v.GetPulseRune(point.Pattern, point.IsAccent)
		v.Screen.SetContent(x+point.Position, y, symbol, nil, style)
	}
}

func (v *View) drawPulseView() {
	width, height := v.GetScreenSize()
	timelineW := width - 2 // room for border drawing

	// Clear screen completely first
	v.Screen.Clear()

	// Redraw borders
	v.DrawViewBorder(width, height)

	// Clear or dim the background first
	/*
		v.drawPulseBackground()
	*/

	// Show current filter mode
	filterText := "All Patterns"
	if v.pulseFilter != nil {
		switch *v.pulseFilter {
		case Ms.Iamb:
			filterText = "Iamb Only"
		case Ms.Trochee:
			filterText = "Trochee Only"
		case Ms.Amphibrach:
			filterText = "Amphibrach Only"
		}
	}

	v.DrawText(1, 1, width-10, 2, fmt.Sprintf("PULSE VIEW - %s (triple ictus analysis)", filterText))
	v.DrawText(1, 2, width-10, 3, "i=Iamb | t=Trochee | a=Amphibrach | x=All | ► stacked long pulses ◄ ")

	// Draw pulse visualization for each endpoint/metric
	for ni := range v.QNet.Network {
		for di, dm := range v.QNet.Network[ni].Metric {
			yTS := v.CalcTimeseriesY(ni, di, screenGutter)

			// Pass the filter for display
			timelineData := v.QNet.Network[ni].GetPulseVizData(dm, v.pulseFilter)
			v.renderPulseViz(1, yTS, timelineData)

			// Track long pulse boundaries
			var longPulseStart, longPulseEnd = -1, -1
			now := time.Now()

			for _, point := range timelineData {
				pulseAge := now.Sub(point.StartTime).Seconds()

				if pulseAge > float64(timelineW) {
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
				v.Screen.SetContent(1+longPulseStart, yTS, '►', nil, leftStyle)
			}

			if longPulseEnd >= 0 && longPulseEnd != longPulseStart {
				rightStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkTurquoise)
				v.Screen.SetContent(1+longPulseEnd, yTS, '◄', nil, rightStyle)
			}
		}
	}

	// Add pulse view indicator
	v.DrawText(1, 1, 20, 2, "PULSE VIEW")
}

/*
func (v *View) drawPulseBackground() {
	width, height := v.GetScreenSize()

	// Dim the existing content
	for y := 2; y < height-2; y++ {
		for x := 1; x < width-1; x++ {
			// Get current content and dim it
			mainc, combc, style, _ := v.Screen.GetContent(x, y)
			dimmedStyle := style.Background(tcell.ColorBlack).Foreground(tcell.ColorDarkGray)
			v.Screen.SetContent(x, y, mainc, combc, dimmedStyle)
		}
	}
}

*/

////////// PULSE VIS ^^^^^

// DrawRune places a single '' on the screen
// used to draw the accents/second indicator
func (v *View) DrawRune(x, y, m int) {
	color := tcell.NewRGBColor(int32(150+x), int32(150+y), int32(255-m))
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(color)
	v.Screen.SetContent(x, y, '', nil, style)
}

// DrawTimeseries displays the current Timeseries data for a metric
func (v *View) DrawTimeseries(x, y, i int, m string) {
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

		v.Screen.SetContent(x+runeIndex, y, r, nil, style) // Use runeIndex here
	}
}

// DrawText displays the text string at the given (x1, y1) with box size (x2, y2)
func (v *View) DrawText(x1, y1, x2, y2 int, text string) {
	row := y1
	col := x1
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorLightSteelBlue)
	for _, r := range text {
		v.Screen.SetContent(col, row, r, nil, style)
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

// DrawViewBorder displays the outline of the View
func (v *View) DrawViewBorder(width, height int) {
	hvStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorPink)
	v.Screen.SetContent(0, 0, tcell.RuneULCorner, nil, hvStyle)
	for i := 1; i < width; i++ {
		v.Screen.SetContent(i, 0, tcell.RuneHLine, nil, hvStyle)
	}
	v.Screen.SetContent(width, 0, tcell.RuneURCorner, nil, hvStyle)

	for i := 1; i < height; i++ {
		v.Screen.SetContent(0, i, tcell.RuneVLine, nil, hvStyle)
	}

	v.Screen.SetContent(0, height, tcell.RuneLLCorner, nil, hvStyle)

	for i := 1; i < height; i++ {
		v.Screen.SetContent(width, i, tcell.RuneVLine, nil, hvStyle)
	}

	v.Screen.SetContent(width, height, tcell.RuneLRCorner, nil, hvStyle)

	for i := 1; i < width; i++ {
		v.Screen.SetContent(i, height, tcell.RuneHLine, nil, hvStyle)
	}
}

// DrawHarmonyViewMulti draws the HarmonyView itself with tcell
// Includes a toggle for View mode (Accent or Pulse)
func (v *View) DrawHarmonyViewMulti() {
	// This is the border of the box
	width, height := v.GetScreenSize()

	// Lock QNet first, then view state
	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	// Obtain a lock and grab needed display data
	v.MU.Lock()
	showEP := v.ShowEP
	showMe := v.ShowMe
	showPulse := v.ShowPulse
	selectEP := v.SelectEP
	selectMe := v.SelectMe
	v.MU.Unlock()

	// Draw basic elements
	v.DrawViewBorder(width-2, height-1)

	// Support toggle to pulse view by wrapping in a boolean
	if showPulse {
		v.drawPulseView()

		// A MouseClick has happened on a graph
		// - show the Endpoint ID at the bottom
		// - show the metric name and value to the side
		if showEP {
			v.showEndpointWithState(40, 1, showEP, selectEP)
		}
		if showMe {
			v.showMetricWithState(2, screenGutter, 0, 4, showMe, selectEP, selectMe)
		}

		v.DrawText(1, height-1, width, height+10, "/p/ to exit | /ESC/ to quit")
	} else {
		// step through all Network endpoints
		for ni := range v.QNet.Network {
			// step through metrics listed in View.display
			for di, dm := range v.QNet.Network[ni].Metric {
				// look up the key in this Network's Endpoint Metric data.
				ddm := v.QNet.Network[ni].Mdata[dm]

				// Calculate unique y position for each endpoint/metric combination
				yTS := v.CalcTimeseriesY(ni, di, screenGutter)

				// draw timeseries - each endpoint gets its own line
				v.DrawTimeseries(1, yTS, ni, dm)

				// See an Accent happen
				dda := v.QNet.Network[ni].Accent[dm]
				if dda != nil {
					// now get the second from the Timestamp. this is the X position on the display
					newTime := time.Unix(dda.Timestamp/1e9, dda.Timestamp%1e9)
					s := newTime.Second()

					// draw a rune across the top
					v.DrawRune(s, 1, int(ddm))
				}
			}
		}

		// A MouseClick has happened on a graph, show the metric name and value
		// retrieve the data via lock
		if showMe {
			for ni := range v.QNet.Network {
				if ni == selectEP {
					for di, dm := range v.QNet.Network[ni].Metric {
						if dm == selectMe {
							yTS := v.CalcTimeseriesY(ni, di, screenGutter)

							mdata := v.QNet.Network[ni].Mdata[dm]
							label := fmt.Sprintf("... %s ...", dm) // The Metric
							data := fmt.Sprintf("%d", mdata)       // The raw data
							v.DrawText(2, yTS, width, yTS, data)
							v.DrawText(4, height-2, width, height-2, label)
						}
					}
				}
			}
		}

		// A MouseClick has happened on a graph, show the Endpoint ID at the bottom
		if showEP {
			v.showEndpointWithState(40, 1, showEP, selectEP)
		}

		v.DrawText(1, height-1, width, height+10, "/p/ for pulses | /ESC/ to quit")
	}

	v.DrawText(width-12, height-1, width, height+10, "MONTEVERDI")
}

// showMetricWithState does not retrieve a state lock
// and takes parameters for these values instead
func (v *View) showMetricWithState(by, g, dx, lx int, showMe bool, selectEP int, selectMe string) {
	width, height := v.GetScreenSize()

	if showMe {
		for ni := range v.QNet.Network {
			if ni == selectEP {
				for di, dm := range v.QNet.Network[ni].Metric {
					if dm == selectMe {
						yTS := v.CalcTimeseriesY(ni, di, g)
						mdata := v.QNet.Network[ni].Mdata[dm]
						data := fmt.Sprintf("%d", mdata)       // The raw data
						label := fmt.Sprintf("... %s ...", dm) // The Metric

						// Turn off drawing raw metrics by using dx=0
						if dx != 0 {
							v.DrawText(dx, yTS, width, yTS, data)
						}
						v.DrawText(lx, height-by, width, height-by, label)
					}
				}
			}
		}
	}
}

// showEndpointWithState does not retrieve a state lock
// and takes parameters for these values instead
func (v *View) showEndpointWithState(x, by int, showEP bool, selectEP int) {
	width, height := v.GetScreenSize()
	if showEP {
		epName := v.QNet.Network[selectEP].ID
		v.DrawText(x, height-by, width, height, fmt.Sprintf("|  Polling: %s  |", epName))
	}
}

// Exit cleanly
func (v *View) exit() {
	v.MU.Lock()
	defer v.MU.Unlock()
	v.Screen.Fini()
	os.Exit(0)
}

// Running Loop to handle events
func (v *View) handleKeyBoardEvent() {
	for {
		ev := v.Screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			v.ResizeScreen()
		case *tcell.EventKey:
			// Catch quit and exit
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				v.exit()
			}

			// Toggle pulse view with 'p'
			if ev.Rune() == 'p' {
				v.MU.Lock()
				v.ShowPulse = !v.ShowPulse
				v.MU.Unlock()
			}

			// Pattern filtering (only when in pulse view)
			if v.ShowPulse {
				switch ev.Rune() {
				case 'a':
					v.MU.Lock()
					amphibrach := Ms.Amphibrach
					v.pulseFilter = &amphibrach
					v.MU.Unlock()
				case 'i':
					v.MU.Lock()
					iamb := Ms.Iamb
					v.pulseFilter = &iamb
					v.MU.Unlock()
				case 't':
					v.MU.Lock()
					trochee := Ms.Trochee
					v.pulseFilter = &trochee
					v.MU.Unlock()
				case 'x':
					v.MU.Lock()
					v.pulseFilter = nil // Show all patterns
					v.MU.Unlock()
				}
			}

		case *tcell.EventMouse:
			// Button1 is Left Mouse Button
			if ev.Buttons() == tcell.Button1 {
				v.HandleMouseClick(ev.Position())
			}
		}
	}
}

func (v *View) HandleMouseClick(x, y int) {
	// Lock QNet to safely read Network endpoint data
	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	// Lock display for updates
	v.MU.Lock()
	defer v.MU.Unlock()

	// Assume there is no label so the last one is cleared.
	v.ShowEP = false
	v.ShowMe = false

	// Check for a click on any timeseries graph
	for ni := range v.QNet.Network {
		for di, dm := range v.QNet.Network[ni].Metric {
			// yTS is the same as drawHarmonyViewMulti
			yTS := v.CalcTimeseriesY(ni, di, screenGutter)

			// Check if click is on this timeseries line
			// Timeseries spans x=1 to x=60
			// Exit if a match is found
			width, _ := v.GetScreenSize()
			if y == yTS && x >= 1 && x <= width-20 {
				v.SelectEP = ni
				v.SelectMe = dm
				v.ShowEP = true
				v.ShowMe = true
				return
			}
		}
	}
}

// PollQNetAll is for reading the multi metric config in Endpoint
// The error return is currently set to /nil/
// so that Poll misses are only logged, not fatal (and blocking)
func (v *View) PollQNetAll() error {
	start := time.Now()

	err := v.QNet.PollMulti()
	if err != nil {
		// Only log the error, keep going otherwise
		slog.Error("Failed to PollMulti", slog.Any("Error", err))
	}

	duration := time.Since(start).Seconds()
	v.Stats.RecPollTimer(duration)

	return nil
}

// GetScreenSize provides the terminal size for drawing
func (v *View) GetScreenSize() (int, int) {
	width, height := v.Screen.Size()
	return width, height
}

// ResizeScreen resizes HarmonyView after terminal changes
func (v *View) ResizeScreen() {
	v.Screen.Sync()
	v.UpdateScreen()
}

func (v *View) UpdateScreen() {
	v.Screen.Clear()
	v.DrawHarmonyViewMulti()
	v.Screen.Show()
}

// run runs a loop and updates periodically
// each iteration polls the configured Metric[]
// and fills the related Mdata[Metric] in QNet,
// which is then read by drawHarmonyViewMulti
// TODO: parameterize run loop time
func (v *View) run() {
	// Panic recovery and logging
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in run loop", slog.Any("panic", r))
			slog.Error("Recovered from panic", slog.String("stack", string(debug.Stack())))
			debug.PrintStack()
		}
	}()

	// Main application loop
	slog.Info("Starting HarmonyView")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Memory usage reporter
	memTicker := time.NewTicker(5 * time.Second)
	defer memTicker.Stop()

	for {
		select {
		case <-ticker.C:
			// Catch a timeout
			if err := v.PollQNetAll(); err != nil {
				slog.Error("Failed to PollQNetAll", slog.Any("Error", err))
				return
			}
			v.UpdateScreen()
		}
	}
}

// RespWriter is a wrapper with StatsMiddleware, used for Prometheus
type RespWriter struct {
	http.ResponseWriter
	Status int
}

// WriteHeader is a helper for StatsMiddleware, used for Prometheus
func (w *RespWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

// Write is a helper for StatsMiddleware, used for Prometheus
func (w *RespWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func (v *View) StatsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// start := time.Now()

		wrapped := &RespWriter{
			ResponseWriter: w,
			Status:         200,
		}
		next.ServeHTTP(wrapped, r)

		// duration := time.Since(start).Seconds()
		v.Stats.RecWWW(strconv.Itoa(wrapped.Status), r.Method)
	})
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
		Screen:  screen,
		display: display, // something is overranging this display slice!
		Stats:   stats,
	}

	view.UpdateScreen()

	return view, err
}

// StartHarmonyViewWithConfig is called by main to run the program.
// This also starts up the /metrics endpoint that is populated by prometheus.
func StartHarmonyViewWithConfig(c []Ms.ConfigFile) error {
	// with the new config c, we can make other stuff
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
		Handler: view.SetupMux(),
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

func StartWebNoTUI(c []Ms.ConfigFile) error {
	// Init Endpoints
	eps, err := Ms.NewEndpointsFromConfig(c)
	if eps == nil || err != nil {
		slog.Error("Failed to init endpoints", slog.Any("Error", err))
		return err
	}

	qn := Ms.NewQNet(*eps)
	if qn == nil {
		slog.Error("Failed to init QNet")
		return errors.New("failed to init QNet")
	}

	// Create View without tcell screen
	stats := Mo.NewStatsInternal()
	view := &View{
		QNet:  qn,
		Stats: stats,
	}

	// Server for stats endpoint
	view.server = &http.Server{
		Addr:    ":8090",
		Handler: view.SetupMux(),
	}

	// Start polling loop
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := view.PollQNetAll(); err != nil {
				slog.Error("Failed to PollQNetAll", slog.Any("Error", err))
			}
		}
	}()

	// Run stats endpoint (blocks)
	addr := ":8090"
	slog.Info("Starting Monteverdi web server...", slog.String("Port", addr))
	if err := view.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Could not start stats endpoint", slog.Any("Error", err))
		return err
	}

	return nil
}
