# Octez ECAD Sidecar

Octez ECAD Sidecar is a sidecar application that runs alongside a Tezos RPC node. It provides a `/health` endpoint to monitor the health status of the node based on several heuristics.

## Introduction

The Octez ECAD Sidecar is designed to enhance the reliability of Tezos RPC nodes by providing a health check endpoint. This allows load balancers to dynamically manage node availability, ensuring efficient traffic distribution and high availability.

## Purpose

The purpose of the `/health` endpoint is to allow health check probes from load balancers. This enables a load balancer to dynamically include or exclude nodes from the group of origin servers. The service is suitable for use with popular load balancer services such as those from Cloudflare, Amazon, and Google.

## Features

- Provides a `/health` endpoint that returns:
  - `200 OK` if:
    - The `bootstrapped` property from the `/chains/<chain_id>/is_bootstrapped` endpoint is `true`.
    - The `synced` property from the `/chains/<chain_id>/is_bootstrapped` endpoint is `synced`.
    - A new block has been observed from the `/monitor/heads/<chain_id>` endpoint within `N + minimal_block_delay` seconds.
  - `500 Internal Server Error` if any of the above conditions fail.

## Implementation Heuristics

- Monitor the `/chains/<chain_id>/is_bootstrapped` endpoint.
- Check the `bootstrapped` and `synced` properties.
- Observe new blocks from the `/monitor/heads/<chain_id>` endpoint within a configurable time window.
- Fetch `minimal_block_delay` from the `/chains/<chain_id>/blocks/head/context/constants` endpoint.
- Monitor for changes in the protocol and update the `minimal_block_delay` constant from the constants RPC when the protocol changes.
- Configurable additional time window (`N`, default 10 seconds) added to `minimal_block_delay` for block observation.

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
   ./octez-ecad-sc --additional-time-window 10
   ```

### Configuration

The sidecar can be configured via command line flags:

- `--additional-time-window`: The amount of time added to the `minimal_block_delay` value for block observation (default: 10 seconds).

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
