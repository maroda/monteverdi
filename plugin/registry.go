package plugin

import "fmt"

// Transformers is a global map of MetricTransformer plugins.
var Transformers = map[string]func() MetricTransformer{
	"calc_rate": func() MetricTransformer {
		return &CalcRatePlugin{}
	},
}

func TransformerLookup(name string) (MetricTransformer, error) {
	factory, ok := Transformers[name]
	if !ok {
		return nil, fmt.Errorf("unknown transformer: %s", name)
	}
	return factory(), nil
}
