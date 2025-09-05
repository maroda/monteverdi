package main

import (
	"fmt"
	"log"
	"log/slog"

	Md "github.com/maroda/monteverdi/display"
	Ms "github.com/maroda/monteverdi/server"
)

func init() {
	User := Ms.FillEnvVar("USER")
	fmt.Printf("Monteverdi initializing for ... %s\n", User)

	A01 := Ms.NewAccent(1, "main", "output")

	// View some struct data
	fmt.Printf("Accent full timestamp: %s\n", A01.Timestamp)

	// Use an interface method
	fmt.Printf("Interface timestamp: %s\n", A01.TimestampString())

	// Print out the accent
	fmt.Printf("Accent mark '%d' entered for '%s' to be displayed on '%s'\n", A01.Intensity, A01.SourceID, A01.DestLayer)

	// Check Netdata for data
	// this should be its own function
	url := Ms.FillEnvVar("NETDATA_ENDPOINT")
	netdata, err := Ms.MetricKV(url)
	if err != nil {
		log.Fatal(err)
	}
	metricCount := len(netdata)
	fmt.Printf("Netdata metrics: %d\n", metricCount)

	// try out the poll func
	ep := Ms.NewEndpoint("NETDATA", url)
	qn := Ms.NewQNet([]Ms.Endpoint{*ep})
	pollresult, err := qn.Poll("NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ROOT CPU UTIL: %d\n", pollresult)
}

func main() {
	// Check Netdata for data
	// this should be its own function
	url := Ms.FillEnvVar("NETDATA_ENDPOINT")
	ep := Ms.NewEndpoint("NETDATA", url)
	qn := Ms.NewQNet([]Ms.Endpoint{*ep})
	// pollresult will happen in a function inside
	/*
		pollresult, err := qn.Poll("NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL")
		if err != nil {
			log.Fatal(err)
		}
	*/

	err := Md.StartHarmonyView(qn)
	if err != nil {
		slog.Error("Problem starting HarmonyView", slog.Any("Error", err))
		panic("Failed to start harmony view")
	}
}

/*
func main() {
	boxStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorIndigo)
	harmonicStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorOliveDrab)

	s, err := Md.GetTTY()
	if err != nil {
		log.Fatalf("Error getting TTY: %v", err)
	}

	// Draw initial boxes
	// These will always be overdrawn by event loop draws
	// Md.DrawBox(s, 3, 1, 42, 7, boxStyle, "Click and drag to draw a box")
	// Md.DrawBox(s, 5, 9, 32, 14, boxStyle, "Press C to reset")
	Md.DrawBox(s, 4, 4, 20, 10, boxStyle, "craquemattic")

	quit := func() {
		// You have to catch panics in a defer, clean up, and
		// re-raise them - otherwise your application can
		// die without leaving any diagnostic trace.
		maybePanic := recover()
		s.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}
	defer quit()

	// Here's how to get the screen size when you need it.
	// xmax, ymax := s.Size()

	// don't use an event loop here, break this out into functions
	// Md.StartDisplay() -> this way the display interface can be chosen

	// Event loop
	for {
		// Refresh
		s.Clear()

		// Check Netdata for data
		// this should be its own function
		url := Ms.FillEnvVar("NETDATA_ENDPOINT")
		ep := Ms.NewEndpoint("NETDATA", url)
		qn := Ms.NewQNet([]Ms.Endpoint{*ep})
		pollresult, err := qn.Poll("NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL")
		if err != nil {
			log.Fatal(err)
		}

		// I'm not sure about using int64 here yet, but it feels right
		// but for this test, convert pollresult back to int32 for tcell (for now)
		metric := int(pollresult)

		// Write the bar using the updating CPU metric
		Md.WriteBar(s, 20, 1, 22, metric, harmonicStyle)
		s.Show()

		// This breaks keyboard events, but kinda works when active
		// Breaks with:
		//	Process finished with the exit code 137 (interrupted by signal 9:SIGKILL)
		// time.Sleep(time.Millisecond * 500)

		// Poll event
		ev := s.PollEvent()

		// Process event
		// The Event does not have to be processed in order to be recognized.
		// For instance, EventMouse will be captured, and make the loop restart.
		// even if there's no case below to catch.
		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return
			} else if ev.Key() == tcell.KeyCtrlL {
				s.Sync()
			} else if ev.Rune() == 'C' || ev.Rune() == 'c' {
				s.Clear()
			}
		}
	}
}
*/
