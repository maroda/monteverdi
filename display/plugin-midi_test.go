package monteverdi_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	Md "github.com/maroda/monteverdi/display"
	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
)

func TestInitMIDIOutput(t *testing.T) {
	// Set up a logger to check results
	var logBuffer bytes.Buffer
	logHandler := slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	// Define parameters
	port, root, arpd, arpi, scale := "2", "60", "200", "2", "0,2,2,1,2,2,2,1"

	// Set them as ENV VARs, which are used by the function
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_PORT", port)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ROOT", root)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ARP_DELAY", arpd)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ARP_INTERVAL", arpi)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_SCALE", scale)

	t.Run("Creates expected Output", func(t *testing.T) {
		// Create a View and init its output
		v := makeTestView(t)
		err := Md.InitMIDIOutput(v, "MIDI")
		assertError(t, err, nil)
		defer v.QNet.Output.Close()

		// Grab logger contents to compare
		logOut := logBuffer.String()

		// Check against each original setting
		assertStringContains(t, logOut, port)
		assertStringContains(t, logOut, root)
		assertStringContains(t, logOut, arpd)
		assertStringContains(t, logOut, arpi)
		assertStringContains(t, logOut, scale)
	})

	t.Run("Uses default scale on error", func(t *testing.T) {
		// Change scale to something unparseable
		os.Setenv("MONTEVERDI_PLUGIN_MIDI_SCALE", "☶")

		v := makeTestView(t)
		err := Md.InitMIDIOutput(v, "MIDI")
		assertError(t, err, nil)
		defer v.QNet.Output.Close()

		// change the original list to how delimiters appear in the log
		scalelist := strings.ReplaceAll(scale, ",", " ")

		logOut := logBuffer.String()
		assertStringContains(t, logOut, scalelist)
	})

	t.Run("Errors on adapter creation failure", func(t *testing.T) {
		// Set the MIDI port to something unreasonable
		os.Setenv("MONTEVERDI_PLUGIN_MIDI_PORT", "9999")

		v := makeTestView(t)
		err := Md.InitMIDIOutput(v, "MIDI")
		assertGotError(t, err)

		logOut := logBuffer.String()
		assertStringContains(t, logOut, "Failed to create adapter")
	})
}

func TestNewMIDIOutput(t *testing.T) {
	port, root, arpd, arpi, scale := "2", "60", "200", "2", "0,2,2,1,2,2,2,1"

	os.Setenv("MONTEVERDI_PLUGIN_MIDI_PORT", port)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ROOT", root)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ARP_DELAY", arpd)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_ARP_INTERVAL", arpi)
	os.Setenv("MONTEVERDI_PLUGIN_MIDI_SCALE", scale)

	portI := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_PORT", 0)
	arpdI := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ARP_DELAY", 300)
	arpiI := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ARP_INTERVAL", 1)
	rootI := uint8(Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ROOT", 60))

	var scaleI []uint8
	var scaleS []string
	scaleS = strings.Split(scale, ",")
	for _, v := range scaleS {
		interval, err := strconv.Atoi(v)
		assertError(t, err, nil)
		scaleI = append(scaleI, uint8(interval))
	}

	t.Run("MIDI Output Plugin Initialization", func(t *testing.T) {
		out, err := Mp.NewMIDIOutput(portI, arpdI, arpiI, rootI, scaleI)
		assertError(t, err, nil)
		defer out.Close()

		assertStringContains(t, strconv.Itoa(int(out.Root)), root)

		for i := 0; i < len(scaleI); i++ {
			if scaleI[i] != out.Scale[i] {
				t.Errorf("Expected interval %d, got %d", scaleI[i], out.Scale[i])
			}
		}
	})
}

func TestView_MetricsDataHandlerMIDI(t *testing.T) {
	t.Run("Metrics Data Endpoint", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/metrics-data", nil)
		w := httptest.NewRecorder()
		view := makeViewWithMIDI(t)
		view.MetricsDataHandler(w, r)
		assertStatus(t, w.Code, http.StatusOK)
	})
}

func TestView_PluginControlHandlerMIDI(t *testing.T) {
	view := makeViewWithMIDI(t)
	defer view.QNet.Output.Close()

	tests := []struct {
		name     string
		method   string
		target   string
		assert   int
		contains string
	}{
		{
			name:     "Plugin Control Endpoint: Type",
			method:   "POST",
			target:   "/api/plugin/type",
			assert:   http.StatusOK, // 200
			contains: "MIDI",
		},
		{
			name:     "Plugin Control Endpoint: Flush",
			method:   "POST",
			target:   "/api/plugin/flush",
			assert:   http.StatusOK, // 200
			contains: "FLUSHED",
		},
		{
			name:     "Plugin Control Endpoint: Bad Request (too few elements)",
			method:   "POST",
			target:   "/api/plugin",
			assert:   http.StatusBadRequest, // 400
			contains: "invalid",
		},
		{
			name:     "Plugin Control Endpoint: Bad Request (invalid control)",
			method:   "POST",
			target:   "/api/plugin/cornhole",
			assert:   http.StatusBadRequest, // 400
			contains: "invalid",
		},
		{
			name:     "Plugin Control Endpoint: Close",
			method:   "POST",
			target:   "/api/plugin/close",
			assert:   http.StatusOK, // 200
			contains: "CLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.target, nil)
			w := httptest.NewRecorder()
			view.PluginControlHandler(w, r)
			assertStatus(t, w.Code, tt.assert)
			assertStringContains(t, w.Body.String(), tt.contains)
		})
	}
}

// Helpers //

// ViewWithMIDI opens a MIDI driver
func makeViewWithMIDI(t *testing.T) *Md.View {
	t.Helper()
	midiOut, err := Mp.NewMIDIOutput(0, 200, 1, uint8(60), []uint8{0})
	assertError(t, err, nil)

	// Set up data: New QNet, add Endpoint, create View.
	qn := makeNewTestQNet(t)
	qn.Output = midiOut
	qn.Network[0] = &Ms.Endpoint{
		MU:     sync.RWMutex{},
		ID:     "TEST",
		Metric: map[int]string{0: "TMETRICT"},
		Mdata:  map[string]int64{"TMETRICT": 110},
		Maxval: map[string]int64{"TMETRICT": 100},
	}

	return &Md.View{QNet: qn}
}
