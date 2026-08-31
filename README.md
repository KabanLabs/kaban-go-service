[English](README_en.md) | [Русский](README.md)

# Kaban Syncer Service

Сервис маршрутизации событий (Gateway) и управления WebSocket-соединениями для экосистемы Kaban.

Этот микросервис выполняет роль коммуникационного узла: он принимает события от других внутренних сервисов через HTTP и в реальном времени рассылает их подключенным клиентам по WebSockets. Для проверки токенов и авторизации пользователей сервис интегрирован с внешним SSO-сервисом через gRPC.

## Особенности

- **WebSocket Hub (`/ws`)**: Изолированные комнаты на основе `workspaceId`.
- **Event Gateway (`/event`)**: HTTP POST эндпоинт для приема событий от микросервисов и мгновенного броадкаста подключенным WS-клиентам.
- **gRPC Авторизация**: Строгая валидация JWT-токенов через внешний сервис SSO при установке WebSocket-соединения.
- **Конкурентность (High Concurrency)**: Написано с использованием горутин, каналов и `sync.RWMutex`, что исключает состояние гонки (race conditions) при большом числе подключений.
- **Graceful Shutdown**: Корректное завершение работы сервиса и разрыв активных соединений при получении системных сигналов.
- **Observability**: Структурированное логирование через `log/slog` и сбор метрик Prometheus (`/metrics`).

## Стек технологий

- **Go 1.25**
- **WebSockets:** [gorilla/websocket](https://github.com/gorilla/websocket)
- **gRPC:** [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc)
- **Конфигурация:** [cleanenv](https://github.com/ilyakaznacheev/cleanenv)
- **Метрики:** [Prometheus Client](https://github.com/prometheus/client_golang)
- **Task Runner:** [Task](https://taskfile.dev/)

## Структура проекта

```text
├── cmd
│   └── syncer          # Точка входа в приложение (main.go)
├── config              # YAML конфигурации (local, prod)
├── internal
│   ├── app             # Сборка приложения и инициализация слоев
│   │   ├── gateway     # HTTP Gateway (прием внутренних событий)
│   │   └── ws          # WebSocket Hub (комнаты, коннекты клиентов)
│   ├── config          # Структуры конфигурации
│   ├── domain          # Доменные модели
│   └── services
│       └── auth        # gRPC клиент для сервиса SSO
├── Dockerfile          # Многоэтапная (multi-stage) сборка Docker
└── Taskfile.yaml       # Команды для быстрого запуска
```

## Запуск проекта

### Требования
- Go 1.25+
- [Task](https://taskfile.dev/) (опционально, но удобно)
- Запущенный gRPC сервис SSO.

### Конфигурация
Настройки загружаются из YAML файла. Путь к файлу передается через флаг `--config` или переменную окружения `CONFIG_PATH`.

Пример конфигурации (`config/local.yaml`):
```yaml
env: 'local'
auth:
  address: "localhost:44044" # Адрес gRPC SSO сервиса
  timeout: "5s"
http:
  enable_cors: true
  api_key: "dev-secret-key"  # Ключ для защиты Gateway
ws:
  port: 8888
  ws-max-message-size: 1048576
  ws-send-buffer-size: 256
  log-data-stream: true
```

### Локальный запуск

С помощью Task:
```bash
task start:dev
```

Или стандартной командой Go:
```bash
go run ./cmd/syncer --config=./config/local.yaml
```

### Docker
```bash
docker build -t kaban-syncer .
docker run -p 8888:8888 -e CONFIG_PATH=./config/prod.yaml kaban-syncer
```

## API Эндпоинты

### 1. Подключение к WebSocket
**GET** `/ws?workspaceId={id}&userId={id}&token={jwt}`
- Производит Upgrade соединения до WebSocket.
- Параметр `token` синхронно проверяется в SSO-сервисе через gRPC.
- Клиенты изолируются в комнатах по `workspaceId`.

### 2. Публикация события (Gateway)
**POST** `/event`
- **Заголовок:** `x-api-key: {api_key}`
- **Тело (JSON):**
```json
{
  "workspaceId": "ws-123",
  "userId": "user-456",
  "type": "task_updated",
  "payload": { ... },
  "rev": 1
}
```
*Примечание: Событие рассылается всем клиентам в комнате, за исключением пользователя (`userId`), который инициировал событие.*

### 3. Метрики
**GET** `/metrics`
- Отдает метрики для сбора Prometheus.

## Тестирование

Код покрыт юнит-тестами с использованием `httptest` и асинхронных проверок для WebSocket Hub и Rooms.

Запуск всех тестов:
```bash
go test ./... -v
```
