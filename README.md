# Queue Simulator

<br/>
<p align="center">
        <img width="50%" height="50%" src="https://user-images.githubusercontent.com/9891776/191826624-bfc9614c-510f-4211-b4d8-cc3470e6a925.png" alt="Queue Simulator logo">
</p>

<br/>

# goqueuesim

## Table of Contents

- [Introduction](#introduction)
- [Usage](#-usage)
- [Notable Feature Gaps](#notable-feature-gaps)
- [Contributing](#contributing)

## Introduction

Simulator built at Shopify originally purposed to prototype different Checkout Queue algorithms.

Open-sourced at similar low-fidelity to its prototypical origins but readily-actionable for improved usability.

<br/>
<p align="center">
    <img width="80%" src="https://user-images.githubusercontent.com/9891776/191835121-ab33d3fa-9316-48fe-b229-ca423736953c.gif" alt="Queue Simulator Demo">
</p>
<br/>

### 🔗 &nbsp; Requirements

First, install project Go dependencies to your local environment:

```bash
go mod download
```

If you plan to run with [lua-driven](https://redis.io/commands/evalsha/) queues in Redis (ie. `queue_type = "lua_driven_bins_queue"` in [config.go](cmd/config.go)), you'll need to [install Redis](https://redis.io/docs/getting-started/installation/) and [start a server](https://redis.io/docs/getting-started/#exploring-redis-with-the-cli) in the background.

The above step is not necessary for queue types other than  `lua_driven_bins_queue`.

#### **Dashboards**

For dashboard statistics, this project hopes to eventually leverage dockerized [Prometheus](https://prometheus.io/) and [Grafana](https://grafana.com/).

However, for the moment, its statistics are built around [Datadog](https://www.datadoghq.com/) so you'll need a [Datadog API Key](https://docs.datadoghq.com/account_management/api-app-keys/).

Once you have one ready, create a `.env` file with the following:

```
DD_API_KEY=XXXX
DD_DOGSTATSD_NON_LOCAL_TRAFFIC=true
```

and then use [Docker](https://www.docker.com/) to kickoff the Datadog agent:

```bash
docker-compose up
make
```

## 📖&nbsp; Usage

For now, project usage is primarily driven by a [Makefile](https://www.gnu.org/software/make/manual/make.html) with the following commands:

|  Make Task   | Description                                                                        |
| :--------    | :--------------------------------------------------------------------------------- |
| `default`    | Compile the project binary executable & run it                                     |
| `run`        | Run the last compiled project binary executable                                    |
| `clean`      | Remove the last compiled project binary executable                                 |
| `test`       | Run tests                                                                          |

Running a simulation is thus as simple as executing `make` (build & execute) or `make run` (execute last build).

#### **Experiment Configuration**

Experiment configuration (e.g. algorithm used, client behaviour, etc.) can be modified under [cmd/goqueuesim/config.go](cmd/goqueuesim/config.go). Available parameters are documented in that file.

Additionally, client behaviour can be specified via JSON under [config/simulation/client_distributions/](config/simulation/client_distributions/).

A simple example config with 3 different client types might look like:

```json
  {
    "representation_percent": 0.20,
    "client_type": "routinely_polling_client",
    "humanized_label": "fast_poller",
    "obeys_server_poll_after": false,
    "max_initial_delay_ms": 20000,
    "max_network_jitter_ms": 100,
    "custom_int_properties": {
      "dflt_poll_interval_seconds": 1
    }
  },
  {
    "representation_percent": 0.60,
    "client_type": "routinely_polling_client",
    "humanized_label": "strictly_obedient_poller",
    "obeys_server_poll_after": true,
    "max_initial_delay_ms": 20000,
    "max_network_jitter_ms": 450
  },
  {
    "representation_percent": 0.20,
    "client_type": "fully_disappearing_client",
    "humanized_label": "immediately_exiting_client",
    "obeys_server_poll_after": false,
    "max_initial_delay_ms": 20000,
    "max_network_jitter_ms": 1,
    "custom_int_properties": {
      "polls_before_disappearing": 1
    }
  }
```

See [internal/client/impl/](internal/client/impl/) for already-implemented example `client_type`s.

## Notable Feature Gaps

The main notable feature/usability gaps that have yet to be implemented are:
- Migrating our experiment statistics to leverage dockerized [Prometheus](https://prometheus.io/) and [Grafana](https://grafana.com/) rather than [Datadog](https://www.datadoghq.com/)
- Simplifying our build dependencies (incl. [Redis](https://redis.com/)) to a single containerized [Docker](https://www.docker.com/) image

## Contributing

Please refer to the [Contributing document](CONTRIBUTING.md) if you are interested in contributing to goqueuesim!
