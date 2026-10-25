# Метрики TimePilot в minikube

Здесь находится отдельное решение задания по VictoriaMetrics. Подсистема логов
в `observability/` остаётся независимой.

```text
TimePilot Pod :9090/metrics ────────┐
                                  ├─ pull каждые 10 секунд → VictoriaMetrics → PVC
Postgres Pod :9187/metrics ─────────┘                         :8428
       postgres-exporter sidecar                  Kubernetes Pod discovery
```

VictoriaMetrics single установлена официальным Helm-чартом. Встроенный scraper
обнаруживает Pod по `prometheus.io/*` аннотациям и собирает метрики напрямую.
Отдельный vmagent для этого не нужен: `/targets` и `/service-discovery`
находятся у самой VictoriaMetrics на порту 8428.

## Запуск

Понадобятся Docker (OrbStack или Docker Desktop), minikube, kubectl, Helm и jq.
На Mac отсутствующие утилиты можно установить через `brew install helm jq`.

Из корня проекта:

```bash
minikube start
./scripts/minikube-build.sh
./scripts/minikube-deploy.sh
./monitoring/deploy.sh
```

Версии: Helm-чарт `victoria-metrics-single` `0.29.0`, VictoriaMetrics `v1.134.0`,
postgres-exporter `v0.18.1`, Go-библиотека `client_golang` `v1.20.5` (совместима с Go 1.22).

`monitoring/deploy.sh` применяет обновлённые манифесты приложения и PostgreSQL,
перезапускает приложение после пересборки локального образа и устанавливает Helm release
`vm-single` в Namespace `monitoring`. Имя Service VictoriaMetrics фиксировано: `victoria-metrics`.

## Проверка

Terminal 1 — приложение:

```bash
kubectl port-forward -n timepilot service/timepilot 8080:8080
```

Terminal 2 — VictoriaMetrics:

```bash
kubectl port-forward -n monitoring service/victoria-metrics 8428:8428
```

Terminal 3 — тестовые CRUD-операции и проверка сохранённых метрик:

```bash
./monitoring/demo.sh
sleep 45
./monitoring/check.sh
```

Скрипт demo создаёт одну задачу, читает, обновляет и удаляет именно её. Другие задачи
не затрагиваются. Скрипт check проверяет `up`, Go runtime, состояние PostgreSQL,
транзакции PostgreSQL и все четыре бизнес-метрики через API VictoriaMetrics.

В браузере:

- <http://127.0.0.1:8428/targets> — оба обнаруженных таргета и результат scraping;
- <http://127.0.0.1:8428/service-discovery> — результаты service discovery;
- <http://127.0.0.1:8428/vmui/> — запросы к метрикам.

Пример запросов к API:

```bash
curl -fsS http://127.0.0.1:8428/api/v1/label/__name__/values | jq
curl -fsSG http://127.0.0.1:8428/api/v1/query \
  --data-urlencode 'query=up' | jq
curl -fsSG http://127.0.0.1:8428/api/v1/query \
  --data-urlencode 'query=pg_up' | jq
curl -fsSG http://127.0.0.1:8428/api/v1/query \
  --data-urlencode 'query={__name__=~"timepilot_tasks_.*_total"}' | jq
```

Прямой просмотр OpenMetrics приложения в отдельном Terminal:

```bash
kubectl port-forward -n timepilot deployment/timepilot 9090:9090
```

```bash
curl -i -H 'Accept: application/openmetrics-text; version=1.0.0' \
  http://127.0.0.1:9090/metrics
```

Ответ содержит `Content-Type: application/openmetrics-text`, runtime-метрики,
четыре бизнес-счётчика и завершающий `# EOF`. Без заголовка Accept библиотека
также поддерживает обычный Prometheus text format.

## Доступ к метрикам

Порт приложения 9090 и порт экспортера 9187 не включены в NodePort/Ingress.
Они доступны внутри сети кластера; локальный port-forward — административный доступ.
Вне Kubernetes по умолчанию метрики слушают только `127.0.0.1:9090`;
адрес меняется через `METRICS_ADDR`. Основной HTTP-порт 8080 не отдаёт `/metrics`.

Это не изоляция от других Pod: стандартная сеть minikube допускает внутренние
запросы. NetworkPolicy и отдельная учётная запись БД для exporter здесь не добавлены:
для учебной установки exporter использует существующий Secret приложения.

## Артефакты

Отдельный отчёт: [Отчёт.md](Отчёт.md). Сценарий записи: [СКРИНКАСТ.md](СКРИНКАСТ.md).
После записи на Mac:

```bash
./monitoring/package-submission.sh /absolute/path/to/screencast.mov
unzip -l timepilot-metrics.zip
```

В архив попадут исходники, все манифесты, Helm values, отдельный `Отчёт.md` в корне
и видео. Если видео ещё нет, скрипт остановится с подсказкой.

## Диагностика

```bash
kubectl get pods,svc,pvc -n monitoring
kubectl logs -n monitoring statefulset/victoria-metrics --tail=50
kubectl logs -n timepilot postgres-0 -c postgres-exporter --tail=50
kubectl get pods -n timepilot -o wide
helm list -n monitoring
```

Если `up=0`, проверьте `/targets` — там будет последняя ошибка обращения.
Если `up=1`, но `pg_up=0`, exporter доступен, однако не может подключиться к БД.
Scraping выполняется каждые 10 секунд, но VictoriaMetrics по умолчанию скрывает
самые свежие точки в запросах примерно на 30 секунд (`search.latencyOffset`). После
CRUD подождите около 45 секунд. При первом запуске добавляется время discovery;
`check.sh` автоматически ждёт появления всех групп метрик до 90 секунд.

Если minikube не может скачать образ из-за registry-прокси (`ImagePullBackOff`,
`broken pipe`), загрузите его через Docker Mac и импортируйте в кластер:

```bash
docker pull prometheuscommunity/postgres-exporter:v0.18.1
minikube image load prometheuscommunity/postgres-exporter:v0.18.1
docker pull victoriametrics/victoria-metrics:v1.134.0
minikube image load victoriametrics/victoria-metrics:v1.134.0
```

Kubelet повторит запуск с локальным образом автоматически. Затем повторите
`./monitoring/deploy.sh`, если предыдущий запуск завершился по тайм-ауту.

Счётчики сбрасываются при перезапуске процесса. Для количества событий за окно
используйте `sum(increase(timepilot_tasks_created_total[5m]))`, для скорости —
`sum(rate(timepilot_tasks_created_total[5m]))`. Для нескольких реплик суммируйте их
счётчики, сохраняя в хранилище метки `pod` и `instance`.
