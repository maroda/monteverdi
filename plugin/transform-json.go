package plugin

/*
	JSONKey

	This plugin allows for JSON to be used as the metric data.

	Returns a positive integer from the value of a key inside a JSON object

	This expects the /metric/ used by Transform to contain the entire JSON
*/

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type JSONKeyPlugin struct {
	MetricKey string
}

// NewJSONTransformer returns a struct for what to search in the JSON
func NewJSONTransformer(mk string) *JSONKeyPlugin {
	return &JSONKeyPlugin{MetricKey: mk}
}

// Transform extracts the JSONKeyPlugin key from the JSON object
// held as the full 'metric', i.e. the entire JSON response from ParseMetricKV
func (tj *JSONKeyPlugin) Transform(metric string, current int64, historical []int64, timestamp time.Time) (int64, error) {

	var data interface{}
	if err := json.Unmarshal([]byte(metric), &data); err != nil {
		slog.Error("Error unmarshalling json",
			slog.String("search", tj.MetricKey),
			slog.String("json", metric),
			slog.Any("error", err))
		return 0, fmt.Errorf("error unmarshalling json from metric: %v", err)
	}

	value, err := ExtractValue(data, tj.MetricKey)
	if err != nil {
		return 0, fmt.Errorf("error extracting json value from metric: %v", err)
	}

	return value, nil
}

func ExtractValue(data interface{}, metric string) (int64, error) {
	keys := strings.Split(metric, ".")
	current := data

	for _, key := range keys {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[key]
			if !ok {
				return 0, fmt.Errorf("key %s not found", key)
			}
		case []interface{}:
			return 0, fmt.Errorf("array indexing not implemented yet")
		default:
			return 0, fmt.Errorf("cannot traverse into type %T at key %s", v, key)
		}
	}

	// Convert final value to int64
	switch v := current.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			f, err := v.Float64()
			if err != nil {
				return 0, fmt.Errorf("error converting json.Number to int64: %v", err)
			}
			return int64(f), nil
		}
		return i, nil
	default:
		return 0, fmt.Errorf("value not numeric, cannot traverse %T", v)
	}
}

func (tj *JSONKeyPlugin) HysteresisReq() int { return -1 } // Not applicable
func (tj *JSONKeyPlugin) Type() string       { return "json_key" }
