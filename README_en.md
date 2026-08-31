[English](README_en.md) | [Русский](README.md)

# Kaban Syncer Service

Real-time event distribution and WebSocket gateway for the Kaban ecosystem. 

This microservice acts as a communication hub, allowing internal services to push events via HTTP and broadcasting them to connected clients via WebSockets in real-time. It integrates with an external SSO service via gRPC for token validation and user authorization.

## Features

- **WebSocket Hub (`/ws`)**: Manages isolated chat/event rooms based on `workspaceId`.
- **Event Gateway (`/event`)**: HTTP POST endpoint to accept events from other microservices and broadcast them to WS clients.
- **gRPC Authentication**: Connects to an external SSO service to validate JWT tokens strictly on WebSocket connection setup.
- **High Concurrency**: Built with Go's `goroutines`, `channels`, and `sync.RWMutex` to handle massive concurrent connections without race conditions.
- **Graceful Shutdown**: Properly terminates active connections and HTTP/WS servers on OS interrupt signals.
- **Observability**: Uses structured logging (`log/slog`) and exposes `/metrics` for Prometheus.

## Tech Stack

- **Go 1.25**
- **WebSockets:** [gorilla/websocket](https://github.com/gorilla/websocket)
- **gRPC:** [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc)
- **Config Management:** [cleanenv](https://github.com/ilyakaznacheev/cleanenv)
- **Metrics:** [Prometheus Client](https://github.com/prometheus/client_golang)
- **Task Runner:** [Task](https://taskfile.dev/)

## Project Structure

```text
├── cmd
│   └── syncer          # Application entrypoint (main.go)
├── config              # YAML configurations (local, prod)
├── internal
│   ├── app             # Application assembly & layer initialization
│   │   ├── gateway     # HTTP Gateway (accepts internal events)
│   │   └── ws          # WebSocket Hub (rooms, client connections)
│   ├── config          # Configuration struct definitions
│   ├── domain          # Domain models
│   └── services
│       └── auth        # gRPC client for the SSO authentication service
├── Dockerfile          # Multi-stage Docker build
└── Taskfile.yaml       # Useful commands
```

## Setup & Running

### Requirements
- Go 1.25+
- [Task](https://taskfile.dev/) (optional, but recommended)
- A running instance of the SSO gRPC service.

### Configuration
Configuration is loaded via a YAML file. You can specify the file path via the `--config` flag or the `CONFIG_PATH` environment variable.

Example `config/local.yaml`:
```yaml
env: 'local'
auth:
  address: "localhost:44044" # SSO gRPC address
  timeout: "5s"
http:
  enable_cors: true
  api_key: "dev-secret-key"  # Gateway protection key
ws:
  port: 8888
  ws-max-message-size: 1048576
  ws-send-buffer-size: 256
  log-data-stream: true
```

### Running Locally

Using Task:
```bash
task start:dev
```

Using raw Go command:
```bash
go run ./cmd/syncer --config=./config/local.yaml
```

### Docker
To build and run via Docker:
```bash
docker build -t kaban-syncer .
docker run -p 8888:8888 -e CONFIG_PATH=./config/prod.yaml kaban-syncer
```

## API Endpoints

### 1. Connect to WebSocket
**GET** `/ws?workspaceId={id}&userId={id}&token={jwt}`
- Upgrades the connection to WebSocket.
- The `token` is immediately validated against the SSO gRPC service.
- Clients are isolated into rooms by `workspaceId`.

### 2. Publish Event
**POST** `/event`
- **Header:** `x-api-key: {api_key}`
- **Body (JSON):**
```json
{
  "workspaceId": "ws-123",
  "userId": "user-456",
  "type": "task_updated",
  "payload": { ... },
  "rev": 1
}
```
*Note: The event will be broadcasted to all users in the specified workspace, except the user who initiated the event (if specified).*

### 3. Metrics
**GET** `/metrics`
- Exposes Prometheus metrics.

## Testing

The project is fully covered with unit tests using Go's `httptest` and asynchronous channel testing for the WebSocket Hub.

Run tests:
```bash
go test ./... -v
```
