# SimpleBlocklist

Simple plugin for [Traefik](https://github.com/containous/traefik) to block requests based on a list of IP addresses and networks.

## Configuration

The plugin can be installed through Traefik Pilot or as a local plugin.

### Installation through Traefik Pilot

You can install this plugin directly through the Traefik Pilot web interface or by adding it to your Traefik static configuration:

```yaml
experimental:
  plugins:
    simpleblocklist:
      moduleName: github.com/LucaNori/traefik-simpleblocklist
      version: v0.1.2
```

### Installation as Local Plugin

For development or testing, you can install the plugin locally. This example assumes that your Traefik instance runs in a Docker container using the [official image](https://hub.docker.com/_/traefik/).

#### `docker-compose.yml`

```yml
services:
  traefik:
    image: traefik:v2.10
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./traefik.yml:/etc/traefik/traefik.yml
      - ./dynamic-config.yml:/etc/traefik/dynamic-config.yml
      - ./plugins-local/src/github.com/LucaNori/traefik-simpleblocklist:/plugins-local/src/github.com/LucaNori/traefik-simpleblocklist
      - ./blacklist.txt:/etc/traefik/blacklist.txt
    ports:
      - "80:80"
      - "8080:8080"  # Dashboard

  whoami:
    image: traefik/whoami
    labels:
      - traefik.enable=true
      - traefik.http.routers.whoami.rule=Host(`whoami.localhost`)
      - traefik.http.routers.whoami.middlewares=simpleblocklist@file
```

#### `traefik.yml`

```yml
api:
  dashboard: true
  insecure: true  # For testing only

log:
  level: INFO

entryPoints:
  web:
    address: ":80"

providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
  file:
    directory: /etc/traefik
    watch: true

experimental:
  localPlugins:
    simpleblocklist:
      moduleName: github.com/LucaNori/traefik-simpleblocklist
```

#### `dynamic-config.yml`

```yml
http:
  middlewares:
    simpleblocklist:
      plugin:
        simpleblocklist:
          blacklistPath: "/etc/traefik/blacklist.txt"
          allowLocalRequests: true
          logLocalRequests: false
          httpStatusCodeDeniedRequest: 403
```

### Blacklist File Format

The blacklist file supports both individual IP addresses and CIDR notation for blocking IP ranges. Each entry should be on a new line. Comments (lines starting with #) and empty lines are ignored.

Example blacklist.txt:

```text
# Block individual IPs
192.0.2.1
203.0.113.2

# Block entire networks
198.51.100.0/24  # Blocks all IPs from 198.51.100.0 to 198.51.100.255
172.16.0.0/16    # Blocks a larger network range

# IPv6 addresses and networks are also supported
2001:db8::1
2001:db8::/32
```

## Configuration Options

### `blacklistPath` (required)
Path to the file containing the list of IP addresses and networks to block. Supports both individual IPs and CIDR notation.

### `allowLocalRequests` (optional)
If set to true, will not block requests from private IP ranges (default: true)

### `logLocalRequests` (optional)
If set to true, will log every connection from any IP in the private IP range (default: false)

### `httpStatusCodeDeniedRequest` (optional)
HTTP status code to return when a request is denied (default: 403)

## Features

- Blocks individual IP addresses and entire networks using CIDR notation
- Supports both IPv4 and IPv6 addresses
- Allows comments in the blacklist file for better organization
- Handles X-Forwarded-For, X-Real-IP, and RemoteAddr headers for reliable IP detection
- Configurable handling of local/private network requests
- Customizable HTTP status code for denied requests

## Development

To build and test the plugin locally:

1. Clone this repository
2. Build the plugin: `go build ./...`
3. Run tests: `go test ./...`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
