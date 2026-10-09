# TimePilot

TimePilot — компактный веб-планировщик времени. Пользователь ведёт список задач, меняет их статусы и при желании подключает AI-провайдера, который составляет реалистичный план на доступное время. Backend предоставляет REST API и хранит задачи в PostgreSQL.

## Быстрый запуск

Нужен Docker с плагином Compose:

```bash
cp .env.example .env
docker compose up --build
```

Откройте <http://localhost:8080>. Compose поднимет PostgreSQL, выполнит миграцию отдельным процессом и запустит приложение.

Остановка:

```bash
docker compose down
```

Данные останутся в Docker volume. Для полного удаления локальных данных используется `docker compose down -v`.

## Подключение AI-планировщика

AI отключён по умолчанию, поэтому CRUD задач работает без внешних сервисов и токенов. Чтобы включить генерацию плана, отредактируйте локальный `.env`:

```dotenv
AI_PROVIDER=openai
AI_MODEL=<идентификатор доступной вам модели>
OPENAI_API_KEY=<ваш ключ>
```

Поддерживаемые варианты:

| `AI_PROVIDER` | Переменная ключа | API |
|---|---|---|
| `openai` | `OPENAI_API_KEY` | OpenAI Responses API |
| `anthropic` | `ANTHROPIC_API_KEY` | Anthropic Messages API |
| `gemini` | `GEMINI_API_KEY` | Gemini Generate Content API |

Вместо отдельной переменной провайдера можно использовать общую `AI_API_KEY`. `AI_MODEL` задаётся явно, поскольку доступные модели зависят от аккаунта и меняются со временем. `AI_BASE_URL` позволяет подключить совместимый gateway или тестовый сервер, а `AI_TIMEOUT` задаёт таймаут запроса.

После изменения конфигурации пересоздайте приложение:

```bash
docker compose up --build -d
```

Ключ передаётся только backend-контейнеру. Он не встраивается во frontend, не возвращается API и не записывается в логи. Файл `.env` исключён из Git. Для production следует использовать secret manager платформы.

При генерации цель и текущие задачи отправляются выбранному AI-провайдеру. Не помещайте в задачи секреты или персональные данные, если правила вашего провайдера не допускают такую обработку.

Официальная документация: [OpenAI Responses API](https://developers.openai.com/api/reference/cli/resources/responses/methods/create), [Anthropic Messages API](https://docs.anthropic.com/en/api/messages), [Gemini API](https://ai.google.dev/api/generate-content).

## REST API

| Метод | Путь | Назначение |
|---|---|---|
| `GET` | `/api/v1/tasks` | список задач |
| `POST` | `/api/v1/tasks` | создать задачу |
| `GET` | `/api/v1/tasks/{id}` | получить задачу |
| `PUT` | `/api/v1/tasks/{id}` | полностью обновить задачу |
| `DELETE` | `/api/v1/tasks/{id}` | удалить задачу |
| `GET` | `/api/v1/ai/status` | состояние AI без раскрытия ключа |
| `POST` | `/api/v1/ai/plan` | составить план по текущим задачам |
| `GET` | `/healthz` | liveness: HTTP-процесс работает |
| `GET` | `/readyz` | readiness: приложение видит БД |

Создание задачи:

```bash
curl -i http://localhost:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Подготовить презентацию","description":"Собрать основные тезисы","status":"todo"}'
```

Генерация плана:

```bash
curl -i http://localhost:8080/api/v1/ai/plan \
  -H 'Content-Type: application/json' \
  -d '{"goal":"Подготовиться к встрече без переработок","available_minutes":180}'
```

Допустимые статусы задач: `todo`, `in_progress`, `done`.

## Разработка без контейнера приложения

Запустите только БД, задайте строку подключения и выполните миграцию:

```bash
docker compose up -d db
export DATABASE_URL='postgres://timepilot:timepilot@localhost:5432/timepilot?sslmode=disable'
go run ./cmd/timepilot migrate
go run ./cmd/timepilot server
```

Тесты и сборка:

```bash
go test ./...
go build -o timepilot ./cmd/timepilot
```

## Структура

```text
cmd/timepilot/       точка входа и команды server/migrate/healthcheck
internal/config/     конфигурация из environment
internal/httpapi/    REST API и встроенный frontend
internal/planner/    общий AI-интерфейс и адаптеры провайдеров
internal/store/      работа с PostgreSQL
internal/task/       доменная модель и интерфейс хранилища
migrations/          версионируемая схема БД
compose.yaml         локальное окружение
Dockerfile           воспроизводимая сборка одного образа
Отчёт.md             архитектура и разбор 12 факторов
```

Backend использует стандартную маршрутизацию Go 1.22. Единственная внешняя Go-зависимость — PostgreSQL-драйвер `pgx`; запросы к AI выполняются стандартным HTTP-клиентом через единый интерфейс `planner.Planner`.

## Запуск в minikube

Docker, `kubectl` и minikube должны быть установлены. Кластер создаётся с Docker runtime, чтобы команда `minikube docker-env` направляла сборку во внутренний Docker daemon:

```bash
minikube start --driver=docker --container-runtime=docker --cpus=2 --memory=3072
./scripts/minikube-build.sh
./scripts/minikube-deploy.sh
```

Скрипт сначала выполняет требуемую сборку через `minikube docker-env`. Если внутренний Docker daemon не может скачать базовый образ из-за сетевых ограничений, он автоматически собирает образ host-демоном и загружает его официальной командой `minikube image load`.

Проверка и масштабирование:

```bash
make k8s-status
make k8s-scale
minikube service timepilot -n timepilot
```

Манифесты находятся в `k8s/`. Подробный план записи демонстрации приведён в `СКРИНКАСТ.md`.
