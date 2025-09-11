package monteverdi_test

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

// Temporary OS file to use for testing configurations
func createTempFile(t testing.TB, data string) (*os.File, func()) {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "db")
	if err != nil {
		t.Fatalf("could not create temp file %v", err)
	}
	assertError(t, err, nil)

	tmpfile.Write([]byte(data))
	removeFile := func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}
	return tmpfile, removeFile
}

// TODO: DEPRECATE
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

// No config file needed here, we're testing for no file
func TestLoadConfigFileNameWithFS(t *testing.T) {
	mockFS := MockFS{OpenError: true}
	_, err := Ms.LoadConfigFileNameWithFS("anyfilename", mockFS)
	assertGotError(t, err)
	assertStringContains(t, err.Error(), "could not open file")
}

// Config file needed here, we're testing for bad file
func TestValidateLoadWithFS_Error(t *testing.T) {
	configFile, delConfig := createTempFile(t, `[{fake}]`)
	defer delConfig()

	t.Run("Returns error if file does not exist", func(t *testing.T) {
		mockFS := MockFS{StatError: true}
		err := Ms.ValidateLoadWithFS(configFile, mockFS)
		assertGotError(t, err)
		assertStringContains(t, err.Error(), "could not stat file")
	})

	t.Run("Returns error if file is empty", func(t *testing.T) {
		mockFS := MockFS{FileSize: 0}
		err := Ms.ValidateLoadWithFS(configFile, mockFS)
		assertGotError(t, err)
		assertStringContains(t, err.Error(), "file is empty")
	})
}

// MockFS for dependency injection on FileSystem to test config file
type MockFS struct {
	OpenError bool
	StatError bool
	FileSize  int64
}

func (fs MockFS) Open(name string) (*os.File, error) {
	if fs.OpenError {
		return nil, errors.New("mock: could not open file")
	}

	if runtime.GOOS == "windows" {
		return os.Open("NUL")
	}
	return os.Open("/dev/null")
}

func (fs MockFS) Stat(file *os.File) (os.FileInfo, error) {
	if fs.StatError {
		return nil, errors.New("mock: could not stat file")
	}

	return &MockFileInfo{size: fs.FileSize}, nil
}

type MockFileInfo struct {
	size int64
}

func (fi MockFileInfo) Size() int64        { return fi.size }
func (fi MockFileInfo) Name() string       { return "mock-file" }
func (fi MockFileInfo) Mode() os.FileMode  { return 0644 }
func (fi MockFileInfo) ModTime() time.Time { return time.Now() }
func (fi MockFileInfo) IsDir() bool        { return false }
func (fi MockFileInfo) Sys() interface{}   { return nil }
