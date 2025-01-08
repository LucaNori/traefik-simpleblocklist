# SimpleBlocklist

Simple plugin for [Traefik](https://github.com/containous/traefik) to block requests based on a list of IP addresses.

## Configuration

The plugin can be installed locally or through Traefik Pilot.

### Configuration as local plugin

This example assumes that your Traefik instance runs in a Docker container using the [official image](https://hub.docker.com/_/traefik/).

Download the plugin and save it to a location the Traefik container can reach. The plugin source should be mapped into the container via a volume bind mount:

#### `docker-compose.yml`

```yml
services:
  traefik:
    image: traefik

    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /docker/config/traefik/traefik.yml:/etc/traefik/traefik.yml
      - /docker/config/traefik/dynamic-configuration.yml:/etc/traefik/dynamic-configuration.yml
      - /docker/config/traefik/plugin/simpleblocklist:/plugins-local/src/github.com/LucaNori/traefik-simpleblocklist/
      - /docker/config/traefik/blacklist.txt:/etc/traefik/blacklist.txt

    ports:
      - "80:80"

  hello:
    image: containous/whoami
    labels:
      - traefik.enable=true
      - traefik.http.routers.hello.entrypoints=http
      - traefik.http.routers.hello.rule=Host(`hello.localhost`)
      - traefik.http.services.hello.loadbalancer.server.port=80
      - traefik.http.routers.hello.middlewares=my-plugin@file
```

#### `traefik.yml`

```yml
log:
  level: INFO

experimental:
  localPlugins:
    simpleblocklist:
      moduleName: github.com/LucaNori/traefik-simpleblocklist
```

#### `dynamic-configuration.yml`

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

## Example Docker Configuration

```yml
version: "3"
networks:
  proxy:
    external: true
services:
  traefik:
    image: traefik:latest
    container_name: traefik
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    networks:
      proxy:
        aliases:
          - traefik
    ports:
      - 80:80
      - 443:443
    volumes:
      - "/etc/timezone:/etc/timezone:ro"
      - "/etc/localtime:/etc/localtime:ro"
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "/docker/config/traefik/traefik.yml:/etc/traefik/traefik.yml:ro"
      - "/docker/config/traefik/dynamic-configuration.yml:/etc/traefik/dynamic-configuration.yml"
      - "/docker/config/traefik/blacklist.txt:/etc/traefik/blacklist.txt:ro"
```