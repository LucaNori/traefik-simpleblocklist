# SimpleBlocklist

Simple plugin for [Traefik](https://github.com/containous/traefik) to block requests based on a list of IP addresses.

## Configuration

The plugin can be installed through Traefik Pilot or as a local plugin.

### Installation through Traefik Pilot

You can install this plugin directly through the Traefik Pilot web interface or by adding it to your Traefik static configuration:

```yaml
experimental:
  plugins:
    simpleblocklist:
      moduleName: github.com/LucaNori/traefik-simpleblocklist
      version: v0.1.0
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

The blacklist file should contain one IP address per line. For example:

```text
192.0.2.1
203.0.113.2
198.51.100.3
```

Empty lines and malformed IP addresses are ignored.

## Configuration Options

### `blacklistPath` (required)
Path to the file containing the list of IP addresses to block. Each IP should be on a new line.

### `allowLocalRequests` (optional)
If set to true, will not block requests from private IP ranges (default: true)

### `logLocalRequests` (optional)
If set to true, will log every connection from any IP in the private IP range (default: false)

### `httpStatusCodeDeniedRequest` (optional)
HTTP status code to return when a request is denied (default: 403)

## Development

To build and test the plugin locally:

1. Clone this repository
2. Build the plugin: `go build ./...`
3. Run tests: `go test ./...`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
