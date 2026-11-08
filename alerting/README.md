# Алерты TimePilot

Три правила: два бизнес-алерта и один на **настоящую метрику Go runtime** `go_goroutines`, которую уже экспортирует `client_golang`. Spring/Python не нужны: приложение написано на Go.

Схема: TimePilot и пробер → VictoriaMetrics → vmalert → Alertmanager → локальный webhook-приёмник с веб-страницей уведомлений.

## Запуск

Из корня репозитория, при запущенном Docker/OrbStack:

```sh
cd /Users/antonbarabulya/GolandProjects/timepilot
minikube status
# Если кластер остановлен: minikube start
./scripts/minikube-build.sh
./alerting/deploy.sh
```

Скрипт предполагает, что приложение, PostgreSQL и VictoriaMetrics из прошлых заданий уже установлены. Он не пересоздаёт БД и не меняет сохранённые данные. Если установка с нуля — сначала выполните инструкции `monitoring/README.md`.

Откройте четыре вкладки терминала, в каждой оставьте работающую команду:

```sh
kubectl port-forward -n monitoring svc/vmalert 8880:8880
```

```sh
kubectl port-forward -n monitoring svc/alertmanager 9093:9093
```

```sh
kubectl port-forward -n monitoring svc/alert-inbox 9095:9095
```

```sh
kubectl port-forward -n timepilot deployment/timepilot 9090:9090
```

Страницы:

- [Все правила](http://127.0.0.1:8880/vmalert/groups).
- [Активные алерты vmalert](http://127.0.0.1:8880/vmalert/alerts).
- [Alertmanager](http://127.0.0.1:9093/#/alerts).
- [Полученные уведомления](http://127.0.0.1:9095/).

Если порт занят старым `kubectl port-forward`, завершите именно его через Ctrl+C. После перезапуска pod проброс 9090 нужно запустить заново.

## Быстрая проверка и скриншоты

Не требуется записывать скринкаст: в этом задании нужны скриншоты.

1. После запуска дайте проберу около двух минут на сбор данных.
2. Откройте страницу правил. Разверните группу `timepilot`, чтобы были видны имена, выражения и `for: 30s`. Скриншот: `screenshots/01-rules.png`.
3. В отдельной вкладке терминала включите сбои:

   ```sh
   ./alerting/demo.sh business
   ./alerting/demo.sh runtime
   ```

4. Дождитесь перехода `pending → firing`: обычно 1–2 минуты, с учётом scrape 10s, задержки оценки 30s и `for: 30s`. Если история пустая, может понадобиться до трёх минут. На странице активных алертов должны появиться **все три** имени. Скриншот: `screenshots/02-firing.png`.
5. Откройте http://127.0.0.1:9095/. Через ещё 5–15 секунд должны прийти отдельные уведомления по каждому имени. На скриншоте должны быть видны `firing`, два business-алерта и **TimePilotGoGoroutinesHigh**. При необходимости сделайте несколько кадров. Имена: `03-notifications-firing.png`, `03b-runtime-notification.png`.
6. Верните нормальную работу:

   ```sh
   ./alerting/demo.sh healthy
   ```

7. Через 2–3 минуты, после выхода ошибок из окна 2m, на странице уведомлений появятся `resolved`. Это дополнительный скриншот `04-notifications-resolved.png`.

На Mac: **Shift+Cmd+4**, выделить область с URL, именами и содержимым. Сохраните изображения в `alerting/screenshots/` и проверьте ссылки в `alerting/Отчёт.md`. Здесь также лежат скриншоты автоматической проверки, если она уже была выполнена.

Проверка через API:

```sh
./alerting/check.sh
kubectl logs -n monitoring deployment/alert-inbox --tail=20
```

`GET /api/notifications` показывает именно принятые HTTP POST от Alertmanager, а не список вычисленных правил. Приёмник хранит последние 100 сообщений в памяти, отображает время в UTC, перезапуск очищает историю. Доступ только через внутренний ClusterIP/локальный port-forward. Для учебной задачи не нужны Telegram, почта и секреты.

## После проверки

```sh
./alerting/demo.sh healthy
# Отключить управляющие эндпоинты полностью:
kubectl set env deployment/timepilot -n timepilot SLO_DEMO_ENABLED-
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s
```

При необходимости остановить весь учебный кластер без удаления данных:

```sh
minikube stop
```

## Сдача

Архив с отчётом, конфигурациями и скриншотами:

```sh
./alerting/package-submission.sh
```

Скрипт не включает `.env`, Git-историю и видео прошлых заданий. Архив `timepilot-alerting.zip` появится в корне проекта. Пуш автоматически не выполняется.
