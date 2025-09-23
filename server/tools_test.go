package monteverdi_test

import (
	"os"
	"strconv"
	"testing"

	Ms "github.com/maroda/monteverdi/server"
)

func TestFillEnvVarInt(t *testing.T) {

	t.Run("returns the set default", func(t *testing.T) {
		ev := "ANYTHING"
		evDefault := 100
		want := evDefault
		got := Ms.FillEnvVarInt(ev, evDefault)

		assertInt(t, got, want)
	})

	t.Run("returns a set value", func(t *testing.T) {
		ev := "MEASUREMENT"
		evDefault := 123123
		want := evDefault

		// Set an env var to check
		err := os.Setenv(ev, strconv.Itoa(evDefault))
		assertError(t, err, nil)

		got := Ms.FillEnvVarInt(ev, evDefault)
		assertInt(t, got, want)
	})
}

func TestFillEnvVar(t *testing.T) {

	t.Run("returns a default value", func(t *testing.T) {
		ev := "ANYTHING"
		want := "ENOENT"
		got := Ms.FillEnvVar(ev)

		assertString(t, got, want)
	})

	t.Run("returns a set value", func(t *testing.T) {
		ev := "TOKEN"
		want := "ghp_1q2w3e4r5t6y7u8i9o0p"

		// Set an env var to check
		err := os.Setenv(ev, want)
		if err != nil {
			t.Errorf("could not set env var: %s", ev)
		}

		got := Ms.FillEnvVar(ev)
		assertString(t, got, want)
	})
}

// Build a URL takes an arbitrary set of pieces and combines them into a browsable URL.
func TestUrlCat(t *testing.T) {
	WebDomain := "craque.bandcamp.com"
	URIPre := "/track/"
	t.Run("Returns a URL from static strings", func(t *testing.T) {
		URIDyna := "relaxant" // This should be tested as a var that changes, too
		URIPost := ""

		want := "craque.bandcamp.com/track/relaxant"
		got := Ms.UrlCat(WebDomain, URIPre, URIDyna, URIPost)

		assertString(t, got, want)
	})

	t.Run("Returns a URL from dynamic strings inside static strings", func(t *testing.T) {
		URIPost := "/listen"
		three := []string{"relaxant", "manifold", "synapse"}

		for _, h := range three {
			want := "craque.bandcamp.com/track/" + h + "/listen"
			got := Ms.UrlCat(WebDomain, URIPre, h, URIPost)

			assertString(t, got, want)
		}
	})
}

func assertString(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
