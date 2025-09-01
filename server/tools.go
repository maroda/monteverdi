package monteverdi

import "os"

// FillEnvVar returns the value of a runtime Environment Variable
func FillEnvVar(ev string) string {
	// If the EnvVar doesn't exist return a default string
	value := os.Getenv(ev)
	if value == "" {
		value = "ENOENT"
	}
	return value
}
