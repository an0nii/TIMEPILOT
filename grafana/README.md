# Grafana · TimePilot

Готовы два дашборда: TimePilot (RED, Go runtime, бизнес-метрики, SLA пробера) и импортированный PostgreSQL Exporter #9628. Скринкаст записывается отдельно.

## Открыть уже установленную Grafana

```sh
cd /Users/antonbarabulya/GolandProjects/timepilot
orb start                 # Если используется OrbStack и он выключен
minikube start           # Если кластер остановлен
kubectl port-forward -n monitoring svc/grafana 3000:80
```

Не закрывайте терминал с port-forward. Откройте:

- [TimePilot](http://127.0.0.1:3000/d/timepilot-overview?from=now-15m&to=now&refresh=10s).
- [PostgreSQL Exporter](http://127.0.0.1:3000/d/timepilot-postgres?from=now-15m&to=now&refresh=10s).

Для просмотра на весь экран добавьте `&kiosk` к адресу. В левом верхнем углу дашбордов есть ссылка переключения между ними.

Включён анонимный доступ **Viewer** для локальной демонстрации. Service — ClusterIP, публикации наружу нет. Это не production-настройка доступа. Для административного входа пользователь `admin`; пароль генерирует Helm и хранит в Secret:

```sh
kubectl get secret grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 --decode; echo
```

Пароль не добавляйте в Git или отчёт. UI-редактирование provisioned TimePilot отключено; измените `build_dashboards.py`, пересоберите JSON и обновите ConfigMap. Импортированный PostgreSQL-дашборд хранится в БД Grafana и переживает перезапуск благодаря PVC.

## Воспроизвести установку

Нужны Docker/OrbStack, minikube, kubectl, Helm и Python 3 (только стандартная библиотека). Предполагается сохранённое окружение предыдущих ДЗ: TimePilot, PostgreSQL Exporter, пробер и VictoriaMetrics. При установке с нуля сначала выполните инструкции `monitoring/README.md`, затем `slo/README.md`.

```sh
./scripts/minikube-build.sh
kubectl rollout restart deployment/timepilot -n timepilot
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s
./grafana/deploy.sh
```

Helm-чарт закреплён: `grafana-community/grafana` версии `14.4.1`, Grafana `13.3.1`. Источник VictoriaMetrics и TimePilot provisioned автоматически. Полный источник дашборда #9628 rev8 включён в `vendor/`, повторное скачивание из каталога не требуется.

Запустите port-forward Grafana (выше), затем:

```sh
python3 grafana/api.py import-postgres
python3 grafana/api.py export
```

Импорт использует официальный HTTP API Grafana и адаптированный JSON каталога. `api.py` читает admin Secret через kubectl, не выводя пароль. Повторный импорт перезаписывает **только** дашборд с UID `timepilot-postgres`; учитывайте это, если редактировали его вручную.

## Проверить все панели

В ещё одном терминале:

```sh
kubectl port-forward -n monitoring svc/victoria-metrics 8428:8428
```

После накопления первых scrape (примерно 1–2 минуты):

```sh
python3 grafana/api.py check
```

Проверяются здоровье datasource, наличие дашбордов в Grafana и **каждый запрос панели** на реальные конечные значения в VictoriaMetrics. Проверка подставляет текущий `instance` экспортера, а не сохраняет IP pod в конфигурации. Нули (ошибки, deadlocks, временные файлы) — допустимые значения, не `No data`.

После изменения исходников дашборда:

```sh
python3 grafana/build_dashboards.py
kubectl create configmap timepilot-grafana-dashboard -n monitoring \
  --from-file=grafana/dashboards/timepilot.json --dry-run=client -o yaml | kubectl apply -f -
# Обновление тома ConfigMap + provider может занять 1–2 минуты.
python3 grafana/api.py import-postgres
python3 grafana/api.py export
```

`exports/timepilot.json` — требуемый артефакт: **экспорт реально установленного дашборда Grafana**, а не только генератор/шаблон. Его можно импортировать в другую Grafana с Prometheus datasource UID `victoriametrics`.

## Важное о SLA

`P(30d) = 100 × успешные проверки / все проверки за 30 дней`, цель SLA — `99%`. Красное число ниже порога показывает результат по **собранным** проверкам. Оно не является доказательством нарушения/выполнения месячного договора при неполной истории. Рядом отдельно показано приблизительное покрытие истории за 30 дней. Остановленный minikube не создаёт наблюдений задним числом.

Панели `P(2m)` и три бизнес-SLI за 2m нужны для наглядной демонстрации; они не заменяют месячную оценку. Пробер уже генерирует безопасный трафик, поэтому для обычного просмотра не требуется нагрузочный генератор.

## Запись скринкаста позже

1. Показать `kubectl get pods -n monitoring` и `kubectl get pods -n timepilot`.
2. Открыть TimePilot: верхние статусы/RPS/ошибки/latency.
3. Показать отдельную панель `SLA · P(30d)` и объяснить отличие от демо-окна 2m и покрытия.
4. Прокрутить к Go runtime: горутины, память, CPU, GC.
5. По ссылке открыть PostgreSQL Exporter: показать Instance/Database, транзакции, записи и cache hit ratio.
6. Показать наличие экспортированного `grafana/exports/timepilot.json`.

Для изменения графиков при записи можно воспользоваться безопасным отказом из прошлых ДЗ. Это **не обязательно** для текущего ТЗ. Включать только в учебном окружении:

```sh
kubectl set env deployment/timepilot -n timepilot SLO_DEMO_ENABLED=true
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s
```

В отдельном терминале после rollout:

```sh
kubectl port-forward -n timepilot deployment/timepilot 9090:9090
```

```sh
./slo/mode.sh degraded
# Через 30–60 секунд будут видны 503 и падение краткосрочной успешности.
./slo/mode.sh healthy
# После записи обязательно отключить управляющие маршруты:
kubectl set env deployment/timepilot -n timepilot SLO_DEMO_ENABLED-
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s
```

## Файлы для сдачи

- `exports/timepilot.json` — экспорт дашборда приложения.
- `exports/postgres-exporter.json` — дополнительный экспорт импортированного дашборда.
- `Отчёт.md` — описание выбранных метрик и скриншоты.
- `screenshots/` — настоящие скриншоты Grafana на Mac.
- `values.yaml`, `dashboards/`, `vendor/`, скрипты — воспроизведение окружения.
- Скринкаст пока **не записан**, его нужно добавить перед сдачей.

После записи можно собрать финальный архив:

```sh
./grafana/package-submission.sh /полный/путь/к/скринкасту.mov
```

Без видео скрипт не создаёт архив, выдаваемый за полное решение. Чтобы остановить все сервисы без удаления данных: `minikube stop`.
