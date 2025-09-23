package monteverdi

import (
	"log/slog"
	"os"
	"strconv"
)

// FillEnvVar returns the value of a runtime Environment Variable
func FillEnvVar(ev string) string {
	// If the EnvVar doesn't exist return a default string
	value := os.Getenv(ev)
	if value == "" {
		value = "ENOENT"
	}
	return value
}

// FillEnvVarInt returns a runtime Environment Variable as an int
// It takes the name of the ENV VAR and a default
// For non-default and string ENV VARs, use FillEnvVar()
func FillEnvVarInt(ev string, def int) int {
	fetch := os.Getenv(ev)
	if fetch == "" {
		return def
	}

	value, err := strconv.Atoi(fetch)
	if err != nil || value < 0 {
		slog.Info("Invalid environment variable " + ev)
		return def
	}
	return value
}

// UrlCat is variadic, concatenating any set of strings into a URL.
// It can be used to embed a dynamic string alongside static parts of a URI.
// /u/ is a slice of strings used to build completeURL
func UrlCat(u ...string) string {
	var completeURL string
	for _, p := range u {
		completeURL = completeURL + p
	}
	slog.Info("New endpoint", slog.String("URL", completeURL))
	return completeURL
}
