package monteverdi

import (
	"os"
	"testing"
)

func TestFillEnvVar(t *testing.T) {

	t.Run("returns a default value", func(t *testing.T) {
		ev := "ANYTHING"
		want := "ENOENT"
		got := FillEnvVar(ev)

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

		got := FillEnvVar(ev)
		assertString(t, got, want)
	})
}

func assertString(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
