# Monteverdi Plugin Architecture

> **Status**: Proposed for v1.1+  
> **Current Version (v1.0)**: Gauge metrics only, no plugin system

## Philosophy

Monteverdi's core purpose is detecting harmonic patterns through accent/non-accent analysis. The **source** and **transformation** of metric data should be extensible without complicating the core analysis engine (i.e. **github.com/maroda/monteverdi/server**).

A plugin architecture allows:

- Clean separation between data acquisition and harmonic analysis
- Community contributions for specialized use cases
- Experimentation with custom patterns and transformers
- Easier implementation with multi-dimensional or non KV protocols

## Notes on Gauge-Only v1.0

The current version (up until v1.0) supports **gauge metrics only**. This means:

- Metrics must represent current state (CPU %, memory usage, queue depth)
- Monotonic counters are not supported without manual rate calculation
- Configuration only accepts simple `max` threshold values

This design choice keeps v1.0 focused on the core harmonic analysis without the complexity of stateful transformations. The plugin architecture will enable counter support and other transformations in future versions.

## Design Principles

1. Plugins are optional; core Monteverdi has no plugin dependencies.
2. Missing or broken plugins don't crash the system and fail gracefully.
3. Each type has a clear, minimal interface.
4. They are _composable_, with input and output (e.g., CalcRate → MovingAverage → Accent Detection)
5. Plugin development is meant to be community-driven and open to contributors.

## Plugin Types

> _Proposed Types for plugin ideas._

### 1. Metric Transformers

Transform raw metric data into gauges suitable for accent detection.

**Interface**:
```go
type MetricTransformer interface {
    Transform(current int64, historical []int64, timestamp time.Time) (int64, error)
    HysteresisReq() int // Number of previous values required
    Type() string      // Identifier for the transformer
}
```

**Proposed Transformers**:

| Plugin | Purpose | Input | Output |
|--------|---------|-------|--------|
| `CalcRate` | Counter → Rate | Monotonic counter | Requests/sec, errors/sec |
| `CalcPercentile` | Histogram → Percentile | Histogram buckets | p50, p95, p99 values |
| `ParseHistogram` | Multi-dimensional → Gauges | Prometheus histogram | Multiple gauge metrics |
| `MovingAverage` | Smoothing | Gauge with noise | Smoothed gauge |
| `Derivative` | Rate of change | Any metric | Change per time unit |

**Example: CalcRate**
```go
type CalcRatePlugin struct {
    previousValue map[string]int64
    previousTime  map[string]time.Time
}

func (p *CalcRatePlugin) Transform(current int64, historical []int64, timestamp time.Time) (int64, error) {
    if prev, exists := p.previousValue[metric]; exists {
        delta := current - prev
        timeDelta := timestamp.Sub(p.previousTime[metric]).Seconds()
        
        // Handle counter reset
        if delta < 0 {
            delta = current
        }
        
        rate := int64(float64(delta) / timeDelta)
        return rate, nil
    }
    
    // First reading, no rate yet
    p.previousValue[metric] = current
    p.previousTime[metric] = timestamp
    return 0, nil
}

func (p *CalcRatePlugin) HysteresisReq() int { return 1 }
func (p *CalcRatePlugin) Type() string { return "calc_rate" }
```

**Configuration Example**:
```json
{
  "id": "PROMETHEUS",
  "url": "http://localhost:8080/metrics",
  "delim": " ",
  "metrics": {
    "http_requests_total": {
      "max": 100,
      "transformer": "calc_rate",
      "rate_window": "1s"
    }
  }
}
```

### 2. Data Source Plugins

Fetch metrics from various monitoring systems.

**Interface**:
```go
type DataSource interface {
    Poll() (map[string]int64, error)
    Configure(config map[string]interface{}) error
    ID() string
}
```

**Proposed Data Sources**:

| Plugin | Purpose | Configuration |
|--------|---------|---------------|
| `PrometheusPoller` | Prometheus `/metrics` endpoint | URL, delimiter |
| `DatadogPoller` | Datadog API | API key, query |
| `CloudWatchPoller` | AWS CloudWatch | Region, namespace, metric |
| `JSONPoller` | Generic JSON API | URL, JSONPath |
| `SQLPoller` | Database queries | Connection string, query |

**Example Configuration**:
```json
{
  "id": "CLOUDWATCH",
  "source": "cloudwatch",
  "config": {
    "region": "us-west-2",
    "namespace": "AWS/EC2",
    "metric_name": "CPUUtilization",
    "instance_id": "i-1234567890abcdef0"
  },
  "metrics": {
    "cpu_utilization": { "max": 80 }
  }
}
```

### 3. Pattern Analyzers

Extend pulse detection beyond the built-in patterns.

**Interface**:
```go
type PatternAnalyzer interface {
    DetectPatterns(sequence []Ictus) ([]PulseEvent, error)
    PatternName() string
    Dimension() int // Which dimension this analyzer operates on
}
```

**Proposed Analyzers**:

| Plugin | Purpose | Detection Logic |
|--------|---------|-----------------|
| `CustomPulseDetector` | User-defined patterns | Configurable accent sequences |
| `AnomalyDetector` | Statistical outliers | Sigma-based deviation |
| `SeasonalityDetector` | Periodic cycles | FFT or autocorrelation |
| `CascadeDetector` | Multi-metric correlations | Cross-metric pattern matching |

**Example: CustomPulseDetector**
```json
{
  "pattern_analyzers": [
    {
      "type": "custom_pulse",
      "name": "double_accent",
      "sequence": [true, true, false],
      "dimension": 1
    }
  ]
}
```

## Implementation Roadmap

> _Proposed roadmap for extending Monteverdi beyond simple gauge and max operations._

### v1.0 (Current)
- ✅ Gauge metrics only
- ✅ Prometheus-style KV polling
- ✅ Built-in Iamb, Trochee, Amphibrach detection

### v1.1 (Plugin Foundation)
- [ ] Define plugin interfaces
- [ ] Plugin discovery mechanism
- [ ] Configuration schema for plugins
- [ ] `CalcRate` transformer (reference implementation)

### v1.2 (Expanded Transformers)
- [ ] Histogram/percentile transformers
- [ ] Moving average transformer
- [ ] Plugin documentation and SDK

### v2.0 (Full Plugin Ecosystem)
- [ ] Additional data source plugins
- [ ] Custom pattern analyzer support
- [ ] Plugin marketplace/registry

## For Plugin Developers

> _This is a generalized outline of what plugin development may look like._

### Creating a Transformer

1. Implement the `MetricTransformer` interface
2. Handle edge cases (missing data, resets, first reading)
3. Document configuration options
4. Provide examples with real metrics

### Creating a Data Source

1. Implement the `DataSource` interface
2. Handle authentication and errors
3. Map external metric format to Monteverdi's KV format
4. Test with rate limiting and timeouts

### Creating a Pattern Analyzer

1. Implement the `PatternAnalyzer` interface
2. Operate on `IctusSequence` or `PulseSequence`
3. Emit `PulseEvent` with appropriate dimension
4. Document when the pattern is useful

---

*For questions about plugin development, please open an issue with the `plugin-proposal` label.*