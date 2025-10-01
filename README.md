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

- There is a D3 web UI at <http://localhost:8090> with several visualization features. This "radar view" shows pattern recognition only, no raw data.
- Monteverdi has a warmup period before it will show any pattern recognition. This is because it checks for a minimum number of accents (currently 10) to detect patterns, and if accents aren't triggered then patterns won't be detected.
- If no TTY is detected the logging is directed to STDOUT
 
### Terminal UI

- This is the default view of Monteverdi when it is run in a TTY, parallel with the Web UI.
- There is now a `-headless` runtime flag for no TTY (i.e. containers).
- Draws the accent values in the display as they happen.
- Graphs can be clicked on to reveal the metric name and its updating _raw_ value (not the accent, which is what is shown visually).
- Pattern recognition can be seen in the TUI if you hit 'p' for "pulse view" (but it is buggy, see known issues below).
- Pulse view shows only the metric name.

### API

- In addition to the prometheus `/metrics` endpoint, there is now a `/version` endpoint for programmatically displaying the version in the Web UI.
- This API starts up regardless of a TUI or Web UI runtime
- The app should no longer block if Endpoints are unreachable and log the error.

## Feature Requests

- [Plugin Architecture](./PLUGINS.md) to support things like rates and non-KV.
- Better support for Env Vars (config file, logging setup, headless, etc)
- Automatically reload config. Currently loads on first run.
- Metrics input. Can read an existing timeseries with KV and extract pulses.
- Inference. How do we take a history of pulses and define expected behaviors?
- Monitor. How do we "alert" on pulse diversion?

## How to use

Monteverdi reads any number of metrics from any number of endpoints.

You will need at least one endpoint configured. It should respond with a Key/Value format for metrics (e.g. Netdata, Prometheus). Populate these into a `config.json` in the same directory as running Monteverdi.

> This will work best when using 'gauge' type metrics, where you define a threshold to trigger an accent. Plugin support is coming for converting counters to rates.

The fields are:

- **id**: The unique string ID of the endpoint where you're getting metrics. Small is best, it is displayed in the UI.
- **url**: The URL of that endpoint.
- **delim**: The delimiter used to indicate "key" and "value".
- **metrics**: Each is defined as: <metric_name>: <data_maxval>, where data_maxval is the "trigger" for this metric to record an accent.

> See `example_config.json` for a complex example, or `config.json` to play around with Monteverdi's own Prometheus stats.

With a valid `config.json` in the same directory, run Monteverdi in the terminal you prefer.

Currently its default size is 80x20, but you can set it to wider in your environment with:
```shell
MONTEVERDI_TUI_TSDB_VISUAL_WINDOW=100
```

Browse to <http://localhost:8090> for the web interface.

### Build Flags

This is done automatically by Goreleaser but if you need to iterate in the terminal, use the following to compile the git tag into the Version:
```shell
go build -ldflags "-X github.com/maroda/monteverdi/display.Version=$(git describe --tags --always)"
```

## Docker

This repo builds public container packages that you can use to try Monteverdi out for yourself. They can be run like this:
```shell
docker run -p 8090:8090 --network host ghcr.io/maroda/monteverdi:latest
```

The included `config.json` checks monteverdi's own `/metrics` endpoint, which requires the `--network host` part so that `localhost` works.

External (public) addresses won't have this problem.

To use your own config, set up the JSON and pass it to the container:
```shell
docker run -p 8090:8090 -v ./myconfig.json:/app/config.json ghcr.io/maroda/monteverdi:latest
```

## Known bugs

1. The Pulse View in the TUI is drawing weird, covering more space in the terminal than its configuration is supposed to be allowing.

