# starter-plugin

Minimal bizzy plugin using the Go `pluginsdk`.

## What it provides

- Plugin name: `starter`
- Tool: `plugin.starter.echo`

## Run

From repo root, start the server first:

```bash
make start
```

In another terminal:

```bash
cd plugins/starter-plugin
NATS_URL=nats://127.0.0.1:4225 go run .
```

`NATS_URL` is optional; this plugin defaults to `nats://127.0.0.1:4225`.

## Build

```bash
cd plugins/starter-plugin
make build
```
