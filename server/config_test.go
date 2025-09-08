package monteverdi_test

import (
	"os"
	"testing"

	Ms "github.com/maroda/monteverdi/server"
)

// Temporary OS file to use for testing configurations
func createTempFile(t testing.TB, data string) (*os.File, func()) {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "db")
	if err != nil {
		t.Fatalf("could not create temp file %v", err)
	}

	tmpfile.Write([]byte(data))
	removeFile := func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}
	return tmpfile, removeFile
}

// TODO: Test validation from here
func TestLoadConfigFileName(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{
		  "id": "NETDATA",
		  "url": "http://localhost:19999/api/v3/allmetrics",
          "delim": "=",
		  "metrics": {
		    "NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL": 10,
		    "NETDATA_APP_WINDOWSERVER_CPU_UTILIZATION_VISIBLETOTAL": 3,
		    "NETDATA_USER_MATT_CPU_UTILIZATION_VISIBLETOTAL": 10
		  }
		}]`)
	defer delConfig()
	fileName := configFile.Name()

	t.Run("Displays correct delimiter", func(t *testing.T) {
		loadConfig, err := Ms.LoadConfigFileName(fileName)
		got := loadConfig[0].Delim
		want := "="

		assertError(t, err, nil)
		assertString(t, got, want)
	})

	t.Run("Returns the correct filename when loading", func(t *testing.T) {
		loadConfig, err := Ms.LoadConfigFileName(fileName)
		got := loadConfig[0].ID
		want := "NETDATA"

		assertError(t, err, nil)
		assertString(t, got, want)
	})

	t.Run("Errors with malformed JSON", func(t *testing.T) {
		configFile, delConfig = createTempFile(t, `[{
		  "id": "NETDATA",
		  "url": "http://localhost:19999/api/v3/allmetrics",
		  "metrics": "NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL:10"
		}]`)
		defer delConfig()
		fileName = configFile.Name()

		_, err := Ms.LoadConfigFileName(fileName)
		assertGotError(t, err)
	})

	t.Run("Errors with an empty file", func(t *testing.T) {
		configFile, delConfig = createTempFile(t, ``)
		defer delConfig()
		fileName = configFile.Name()

		_, err := Ms.LoadConfigFileName(fileName)
		assertGotError(t, err)
	})

	t.Run("Errors with missing file", func(t *testing.T) {
		configFile, delConfig = createTempFile(t, ``)
		fileName = configFile.Name()
		delConfig()

		_, err := Ms.LoadConfigFileName(fileName)
		assertGotError(t, err)
	})
}
