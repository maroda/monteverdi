package monteverdi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
)

// ConfigFile contains the options to configure Endpoints
type ConfigFile struct {
	ID       string         `json:"id"`      // Unique string ID
	URL      string         `json:"url"`     // Endpoint URL (no auth yet)
	Delim    string         `json:"delim"`   // Stats delimiter (eg: "=")
	MWithMax map[string]int `json:"metrics"` // Value to trigger an accent
}

// FileSystem is for operating with local configs and/or data.
type FileSystem interface {
	Open(name string) (*os.File, error)
	Stat(file *os.File) (os.FileInfo, error)
}

type RealFS struct{}

func (fs RealFS) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (fs RealFS) Stat(file *os.File) (os.FileInfo, error) {
	return file.Stat()
}

// LoadConfigFileNameWithFS takes the filename from the fs and validates before loading the config
func LoadConfigFileNameWithFS(filename string, fs FileSystem) ([]ConfigFile, error) {
	file, err := fs.Open(filename)
	if err != nil {
		slog.Error("Could not open config file", slog.String("Filename", filename))
		return nil, err
	}
	defer file.Close()

	// validation
	err = ValidateLoadWithFS(file, fs)
	if err != nil {
		slog.Error("Validation failed", slog.Any("Error", err))
		return nil, err
	}

	// if validation passes, we're good to load the config
	return LoadConfigWithFS(file, fs)
}

// ValidateLoadWithFS returns an error on issue
func ValidateLoadWithFS(file *os.File, fs FileSystem) error {
	// validate file
	info, err := fs.Stat(file)
	if err != nil {
		slog.Error("could not stat file")
		return err
	}

	// validate size is not zero
	if info.Size() == 0 {
		slog.Error("file is empty")
		return errors.New("file is empty")
	}

	return nil
}

// LoadConfigWithFS is the final step for validating and opening the config file and pulling it into a struct
func LoadConfigWithFS(file *os.File, fs FileSystem) ([]ConfigFile, error) {
	file.Seek(0, 0)

	// decode json
	var config []ConfigFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		slog.Error("could not decode file")
		return nil, err
	}

	return config, nil
}

// LoadConfigFileName is a wrapper for backward compatibility for now
func LoadConfigFileName(filename string) ([]ConfigFile, error) {
	return LoadConfigFileNameWithFS(filename, RealFS{})
}

//////

/*
// LoadConfigFileName pulls a given filename config off local disk
// Validation is performed on the file before opening
func LoadConfigFileName(filename string) ([]ConfigFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// validation
	err = validateLoad(file)
	if err != nil {
		slog.Error("Validation failed", slog.Any("Error", err))
		return nil, err
	}

	return LoadConfig(file)
}

func validateLoad(file *os.File) error {
	// validate file
	info, err := file.Stat()
	if err != nil {
		slog.Error("could not stat file")
		return err
	}

	// validate size
	if info.Size() == 0 {
		slog.Error("file is empty")
		return errors.New("file is empty")
	}

	return nil
}

func LoadConfig(file *os.File) ([]ConfigFile, error) {
	// open file
	cf, err := os.Open(file.Name())
	if err != nil {
		slog.Error("could not open file")
		return nil, err
	}
	defer cf.Close()

	// decode json
	var config []ConfigFile
	decoder := json.NewDecoder(cf)
	if err := decoder.Decode(&config); err != nil {
		slog.Error("could not decode file")
		return nil, err
	}

	return config, nil
}
*/
