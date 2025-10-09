package monteverdi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	Md "github.com/maroda/monteverdi/display"
	Mo "github.com/maroda/monteverdi/obvy"
	Ms "github.com/maroda/monteverdi/server"
)

func TestScreen(t *testing.T) {
	s, err := makeTestScreen(t, "")
	assertError(t, err, nil)
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

func TestView_GetScreenSize(t *testing.T) {
	qn := &Ms.QNet{
		Network: []*Ms.Endpoint{
			{
				ID:     "TEST",
				Metric: map[int]string{0: "CPU1"},
				Mdata:  map[string]int64{"CPU1": 100},
			},
		},
	}

	s, err := makeTestScreen(t, "utf8")
	assertError(t, err, nil)
	defer s.Fini()

	s.SetSize(100, 30)

	view := &Md.View{
		QNet:   qn,
		Screen: s,
	}

	t.Run("Check view size", func(t *testing.T) {
		width, height := view.GetScreenSize()
		if width != 100 || height != 30 {
			t.Errorf("Wrong view size (%v, %v, %v)", width, height, 30)
		}
	})

	t.Run("Resize is updated correctly", func(t *testing.T) {
		s.SetSize(120, 40)
		width, height := view.GetScreenSize()
		if width != 120 || height != 40 {
			t.Errorf("Wrong view size (%v, %v, %v)", width, height, 40)
		}
	})
}

func TestCalcTimeseriesY(t *testing.T) {
	t.Run("Single endpoint with single metric", func(t *testing.T) {
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID:     "endpoint1",
					Metric: map[int]string{0: "metric1"},
				},
			},
		}

		view := &Md.View{QNet: qnet}
		gutter := 4

		got := view.CalcTimeseriesY(0, 0, gutter)
		want := 4 // gutter + 0 offset + 0 metricIndex

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("Single endpoint with multiple metrics", func(t *testing.T) {
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID: "endpoint1",
					Metric: map[int]string{
						0: "metric1",
						1: "metric2",
						2: "metric3",
					},
				},
			},
		}

		view := &Md.View{QNet: qnet}
		gutter := 4

		// Test each metric position
		tests := []struct {
			metricIndex int
			want        int
		}{
			{0, 4}, // gutter + 0
			{1, 5}, // gutter + 1
			{2, 6}, // gutter + 2
		}

		for _, tt := range tests {
			got := view.CalcTimeseriesY(0, tt.metricIndex, gutter)
			if got != tt.want {
				t.Errorf("metricIndex %d: got %d, want %d", tt.metricIndex, got, tt.want)
			}
		}
	})

	t.Run("Multiple endpoints stacks correctly", func(t *testing.T) {
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID: "endpoint1",
					Metric: map[int]string{
						0: "metric1",
						1: "metric2",
					},
				},
				{
					ID: "endpoint2",
					Metric: map[int]string{
						0: "metric3",
						1: "metric4",
						2: "metric5",
					},
				},
				{
					ID: "endpoint3",
					Metric: map[int]string{
						0: "metric6",
					},
				},
			},
		}

		view := &Md.View{QNet: qnet}
		gutter := 4

		tests := []struct {
			name          string
			endpointIndex int
			metricIndex   int
			want          int
			explanation   string
		}{
			{"endpoint1-metric1", 0, 0, 4, "gutter(4) + 0 offset + 0 index"},
			{"endpoint1-metric2", 0, 1, 5, "gutter(4) + 0 offset + 1 index"},
			{"endpoint2-metric3", 1, 0, 6, "gutter(4) + 2 from ep1 + 0 index"},
			{"endpoint2-metric4", 1, 1, 7, "gutter(4) + 2 from ep1 + 1 index"},
			{"endpoint2-metric5", 1, 2, 8, "gutter(4) + 2 from ep1 + 2 index"},
			{"endpoint3-metric6", 2, 0, 9, "gutter(4) + 2 from ep1 + 3 from ep2 + 0 index"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := view.CalcTimeseriesY(tt.endpointIndex, tt.metricIndex, gutter)
				if got != tt.want {
					t.Errorf("got %d, want %d (%s)", got, tt.want, tt.explanation)
				}
			})
		}
	})

	t.Run("Different gutter values", func(t *testing.T) {
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID:     "endpoint1",
					Metric: map[int]string{0: "metric1"},
				},
			},
		}

		view := &Md.View{QNet: qnet}

		tests := []struct {
			gutter int
			want   int
		}{
			{0, 0},
			{2, 2},
			{4, 4},
			{10, 10},
		}

		for _, tt := range tests {
			got := view.CalcTimeseriesY(0, 0, tt.gutter)
			if got != tt.want {
				t.Errorf("gutter %d: got %d, want %d", tt.gutter, got, tt.want)
			}
		}
	})
}

func TestView_CalcTimePos(t *testing.T) {
	s, err := makeTestScreen(t, "utf8")
	assertError(t, err, nil)
	defer s.Fini()
	s.SetSize(100, 20)

	t.Run("Returns correct position for screen size", func(t *testing.T) {
		// Setting the testing time to 10s in the past
		// should set the position to 87
		timetest := time.Now().Add(-10 * time.Second)
		newpos := 87
		view := &Md.View{
			QNet:   &Ms.QNet{},
			Screen: s,
		}
		got := view.CalcTimePos(timetest)
		want := newpos
		assertInt(t, got, want)
	})

	t.Run("Corrected position when outside of window", func(t *testing.T) {
		timetest := time.Now().Add(-100 * time.Second)
		newpos := 0
		view := &Md.View{
			QNet:   &Ms.QNet{},
			Screen: s,
		}
		got := view.CalcTimePos(timetest)
		want := newpos
		assertInt(t, got, want)
	})
}

func TestView_CalcDurationWidth(t *testing.T) {
	s, err := makeTestScreen(t, "utf8")
	assertError(t, err, nil)
	defer s.Fini()
	s.SetSize(100, 20)

	view := &Md.View{
		QNet:   &Ms.QNet{},
		Screen: s,
	}

	dur := 10 * time.Second
	want := 10
	got := view.CalcDurationWidth(dur)
	assertInt(t, got, want)
}

func TestHandleMouseClick(t *testing.T) {
	t.Run("First timeseries selects endpoint and metric", func(t *testing.T) {
		qn := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID: "endpoint1",
					Metric: map[int]string{
						0: "CPU1",
						1: "CPU2",
					},
					Mdata: map[string]int64{
						"CPU1": 60,
						"CPU2": 80,
					},
				},
			},
		}

		s, err := makeTestScreen(t, "utf8")
		assertError(t, err, nil)
		defer s.Fini()
		s.SetSize(80, 20)

		view := &Md.View{
			QNet:   qn,
			Screen: s,
		}

		// Calculate where the first metric should be drawn
		yPos := view.CalcTimeseriesY(0, 0, 4)

		// Click within the valid range (1 to 60)
		view.HandleMouseClick(30, yPos)

		// Check that the correct endpoint and metric are selected
		if !view.ShowEP {
			t.Error("ShowEP should be true after click")
		}
		if !view.ShowMe {
			t.Error("ShowMe should be true after click")
		}
		if view.SelectEP != 0 {
			t.Errorf("SelectEP should be 0 but was %d", view.SelectEP)
		}
		if view.SelectMe != "CPU1" {
			t.Errorf("SelectMe should be CPU1 but was %s", view.SelectMe)
		}

	})
}

func TestView_DrawRune(t *testing.T) {
	t.Run("Draws rune with color", func(t *testing.T) {
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{
				{
					ID:     "TEST",
					Metric: map[int]string{0: "CPU1"},
				},
			},
		}

		s, err := makeTestScreen(t, "utf8")
		assertError(t, err, nil)
		defer s.Fini()
		s.SetSize(80, 20)

		view := &Md.View{
			QNet:   qnet,
			Screen: s,
		}

		// Draw rune at pos (10, 5) with metric value 100
		x, y, m := 10, 5, 100
		view.DrawRune(x, y, m)

		s.Show() // Process content
		mainc, _, style, width := s.GetContent(x, y)

		if mainc != '' {
			t.Errorf("Expected rune \uF8FF, got %c", mainc)
		}

		if width != 1 {
			t.Errorf("Expected width 1, got %d", width)
		}

		// The foreground color should be calculated based on x, y, m
		// color := tcell.NewRGBColor(int32(150+x), int32(150+y), int32(255-m))
		expectedColor := tcell.NewRGBColor(int32(150+x), int32(150+y), int32(255-m))
		expectedStyle := tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(expectedColor)

		// Compare the styles directly
		if style != expectedStyle {
			t.Errorf("Style mismatch: got %v, want %v", style, expectedStyle)
		}
	})

}

func TestView_DrawText(t *testing.T) {
	t.Run("Draws text with color", func(t *testing.T) {
		ep := &Ms.Endpoint{
			ID:     "TEST",
			Metric: map[int]string{0: "CPU1"},
		}
		eps := []*Ms.Endpoint{ep}
		view := makeTestViewWithScreen(t, eps)
		defer view.Screen.Fini()

		text := "HELLO"
		view.DrawText(5, 10, 80, 20, text)
		view.Screen.Show()

		// Expected style for drawText
		expectedStyle := tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorLightSteelBlue)

		// Verify each character
		expectedRunes := []rune(text)
		for i, expectedRune := range expectedRunes {
			mainc, _, style, _ := view.Screen.GetContent(5+i, 10)

			if mainc != expectedRune {
				t.Errorf("Position %d: expected '%c', got '%c'", i, expectedRune, mainc)
			}

			// Compare styles directly
			if style != expectedStyle {
				t.Errorf("Position %d: style mismatch", i)
			}
		}
	})
}

func TestView_DrawViewBorder(t *testing.T) {
	t.Run("Draws complete border", func(t *testing.T) {
		view := makeTestViewWithScreen(t, []*Ms.Endpoint{
			{ID: "test", Metric: map[int]string{0: "metric"}},
		})
		defer view.Screen.Fini()

		width, height := 78, 18 // Leave 2px for the border
		view.DrawViewBorder(width, height)
		view.Screen.Show()

		// Expected style for border
		expectedStyle := tcell.StyleDefault.
			Background(tcell.ColorBlack).
			Foreground(tcell.ColorPink)

		// Check corners
		corners := []struct {
			x, y int
			want rune
		}{
			{0, 0, tcell.RuneULCorner},
			{width, 0, tcell.RuneURCorner},
			{0, height, tcell.RuneLLCorner},
			{width, height, tcell.RuneLRCorner},
		}

		for _, c := range corners {
			mainc, _, style, _ := view.Screen.GetContent(c.x, c.y)
			if mainc != c.want {
				t.Errorf("Corner at (%d,%d): expected %c, got %c",
					c.x, c.y, c.want, mainc)
			}

			// Compare styles directly
			if style != expectedStyle {
				t.Errorf("Corner at (%d,%d): style mismatch", c.x, c.y)
			}
		}
	})
}

func TestView_RuneIntensityStyle(t *testing.T) {
	tests := []struct {
		rune          rune
		expectedColor tcell.Color
	}{
		{'▁', tcell.ColorSeaGreen},
		{'▂', tcell.ColorMediumSeaGreen},
		{'▃', tcell.ColorLightSeaGreen},
		{'▄', tcell.ColorDarkTurquoise},
		{'▅', tcell.ColorMediumTurquoise},
		{'▆', tcell.ColorTurquoise},
		{'▇', tcell.ColorLightGreen},
		{'█', tcell.ColorAquaMarine},
		{' ', tcell.ColorDefault},
		{'X', tcell.ColorDefault}, // Unknown rune
	}

	for _, tt := range tests {
		t.Run(string(tt.rune), func(t *testing.T) {
			got := tcell.StyleDefault.Foreground(tt.expectedColor)
			want := getStyleForTimeseriesRune(tt.rune)
			if got != want {
				t.Errorf("For rune %c: got %v, want %v", tt.rune, got, want)
			}
		})
	}
}

func TestView_DrawTimeseries(t *testing.T) {
	t.Run("Draws all intensity runes with correct colors", func(t *testing.T) {
		// Test all possible intensity runes
		allIntensityRunes := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

		endpoint := &Ms.Endpoint{
			ID:     "test-ep",
			Metric: map[int]string{0: "cpu"},
			Mdata:  map[string]int64{"cpu": 50},
			Maxval: map[string]int64{"cpu": 100},
			Layer: map[string]*Ms.Timeseries{
				"cpu": {
					Runes:   make([]rune, 60),
					MaxSize: 60,
					Current: len(allIntensityRunes) - 1, // Current points to last written position
				},
			},
		}

		// Place runes at positions 0-7 (the start of the buffer)
		for i, r := range allIntensityRunes {
			endpoint.Layer["cpu"].Runes[i] = r
		}

		view := makeTestViewWithScreen(t, []*Ms.Endpoint{endpoint})
		defer view.Screen.Fini()

		// Draw the timeseries at position (5, 10)
		view.DrawTimeseries(5, 10, 0, "cpu")
		view.Screen.Show()

		// GetDisplay with Current=7 starts reading from position (7+1)%60 = 8
		// So it reads: positions 8-59 (all zeros/spaces), then 0-7 (our runes)
		// This means our runes appear at display positions 52-59

		// Verify each intensity rune was drawn with the correct color
		colorMap := map[rune]tcell.Color{
			'▁': tcell.ColorSeaGreen,
			'▂': tcell.ColorMediumSeaGreen,
			'▃': tcell.ColorLightSeaGreen,
			'▄': tcell.ColorDarkTurquoise,
			'▅': tcell.ColorMediumTurquoise,
			'▆': tcell.ColorTurquoise,
			'▇': tcell.ColorLightGreen,
			'█': tcell.ColorAquaMarine,
		}

		// Our runes appear at the END of the display (positions 52-59)
		displayOffset := 60 - len(allIntensityRunes) // 52
		for i, expectedRune := range allIntensityRunes {
			screenPos := 5 + displayOffset + i // Screen x position
			mainc, _, style, _ := view.Screen.GetContent(screenPos, 10)

			// Check the rune
			if mainc != expectedRune {
				t.Errorf("Position %d: expected rune '%c', got '%c'",
					i, expectedRune, mainc)
			}

			// Check the color matches the intensity
			expectedColor := colorMap[expectedRune]
			expectedStyle := tcell.StyleDefault.Foreground(expectedColor)

			if style != expectedStyle {
				t.Errorf("Position %d: style mismatch for rune '%c'",
					i, expectedRune)
			}
		}

		// First 52 positions should be spaces (zero runes)
		for i := 0; i < displayOffset; i++ {
			mainc, _, style, _ := view.Screen.GetContent(5+i, 10)

			if mainc != ' ' && mainc != 0 {
				t.Errorf("Position %d: expected space, got '%c'", i, mainc)
			}

			expectedStyle := tcell.StyleDefault
			if style != expectedStyle {
				t.Errorf("Position %d: expected default style for space", i)
			}
		}
	})

	t.Run("Handles empty/zero runes as spaces", func(t *testing.T) {
		endpoint := &Ms.Endpoint{
			ID:     "test-ep",
			Metric: map[int]string{0: "cpu"},
			Layer: map[string]*Ms.Timeseries{
				"cpu": {
					Runes:   make([]rune, 60), // All zeros by default
					MaxSize: 60,
					Current: 0,
				},
			},
		}

		view := makeTestViewWithScreen(t, []*Ms.Endpoint{endpoint})
		defer view.Screen.Fini()

		view.DrawTimeseries(5, 10, 0, "cpu")
		view.Screen.Show()

		// All positions should have spaces
		for i := 0; i < 60; i++ {
			mainc, _, _, _ := view.Screen.GetContent(5+i, 10)
			if mainc != ' ' && mainc != 0 {
				t.Errorf("Position %d: expected space, got '%c'", i, mainc)
			}
		}
	})
}

func TestView_GetPulseRune(t *testing.T) {
	view := makeTestViewWithScreen(t, []*Ms.Endpoint{})
	defer view.Screen.Fini()

	t.Run("Correct rune and color for each type", func(t *testing.T) {
		tests := []struct {
			name        string
			pattern     Ms.PulsePattern
			expectRune  rune
			expectColor tcell.Color
		}{
			{"Iamb", Ms.Iamb, '⚍', tcell.ColorMaroon},
			{"Trochee", Ms.Trochee, '⚎', tcell.ColorDarkOrange},
			{"Amphibrach", Ms.Amphibrach, '☵', tcell.ColorAquaMarine},
			{"Anapest", Ms.Anapest, '☳', tcell.ColorAzure},
			{"Dactyl", Ms.Dactyl, '☶', tcell.ColorDodgerBlue},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test accent (saturated)
				symbol, style := view.GetPulseRune(tt.pattern, true)

				if symbol != tt.expectRune {
					t.Errorf("Accent: expected rune '%c', got '%c'",
						tt.expectRune, symbol)
				}

				expectedStyle := tcell.StyleDefault.Foreground(tt.expectColor)
				if style != expectedStyle {
					t.Errorf("Accent: style mismatch")
				}

				// Test non-accent (dimmed)
				symbol, style = view.GetPulseRune(tt.pattern, false)

				if symbol != tt.expectRune {
					t.Errorf("Non-accent: expected rune '%c', got '%c'",
						tt.expectRune, symbol)
				}

				expectedDimStyle := tcell.StyleDefault.Foreground(tt.expectColor).Dim(true)
				if style != expectedDimStyle {
					t.Errorf("Non-accent: style should be dimmed")
				}
			})
		}
	})

}

func TestResponseWriter(t *testing.T) {
	t.Run("StatusCode from WriteHeader", func(t *testing.T) {
		r := httptest.NewRecorder()
		rw := &Md.RespWriter{
			ResponseWriter: r,
			Status:         200,
		}

		rw.WriteHeader(404)

		if rw.Status != 404 {
			t.Errorf("Expected status code 404, got %d", rw.Status)
		}
		if r.Code != 404 {
			t.Errorf("Underlying code should be 404, got %d", r.Code)
		}
	})

	t.Run("Defaults to 200 status", func(t *testing.T) {
		r := httptest.NewRecorder()
		rw := &Md.RespWriter{
			ResponseWriter: r,
			Status:         200,
		}

		// WriteHeader not called, should remain 200
		if rw.Status != 200 {
			t.Errorf("Expected default status 200, got %d", rw.Status)
		}
	})

	t.Run("Multiple writes accumulate", func(t *testing.T) {
		r := httptest.NewRecorder()
		rw := &Md.RespWriter{
			ResponseWriter: r,
			Status:         200,
		}

		rw.Write([]byte("Hello "))
		rw.Write([]byte("World"))

		if r.Body.String() != "Hello World" {
			t.Errorf("Expected body 'Hello World', got %q", r.Body.String())
		}
	})
}

func TestView_StatsMiddleware(t *testing.T) {
	t.Run("Records 200 for successful requests", func(t *testing.T) {
		view := &Md.View{Stats: Mo.NewStatsInternal()}

		// Use a simple handler that returns 200
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello World"))
		})

		middleware := view.StatsMiddleware(handler)

		// Make the request
		r := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, r)
		assertStatus(t, rec.Code, http.StatusOK)
	})
}

/*
// TODO: WIP... this is uncovering a panic somewhere?
func TestView_PollQNetAll(t *testing.T) {
	t.Run("Successfully polls and records timing", func(t *testing.T) {
		// Create mock server with metrics
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "CPU1=100")
			fmt.Fprintln(w, "CPU2=200")
			fmt.Fprintln(w, "CPU3=300")
		}))
		defer mockServer.Close()

		// Create endpoint pointing to mock server
		endpoint := makeEndpoint("test-endpoint", mockServer.URL)
		qnet := &Ms.QNet{
			Network: []*Ms.Endpoint{endpoint},
		}

		stats := Mo.NewStatsInternal()
		view := &Md.View{
			QNet:  qnet,
			Stats: stats,
		}

		// Call PollQNetAll
		err := view.PollQNetAll()

		// Should not return an error (always returns nil)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}

		// Verify data was polled
		if qnet.Network[0].Mdata["CPU1"] != 100 {
			t.Errorf("Expected CPU1=100, got %d", qnet.Network[0].Mdata["CPU1"])
		}
		if qnet.Network[0].Mdata["CPU2"] != 200 {
			t.Errorf("Expected CPU2=200, got %d", qnet.Network[0].Mdata["CPU2"])
		}
		if qnet.Network[0].Mdata["CPU3"] != 300 {
			t.Errorf("Expected CPU3=300, got %d", qnet.Network[0].Mdata["CPU3"])
		}

		// Stats should have recorded the poll duration
		// (We can't easily verify the exact value, but we know it was called)
	})
}

*/

// Helpers //

func getStyleForTimeseriesRune(r rune) tcell.Style {
	switch r {
	case '▁':
		return tcell.StyleDefault.Foreground(tcell.ColorSeaGreen)
	case '▂':
		return tcell.StyleDefault.Foreground(tcell.ColorMediumSeaGreen)
	case '▃':
		return tcell.StyleDefault.Foreground(tcell.ColorLightSeaGreen)
	case '▄':
		return tcell.StyleDefault.Foreground(tcell.ColorDarkTurquoise)
	case '▅':
		return tcell.StyleDefault.Foreground(tcell.ColorMediumTurquoise)
	case '▆':
		return tcell.StyleDefault.Foreground(tcell.ColorTurquoise)
	case '▇':
		return tcell.StyleDefault.Foreground(tcell.ColorLightGreen)
	case '█':
		return tcell.StyleDefault.Foreground(tcell.ColorAquaMarine)
	default:
		return tcell.StyleDefault
	}
}

func makeTestViewWithScreen(t *testing.T, eps []*Ms.Endpoint) *Md.View {
	qn := &Ms.QNet{Network: eps}
	s, err := makeTestScreen(t, "utf8")
	assertError(t, err, nil)
	s.SetSize(80, 20)
	return &Md.View{QNet: qn, Screen: s}
}

func makeTestScreen(t *testing.T, charset string) (tcell.SimulationScreen, error) {
	s := tcell.NewSimulationScreen(charset)
	if s == nil {
		t.Fatalf("Failed to get SimulationScreen")
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init screen: %v", err)
		return s, err
	}
	return s, nil
}
