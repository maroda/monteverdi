# Monteverdi

## Seconda Practica Observability

Monteverdi is a live data streaming system that uses _Harmonic Accent Analysis_ to identify operational pulses of the system.

This is an observability tool. It employs Leonard Meyer's musical analysis techniques to study the form present in technical (infra-)structure. System operators (like SREs or other DevOps roles) gain a multi-spacial system using hierarchical pattern recognition for understanding complex interaction dynamics. It pays attention to the _pulse_ of the system rather than the codified _rhythms_.

> **Monteverdi measures how well the whole system is breathing, rather than how much air the lungs can use.**

Traditional monitoring observes individual component metric in isolation. Monteverdi analyzes the **interaction harmonics** between system components. The hope is that this will reveal emergent patterns and early warning signals that conventional dashboards miss. Events like "near misses" and "gray failures" can become more observable as interactions of patterns propagate through the system.

## What it can do now

Streams data from multiple endpoints to draw histograms of the "accents" found by configuring triggers, i.e. a maximum value for the metric being recorded.

- The included `config.json` checks Monteverdi's local prometheus metrics only. This means you can try it without needing to have to set up a KV endpoint.
- The included `example_config.json` shows three endpoints, including the local, but also Netdata and something else I have running locally.
- Logging is found in `monteverdi.log`.

If you run `monteverdi` in the same directory as the shipped `config.json` you should get some metrics. You may need to fiddle with the max values in the config to trigger accents correctly.

### Web UI

There is now a D3 web UI at <http://localhost:8090> with several visualization features. This "radar view" shows pattern recognition only, no raw data.

Monteverdi has a warmup period before it will show any pattern recognition. This is because it checks for a minimum number of accents (currently 10) to detect patterns, and if accents aren't triggered then patterns won't be detected.

### Terminal UI

- This is the default view of Monteverdi when it is run in a TTY, parallel with the Web UI.
- Pattern recognition can be seen in the TUI if you hit 'p' for "pulse view".
- Draws the accent values in the display as they happen.
- Graphs can be clicked on to reveal the metric name and its updating _raw_ value (not the accent, which is what is shown visually). Pulse view shows only the metric name.

### API

In addition to the prometheus `/metrics` endpoint, there is now a `/version` endpoint for programmatically displaying the version in the Web UI.

### v0.3 demo

[![asciicast](https://asciinema.org/a/741649.svg)](https://asciinema.org/a/741649)

## Feature Requests

- Plugin architecture.
- Rate support for counters. This will mean some additional config file entries.
- Automatically reload config. Currently loads on first run.
- Metrics input. Can read an existing timeseries with KV and extract pulses.
- Inference. How do we take a history of pulses and define expected behaviors?
- Monitor. How do we "alert" on pulse diversion?

## How to use

Monteverdi reads any number of metrics from any number of endpoints. In the example below, I have two separate endpoints defined, each with three metrics I'm gathering.

You will need at least one endpoint that responds with a Key/Value format for metrics. Populate these into a `config.json` in the same directory as running Monteverdi.

> This will work best when using 'gauge' type metrics, where you can define a threshold that an accent is triggered. Support is coming for converting counters to rates.

The fields are:

- **id**: The unique string ID of the endpoint where you're getting metrics.
- **url**: The URL of that endpoint
- **delim**: The delimiter used to indicate "key" and "value"
- **metrics**: Each is defined as: <metric_name>: <data_maxval>, where data_maxval is the "trigger" for this metric (think alert threshold)

Here's an example from my laptop, running both Netdata and Monteverdi's prometheus endpoint simultaneously, as well as another app running in kubernetes:
```json
[{
  "id": "NETDATA",
  "url": "http://localhost:19999/api/v3/allmetrics",
  "delim": "=",
  "metrics": {
    "NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL": 10,
    "NETDATA_APP_WINDOWSERVER_CPU_UTILIZATION_VISIBLETOTAL": 20,
    "NETDATA_USER_MATT_CPU_UTILIZATION_VISIBLETOTAL": 100
  }
},
{
  "id": "VERIFICAT",
  "url": "http://verificat.rainbowq.com/metrics",
  "delim": " ",
  "metrics": {
    "go_memstats_heap_alloc_bytes": 3000000,
    "go_memstats_heap_inuse_bytes": 6000000,
    "go_memstats_heap_objects": 10000
  }
},
{
  "id": "MONTEVERDI_INTERNAL",
  "url": "http://localhost:8090/metrics",
  "delim": " ",
  "metrics": {
    "go_memstats_heap_alloc_bytes": 10000000,
    "go_memstats_heap_inuse_bytes": 12000000,
    "go_goroutines": 18,
    "go_gc_heap_objects_objects": 44000,
    "process_resident_memory_bytes": 35000000,
    "process_open_fds": 18
  }
}]
```

With that in the same directory, run Monteverdi in the terminal you prefer. Currently its default size is 80x20. :)

Browse to <http://localhost:8090> for the web interface.

### Build Flags

This is done automatically by Goreleaser but if you need to iterate in the terminal, use the following to compile the git tag into the Version:
```shell
go build -ldflags "-X github.com/maroda/monteverdi/display.Version=$(git describe --tags --always)"
```

## Known bugs

1. The Pulse View in the TUI is drawing weird, covering more space in the terminal than its configuration is supposed to be allowing.

