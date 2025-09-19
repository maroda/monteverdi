# Monteverdi

## Seconda Practica Observability

Monteverdi is a live data streaming system that uses _Harmonic Accent Analysis_ to identify operational pulses of the system.

This is an observability tool. It employs Leonard Meyer's musical analysis techniques to study the form present in technical (infra-)structure. System operators (like SREs or other DevOps roles) gain a multi-spacial system using hierarchical pattern recognition for understanding complex interaction dynamics. It pays attention to the _pulse_ of the system rather than the codified _rhythms_.

> **Monteverdi measures how well the whole system is breathing, rather than how much air the lungs can use.**

Traditional monitoring observes individual component metric in isolation. Monteverdi analyzes the **interaction harmonics** between system components. The hope is that this will reveal emergent patterns and early warning signals that conventional dashboards miss. Events like "near misses" and "gray failures" can become more observable as interactions of patterns propagate through the system.

## What it can do now

- **Pulse View is now working!** Pattern recognition can be seen floating by if you hit 'p'.
- Streams data from multiple endpoints to draw histograms of the "accents" found by configuring triggers, i.e. a maximum value for the metric being recorded.
- Draws the accent values in the display as they happen.
- Graphs can be clicked on to reveal the metric name and its updating _raw_ value (not the accent, which is what is shown visually). Pulse view shows only the metric name.

### v0.3 demo

[![asciicast](https://asciinema.org/a/741649.svg)](https://asciinema.org/a/741649)

## Feature Requests

- Rate support for counters. This will mean some additional config file entries.
- Automatically reload config. Currently loads on first run.
- Add a Layer of Hierarchy. Pulses of the pulses.
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

Here's an example from my laptop, running both Netdata and Monteverdi's prometheus endpoint simultaneously. I'm retrieving just three per endpoint as the app streams the data (where thousands of metrics appear):

```json
[{
  "id": "NETDATA",
  "url": "http://localhost:19999/api/v3/allmetrics",
  "delim": "=",
  "metrics": {
    "NETDATA_USER_ROOT_CPU_UTILIZATION_VISIBLETOTAL": 10,
    "NETDATA_APP_WINDOWSERVER_CPU_UTILIZATION_VISIBLETOTAL": 10,
    "NETDATA_USER_MATT_CPU_UTILIZATION_VISIBLETOTAL": 100
  }
},
  {
    "id": "PROMETHEUS",
    "url": "http://localhost:8080/metrics",
    "delim": " ",
    "metrics": {
      "go_memstats_heap_released_bytes": 4000000,
      "go_memstats_heap_inuse_bytes": 10000000,
      "go_memstats_next_gc_bytes": 12000000
    }
  }]
```

With that in the same directory, run Monteverdi in the terminal you prefer. Currently its default size is 80x20. :)

## Known bugs

1. The JSON config is quite finnicky, it has not been extensively tested on more than two sources.

