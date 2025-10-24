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
	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
	Mt "github.com/maroda/monteverdi/types"
)

const (
	screenGutter = 4
)

// View is updated by whatever is in the QNet
type View struct {
	MU          sync.Mutex        // State locks to read data
	QNet        *Ms.QNet          // Quality Network
	Screen      tcell.Screen      // the screen itself
	Display     []string          // rune display sequence
	Stats       *Mo.StatsInternal // Internal status for prometheus
	server      *http.Server      // Prometheus metrics server
	SelectEP    int               // Selected Endpoint with MouseClick
	ShowEP      bool              // Display Endpoint ID
	SelectMe    string            // Selected Metric with MouseClick
	ShowMe      bool              // Display Metric ID
	ShowPulse   bool              // Display pulse view overlay
	PulseFilter *Mt.PulsePattern  // For filtering the display
	Supervisor  *PollSupervisor   // Supervisor for performing QNet polling
	ConfigPath  string            // Path to JSON configuration
}

// NewViewWithScreen inits the tcell screen that displays HarmonyView.
// It takes a tcell.screen as an input alongside the current QNet.
func NewViewWithScreen(q *Ms.QNet, screen tcell.Screen) (*View, error) {
	if q == nil || q.Network == nil {
		slog.Error("Could not get a QNet for display")
		return nil, errors.New("quality network not found")
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
		Display: display,
		Stats:   stats,
	}

	view.UpdateScreen()

	return view, nil
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

func (v *View) GetPulseRune(pattern Mt.PulsePattern, isAccent bool) (rune, tcell.Style) {
	var baseColor tcell.Color
	var symbol rune

	// Pattern type determines baseColor
	switch pattern {
	case Mt.Iamb:
		baseColor = tcell.ColorMaroon
		symbol = '⚍'
	case Mt.Trochee:
		baseColor = tcell.ColorDarkOrange
		symbol = '⚎'
	case Mt.Amphibrach:
		baseColor = tcell.ColorAquaMarine
		symbol = '☵'
	case Mt.Anapest:
		baseColor = tcell.ColorAzure
		symbol = '☳'
	case Mt.Dactyl:
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

func (v *View) RenderPulseViz(x, y int, tld []Mt.PulseVizPoint) {
	for _, point := range tld {
		symbol, style := v.GetPulseRune(point.Pattern, point.IsAccent)
		v.Screen.SetContent(x+point.Position, y, symbol, nil, style)
	}
}

func (v *View) DrawPulseView() {
	width, height := v.GetScreenSize()
	timelineW := width - 2 // room for border drawing

	// Clear screen completely first
	v.Screen.Clear()

	// Redraw borders
	v.DrawViewBorder(width, height)

	// Clear or dim the background first
	v.drawPulseBackground()

	// Show current filter mode
	filterText := "All Patterns"
	if v.PulseFilter != nil {
		switch *v.PulseFilter {
		case Mt.Iamb:
			filterText = "Iamb Only"
		case Mt.Trochee:
			filterText = "Trochee Only"
		case Mt.Amphibrach:
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
			timelineData := v.QNet.Network[ni].GetPulseVizData(dm, v.PulseFilter)
			v.RenderPulseViz(1, yTS, timelineData)

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
		if row > y2 {
			break
		}
		col++
		if col >= x2 {
			row++
			col = x1
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
		v.DrawPulseView()

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
// coordinate args are: bottom-y, gutter, data-x, label-x
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

// HandleMouseClick shows corresponding data with what was clicked
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
func (v *View) PollQNetAll() {
	start := time.Now()
	v.QNet.PollMulti()

	duration := time.Since(start).Seconds()
	v.Stats.RecPollTimer(duration)
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

// UpdateScreen clears, calls the main drawing function, and displays
func (v *View) UpdateScreen() {
	v.Screen.Clear()
	v.DrawHarmonyViewMulti()
	v.Screen.Show()
}

// runTUI updates the display every 60 seconds in a loop
func (v *View) runTUI() {
	// Panic recovery and logging
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in runTUI loop", slog.Any("panic", r))
			slog.Error("Recovered from panic", slog.String("stack", string(debug.Stack())))
			debug.PrintStack()
		}
	}()

	// Main application loop
	slog.Info("Starting HarmonyView")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			v.UpdateScreen()
		}
	}
}

// handleEvent is a runtime loop to handle keyboard and mouse
func (v *View) handleEvent() {
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
					amphibrach := Mt.Amphibrach
					v.PulseFilter = &amphibrach
					v.MU.Unlock()
				case 'i':
					v.MU.Lock()
					iamb := Mt.Iamb
					v.PulseFilter = &iamb
					v.MU.Unlock()
				case 't':
					v.MU.Lock()
					trochee := Mt.Trochee
					v.PulseFilter = &trochee
					v.MU.Unlock()
				case 'x':
					v.MU.Lock()
					v.PulseFilter = nil // Show all patterns
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

// exit cleanly
func (v *View) exit() {
	v.MU.Lock()
	defer v.MU.Unlock()
	v.Screen.Fini()
	os.Exit(0)
}

// RespWriter is used by StatsMiddleware, used for Prometheus
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

// StatsMiddleware invokes the HTTP Responses above, used for Prometheus
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

// StartHarmonyViewWebOnly is the Web UI only, running on localhost:8090
// The ticker for the runtime loop is here as a goroutine, the web server blocks.
// This runs when using the `-headless` flag.
// Logs appear in the console instead of a file.
func StartHarmonyViewWebOnly(c []Ms.ConfigFile, path string) error {
	// Init Endpoints
	eps := Ms.NewEndpointsFromConfig(c)
	qn := Ms.NewQNet(*eps)

	// Create View without tcell screen
	stats := Mo.NewStatsInternal()
	view := &View{
		QNet:  qn,
		Stats: stats,
	}

	// Register config file location
	view.ConfigPath = path

	// Server for web endpoint
	view.server = &http.Server{
		Addr:    ":8090",
		Handler: view.SetupMux(),
	}

	// Create new Poll Supervisor to handle data fetches every minute
	ps := view.NewPollSupervisor()
	ps.Start()
	defer ps.Stop()

	// Run web endpoint (blocks)
	addr := ":8090"
	slog.Info("Starting Monteverdi web server...", slog.String("Port", addr))
	if err := view.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Could not start", slog.Any("Error", err))
		return err
	}

	return nil
}

// StartHarmonyView is the Terminal UI alongside the Web UI, running on localhost:8090
// This is the default view when runTUI from a shell. If there is no TTY, it will not runTUI.
// The `-headless` flag can be used to runTUI in Web UI only mode, StartHarmonyViewWebOnly
// The TUI operates with several looping and blocking processes, all handled here.
func StartHarmonyView(c []Ms.ConfigFile, path string) error {
	// Init endpoints
	eps := Ms.NewEndpointsFromConfig(c)
	qn := Ms.NewQNet(*eps)

	// Define a tcell screen for the view
	screen, err := tcell.NewScreen()
	if err != nil {
		slog.Error("Failed to get Screen", slog.Any("Error", err))
		return err
	}

	// Init the new screen before use
	if err = screen.Init(); err != nil {
		slog.Error("Failed to init Screen", slog.Any("Error", err))
		return err
	}
	defer screen.Fini()

	// If all is good, instantiate the new Terminal UI object
	view, err := NewViewWithScreen(qn, screen)
	if err != nil {
		slog.Error("Could not start HarmonyView", slog.Any("Error", err))
		return err
	}

	// Configure output if set
	// For now, assume BadgerDB
	outputLocation := Ms.FillEnvVar("MONTEVERDI_OUTPUT")
	if outputLocation != "" {
		batchSize := 100
		output, err := Mp.NewBadgerOutput(outputLocation, batchSize)
		if err != nil {
			slog.Error("Failed to create adapter",
				slog.String("output", outputLocation),
				slog.Any("error", err))
			return err
		}
		view.QNet.Output = output
		slog.Info("BadgerOutput Adapter Enabled", slog.String("output", outputLocation))
	}

	// Register config file location
	view.ConfigPath = path

	// Server for web endpoint
	view.server = &http.Server{
		Addr:    ":8090",
		Handler: view.SetupMux(),
	}

	// Create new Poll Supervisor to handle data fetches every minute
	ps := view.NewPollSupervisor()
	ps.Start()
	defer ps.Stop()

	// Run HarmonyView in the terminal
	go view.runTUI()

	// Run webserver in parallel
	go func() {
		addr := ":8090"
		slog.Info("Starting Monteverdi web server...", slog.String("Port", addr))
		if err := view.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Could not start", slog.Any("Error", err))
		}
	}()

	// Capture keyboard events for TUI controls
	view.handleEvent()

	return err
}
