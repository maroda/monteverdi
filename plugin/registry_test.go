package plugin_test

import (
	"testing"

	Mp "github.com/maroda/monteverdi/plugin"
)

func TestTransformerLookup(t *testing.T) {
	t.Run("Returns known transformer", func(t *testing.T) {
		known := "calc_rate"
		got, err := Mp.TransformerLookup(known)
		want := known
		assertError(t, err, nil)
		assertStringContains(t, got.Type(), want)
	})

	t.Run("Returns error if transformers don't exist", func(t *testing.T) {
		unknown := "craquemattic"
		_, err := Mp.TransformerLookup(unknown)
		assertGotError(t, err)
	})
}
