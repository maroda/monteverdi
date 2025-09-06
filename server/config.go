package monteverdi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
)

type ConfigFile struct {
	ID       string         `json:"id"`
	URL      string         `json:"url"`
	MWithMax map[string]int `json:"metrics"`
}

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
