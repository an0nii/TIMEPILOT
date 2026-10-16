# TimePilot: VictoriaLogs в minikube

Каталог содержит отдельный комплект артефактов для задания по централизованному сбору логов.

## Архитектура

```text
TimePilot Pod stdout (JSON)
        │
        ▼
/var/lib/docker/containers на minikube node
        │ Docker JSON envelope
        ▼
Vector DaemonSet ── Elasticsearch Bulk API ──► VictoriaLogs ──► PVC
                                                        │
                                                        ▼
                                      /select/vmui/ через port-forward
```

Remap transform Vector раскрывает служебную JSON-обёртку Docker и прикладной JSON,
после чего filter transform оставляет только события с `service.name=timepilot`.
Логи остальных компонентов кластера в VictoriaLogs не отправляются. Для Docker
runtime каталог `/var/lib/docker/containers` смонтирован в Pod Vector read-only.

## 1. Сборка и запуск приложения

```bash
cd /Users/antonbarabulya/GolandProjects/timepilot
minikube start --driver=docker --container-runtime=docker --cpus=2 --memory=4096
./scripts/minikube-build.sh
./scripts/minikube-deploy.sh
```

Проверьте JSON в stdout:

```bash
kubectl logs -n timepilot deployment/timepilot --tail=3
```

Каждая строка должна быть отдельным JSON-объектом с полями `timestamp`, `level`,
`message`, `service.name` и прикладными полями HTTP-запроса.

## 2. VictoriaLogs и Vector

```bash
./observability/deploy.sh
kubectl get pods,pvc,services -n observability
```

Ожидаются один Pod `victoria-logs`, один Pod `vector` на каждой ноде и Bound PVC.

Автоматическая проверка доставки:

```bash
./observability/check.sh
```

## 3. Создание демонстрационных логов

В первом Terminal оставьте port-forward приложения:

```bash
kubectl port-forward -n timepilot service/timepilot 8080:8080
```

Во втором Terminal создайте обычные запросы и один ответ 404:

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/api/v1/tasks
curl -i http://localhost:8080/api/v1/tasks/999999
```

Дополнительно выполните несколько действий в UI на <http://localhost:8080/>:
создайте, измените и удалите задачу. Это даст разные маршруты и HTTP-статусы для
`stats` и `top` запросов.

## 4. Открытие vmui

В отдельном Terminal:

```bash
kubectl port-forward -n observability service/victoria-logs 9428:9428
```

Откройте <http://localhost:9428/select/vmui/>. По очереди выполните запросы из
`observability/Отчёт.md`. На скриншоте должны одновременно быть видны запрос и результат.

## 5. Сборка архива

Поместите четыре PNG в `observability/screenshots/` под именами из файла
`observability/screenshots/README.md`, затем выполните:

```bash
./observability/package-submission.sh
unzip -l timepilot-victorialogs.zip
```

## Диагностика

```bash
kubectl logs -n observability deployment/victoria-logs --tail=100
kubectl logs -n observability daemonset/vector --tail=100
kubectl describe pod -n observability -l app.kubernetes.io/name=vector
```

Удалить только подсистему логирования:

```bash
kubectl delete namespace observability
```
