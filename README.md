# Monteverdi

Seconda Practica Reliability Observability

## What This Is

Monteverdi is a live data streaming system that uses _Harmonic Accent Analysis_ to identify patterns and relate them to each other.

This is an observability tool that applies Leonard Meyer's musical analysis techniques to distributed systems monitoring. The idea is that it provides operators (like SREs) with multi-spacial pattern recognition for understanding complex interaction dynamics.

Traditional monitoring observes individual component metric in isolation. Monteverdi analyzes the **interaction harmonics** between system components. The hope is that this will reveal emergent patterns and early warning signals that conventional dashboards miss. Events like "near misses" and "gray failures" can become more observable as interactions of patterns propagate through the system.

## What it can do now

- It is optimized to stream data from multiple endpoints and draw histograms of the "accents" found by configuring triggers, i.e. a maximum value for the metric being recorded.
- Does not save these accent histograms anywhere yet.
- Can take the metrics and maxvals from the configured endpoints and draw the accent values in the display as they happen.
- These graphs can be clicked on to reveal the metric name and its updating _raw_ value (not the accent, which is what is shown visually).
- The blocky runes used in the drawing are being colored by static ranges right now, so very large numbers just look red. This may or may not change, since the important part is only the accent.

## What is to come

- Pattern recognition via ML (GoLearn or others)
- Graphics to show pattern recognition and interaction relationships
- Histogram saving

## How to use

Monteverdi reads any number of metrics from any number of endpoints, but right now the display is optimized for six graphs. In the example below, I have two separate endpoints defined.

You will need at least one endpoint that responds with a Key/Value format for metrics. Populate these into a `config.json` in the same directory as running Monteverdi.

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
