# Octez ECAD Sidecar

Octez ECAD Sidecar is a sidecar application that runs alongside a Tezos RPC node. It provides a `/health` endpoint to monitor the health status of the node based on several heuristics.

## Introduction

The Octez ECAD Sidecar is designed to enhance the reliability of Tezos RPC nodes by providing a health check endpoint. This allows load balancers to dynamically manage node availability, ensuring efficient traffic distribution and high availability.

## Purpose

The purpose of the `/health` endpoint is to allow health check probes from load balancers. This enables a load balancer to dynamically include or exclude nodes from the group of origin servers. The service is suitable for use with popular load balancer services such as those from Cloudflare, Amazon, and Google.

## Metrics

Prometheus metrics are exposed via `/metrics` endpoint.

## Getting Started

### Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/ecadlabs/octez-ecad-sc.git
   cd octez-ecad-sc
   ```

2. Build the project:

   ```bash
   go build -o octez-ecad-sc
   ```

3. Run the sidecar:

   ```bash
   ./octez-ecad-sc -c config.yaml
   ```

### Configuration

The sidecar can be configured via YAML file:

| Field                    | Default | Description                                                                       |
| ------------------------ | ------- | --------------------------------------------------------------------------------- |
| listen                   | :8080   | Host and port to listen on                                                        |
| url                      |         | Tezos RPC URL                                                                     |
| chain_id                 |         | Base58 encoded chain id                                                           |
| timeout                  | 30s     | RPC timeout                                                                       |
| tolerance                | 10s     | The amount of time added to the `minimal_block_delay` value for block observation |
| reconnect_delay          | 10s     | Delay before reconnection of a head monitor                                       |
| use_timestamps           | false   | Use blocks' timestamps instead of a system time                                   |
| poll_interval            | 15s     | Interval in whish endpoints are getting polled                                    |
| health_use_bootstrapped  | true    | If true the bootstrap state is used to produce `/health` output                   |
| health_use_block_delay   | true    | If true the block delay is used to produce `/health` output                       |

### Reporting Issues

If you encounter any issues, please create a new issue in the [GitHub issue tracker](https://github.com/ecadlabs/octez-ecad-sc/issues).

### Submitting Pull Requests

1. Fork the repository.
2. Create a new branch with a descriptive name.
3. Make your changes and commit them with clear and concise messages.
4. Push your changes to your fork.
5. Create a pull request to the main repository.

## License

This project is licensed under the Apache 2.0 License.

## Superseding Project

This sidecar project supersedes an old project: [tezos_exporter](https://github.com/ecadlabs/tezos_exporter) that ECAD used as a health check. That project is archived. Octez now exposes Prometheus metrics directly.

## Contact

For questions or support, please open an issue in the [issue tracker](https://github.com/ecadlabs/octez-ecad-sc/issues).
