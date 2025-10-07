# Monteverdi

[![Release](https://github.com/maroda/monteverdi/actions/workflows/release.yml/badge.svg)](https://github.com/maroda/monteverdi/actions/workflows/release.yml)

## Seconda Practica Observability

Monteverdi is a live data streaming system that uses _Harmonic Accent Analysis_ to identify operational pulses of the system.

This is an observability tool. It employs Leonard Meyer's musical analysis techniques to study the form present in technical (infra-)structure. System operators (like SREs or other DevOps roles) gain a multi-spacial system using hierarchical pattern recognition for understanding complex interaction dynamics. It pays attention to the _pulse_ of the system rather than the codified _rhythms_.

> **Monteverdi measures how well the whole system is breathing, rather than how much air the lungs can use.**

Traditional monitoring observes individual component metric in isolation. Monteverdi analyzes the **interaction harmonics** between system components. The hope is that this will reveal emergent patterns and early warning signals that conventional dashboards miss. Events like "near misses" and "gray failures" can become more observable as interactions of patterns propagate through the system.

## What it can do now

Streams data from multiple endpoints to draw histograms of the "accents" found by configuring triggers, i.e. a maximum value for the metric being recorded.

> NEW! Metrics are now configurable as **COUNTERS** using the new Plugin architecture. Using the `transformer="calc_rate"` entry in the config will do the job (see the included configs for examples).

- The included `config.json` checks Monteverdi's local prometheus metrics only. This means you can try it without needing to have to set up a KV endpoint.
- The included `example_config.json` shows three endpoints, including the local, but also Netdata and something else I have running locally.
- Logging is found in `monteverdi.log`.

If you run `monteverdi` in the same directory as the shipped `config.json` you should get some metrics. You may need to fiddle with the max values in the config to trigger accents correctly.

### Web UI

![Web Interface](docs/images/monteverdi-webui-preview-v0.9.png)

- There is a D3 web UI at <http://localhost:8090> with several visualization features. This "radar view" shows pattern recognition only, no raw data.
- Monteverdi has a warmup period before it will show any pattern recognition. This is because it checks for a minimum number of accents (currently 10) to detect patterns, and if accents aren't triggered then patterns won't be detected.
- If no TTY is detected the logging is directed to STDOUT
 
### Terminal UI

![Terminal Interface](docs/images/monteverdi-tui-preview-v0.9.png)

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
- Automatically reload config. Currently loads on first run.
- Metrics input. Can read an existing timeseries with KV and extract pulses.
- Inference. How do we take a history of pulses and define expected behaviors?
- Monitor. How do we "alert" on pulse diversion?
- Output. Can the pulses be converted to other formats?
- Audio. How do the patterns sound?

## How to use

### Configuration File
Monteverdi reads any number of metrics from any number of endpoints.

You will need at least one endpoint configured. It should respond with a Key/Value format for metrics (e.g. Netdata, Prometheus). Populate these into a `config.json` in the same directory as running Monteverdi.

The fields are:

- **id**: The unique string ID of the endpoint where you're getting metrics. Small is best, it is displayed in the UI.
- **url**: The URL of that endpoint.
- **delim**: The delimiter used to indicate "key" and "value".
- **metrics**: Gauges or Counters (with a built-in Rate Transformer Plugin), see examples for details.

> See `example_config.json` for a complex example, or `config.json` to play around with Monteverdi's own Prometheus stats.

Once you have this populated, use the runtime or docker options below to point Monteverdi at your config.

### Runtime

Refer to the command help for any special configurations you want to make. The options and environment variables are listed:
```shell
>>> ./monteverdi -help
Monteverdi - Seconda Practica Observability

Usage: ./monteverdi [options]

Options:
  -config string
    	Path to configuration JSON (default "config.json")
  -headless
    	Container mode: no Terminal UI, logs sink to STDOUT

Environment Variables:
  MONTEVERDI_CONFIG_FILE
        Path to configuration file (default: config.json)
  MONTEVERDI_LOGLEVEL
        Log level: debug or info (default: info)
  MONTEVERDI_TUI_TSDB_VISUAL_WINDOW
        TUI display width in characters (default: 80)
  MONTEVERDI_PULSE_WINDOW_SECONDS
        Pulse lifecycle window in seconds (default: 3600)

Examples:
  ./monteverdi -config=/path/to/config.json
  ./monteverdi -headless
  MONTEVERDI_CONFIG_FILE=myconfig.json ./monteverdi

Run with no options to start the terminal UI with webserver (port 8090).
There is a short warmup before pulses will appear in the web UI.
Logs sink to ./monteverdi.log unless in -headless mode.
```

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

> The included `config.json` checks monteverdi's own `/metrics` endpoint, which requires the `--network host` part so that `localhost` works. When using public hostnames in `config.json`, this is not necessary.

To use your own config, set up the JSON and pass it to the container as a mount:
```shell
docker run -p 8090:8090 -v ./myconfig.json:/app/config.json ghcr.io/maroda/monteverdi:latest
```

Or use a different filename with an environment variable:
```shell
docker run -p 8090:8090 \
  -e MONTEVERDI_CONFIG_FILE=/app/myconfig.json \
  -v ./myconfig.json:/app/myconfig.json \
  ghcr.io/maroda/monteverdi:latest
```

## Kubernetes

You should be able to use a ConfigMap to run this in a Kubernetes cluster. This is an untested config, but should work!

Create the ConfigMap from your local JSON:
```shell
kubectl create configmap monteverdi-config --from-file=config.json
```

Once that is in place, an example manifest should look like:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: monteverdi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: monteverdi
  template:
    metadata:
      labels:
        app: monteverdi
    spec:
      containers:
      - name: monteverdi
        image: ghcr.io/maroda/monteverdi:latest
        args: ["-headless"]
        ports:
        - containerPort: 8090
        volumeMounts:
        - name: config
          mountPath: /app/config.json
          subPath: config.json
      volumes:
      - name: config
        configMap:
          name: monteverdi-config
---
apiVersion: v1
kind: Service
metadata:
  name: monteverdi
spec:
  selector:
    app: monteverdi
  ports:
  - port: 8090
    targetPort: 8090
  type: LoadBalancer
```


## Known bugs

1. The Pulse View in the TUI is drawing weird, covering more space in the terminal than its configuration is supposed to be allowing.
2. Clicking on pulses in WebUI is inconsistent, sometimes the metadata popup will work, sometimes it's really difficult to trigger.

