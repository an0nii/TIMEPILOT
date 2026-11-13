#!/usr/bin/env python3
"""Generate classic Grafana JSON and adapt the vendored catalog dashboard.

Only real collected metrics are used. The original PostgreSQL dashboard is
retained in vendor/ so every compatibility adaptation is reproducible.
"""
import copy
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
DS = {"type": "prometheus", "uid": "victoriametrics"}


def target(expr, legend, ref="A", instant=False):
    return {"datasource": DS, "editorMode": "code", "expr": expr,
            "legendFormat": legend, "refId": ref, "range": not instant, "instant": instant}


def panel(title, targets, pos, unit="short", kind="timeseries", description="", thresholds=None):
    defaults = {"unit": unit, "color": {"mode": "palette-classic"}, "noValue": "Нет данных"}
    if unit == "percent":
        defaults.update({"min":0, "max":100})
    if thresholds:
        defaults["thresholds"] = {"mode": "absolute", "steps": thresholds}
        if kind == "stat":
            defaults["color"] = {"mode": "thresholds"}
    options = ({"reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
                "textMode": "auto", "colorMode": "value", "graphMode": "area",
                "justifyMode": "auto", "orientation": "auto"} if kind == "stat" else
               {"legend": {"displayMode": "list", "placement": "bottom", "calcs": []},
                "tooltip": {"mode": "multi", "sort": "none"}})
    return {"title": title, "type": kind, "datasource": DS, "gridPos": dict(zip(("x", "y", "w", "h"), pos)),
            "description": description, "targets": targets, "options": options,
            "fieldConfig": {"defaults": defaults, "overrides": []}}


def row(title, y):
    return {"type": "row", "title": title, "collapsed": False, "panels": [],
            "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}}


def save(name, dashboard):
    for i, p in enumerate(dashboard["panels"], 1):
        p["id"] = i
    (ROOT / "dashboards").mkdir(exist_ok=True)
    (ROOT / "dashboards" / name).write_text(json.dumps(dashboard, ensure_ascii=False, indent=2) + "\n")


def base(uid, title, description):
    return {"uid": uid, "id": None, "title": title, "description": description,
            "tags": ["timepilot", "sre"], "schemaVersion": 39, "version": 1, "editable": True,
            "timezone": "browser", "time": {"from": "now-30m", "to": "now"}, "refresh": "10s",
            "graphTooltip": 1, "templating": {"list": []}, "panels": []}


def application():
    d = base("timepilot-overview", "TimePilot · Обзор сервиса",
             "RED, Go runtime и request-based SLA синтетического CRUD-пробера. Пробелы мониторинга не считаются успешными проверками.")
    http = 'namespace="timepilot",job="timepilot-web",route=~"/api/.*"'
    all_rate = f'sum(rate(timepilot_http_requests_total{{{http}}}[$__rate_interval]))'
    bad_rate = f'(sum(rate(timepilot_http_requests_total{{{http},status=~"5.."}}[$__rate_interval])) or 0 * {all_rate})'
    errors = f'(100 * {bad_rate} / {all_rate}) and on() ({all_rate} > 0)'
    p95 = f'histogram_quantile(0.95, sum by(le)(rate(timepilot_http_request_duration_seconds_bucket{{{http}}}[$__rate_interval])))'
    probe = lambda w: f'100 * sum(increase(timepilot_probe_requests_total{{namespace="timepilot",result="success"}}[{w}])) / sum(increase(timepilot_probe_requests_total{{namespace="timepilot"}}[{w}]))'
    good = [{"color": "red", "value": None}, {"color": "green", "value": 1}]
    error_colors = [{"color": "green", "value": None}, {"color": "orange", "value": 1}, {"color": "red", "value": 5}]
    sla_colors = [{"color": "red", "value": None}, {"color": "green", "value": 99}]
    panels = d["panels"]
    panels.extend([
        panel("Сбор метрик API · UP", [target('min(up{namespace="timepilot",job="timepilot-web"})', "API", instant=True)], (0,0,4,4), kind="stat", thresholds=good,
              description="Scrape-доступность всех обнаруженных web-реплик. Не равна доступности пользовательского сценария; её проверяет пробер."),
        panel("API · RPS", [target(all_rate, "RPS", instant=True)], (4,0,5,4), "reqps", "stat", "Только /api/; Kubernetes health checks и frontend исключены. Пробер входит в трафик."),
        panel("API · ошибки 5xx", [target(errors, "5xx", instant=True)], (9,0,5,4), "percent", "stat", "100 × 5xx / все API-запросы. При отсутствии трафика результат не определён, не подменяется 0%.", error_colors),
        panel("API · p95 задержки", [target(p95, "p95", instant=True)], (14,0,5,4), "s", "stat", "Оценка p95 по histogram buckets, не точный percentile. Включает время БД и ответы с ошибками."),
        panel("PostgreSQL · UP", [target('min(pg_up{namespace="timepilot",job="timepilot-database"})', "Postgres", instant=True)], (19,0,5,4), kind="stat", thresholds=good),
        row("SLA пробера · цель ≥99% за 30 дней · полнота истории обязательна", 4),
        panel("SLA · P(30d) по собранным проверкам", [target(probe("30d"), "P(30d)", instant=True)], (0,5,8,5), "percent", "stat",
              "SLA-метрика = 100 × успешные CRUD-проверки / все проверки за 30d; цель 99%. Число рассчитано только по имеющимся наблюдениям. При неполной истории это НЕ подтверждение выполнения месячного SLA. См. покрытие справа.", sla_colors),
        panel("Покрытие мониторинга · 30d, оценка", [target('100 * sum(count_over_time(up{namespace="timepilot",job="timepilot-prober"}[30d])) / 259200', "Покрытие", instant=True)], (8,5,8,5), "percent", "stat",
              "Число сохранённых scrape-попыток пробера / (30d ÷ 10s). При одной Recreate-реплике это приблизительная оценка полноты истории, не доступность. <100% означает, что периоды без наблюдений не позволяют подтвердить SLA за весь месяц.", [{"color":"orange","value":None},{"color":"green","value":99.9}]),
        panel("Пробер · доступность за 2m (демо)", [target(probe("2m"), "P(2m)", instant=True)], (16,5,8,5), "percent", "stat", "Короткое демонстрационное окно. Не заменяет SLA за 30 дней.", sla_colors),
        panel("Пробер · успешность и цели на окне 2m", [target(probe("2m"), "Доступность пробера"), target('vector(99)', "Порог SLA 99%", "B"), target('vector(99.9)', "Цель SLO 99.9%", "C")], (0,10,12,7), "percent", description="Для быстрой демонстрации поведения. Цели месячные, график 2m сам по себе не является месячным вердиктом SLA."),
        panel("Пробер · результаты операций в секунду", [target('sum by(operation,result)(rate(timepilot_probe_requests_total{namespace="timepilot"}[$__rate_interval]))', '{{operation}} · {{result}}')], (12,10,12,7), "ops"),
        row("RED · Rate, Errors, Duration — HTTP API",17),
        panel("RPS по методам API", [target(f'sum by(method)(rate(timepilot_http_requests_total{{{http}}}[$__rate_interval]))', '{{method}}')], (0,18,8,7), "reqps"),
        panel("HTTP-статусы API · RPS", [target(f'sum by(status)(rate(timepilot_http_requests_total{{{http}}}[$__rate_interval]))', '{{status}}')], (8,18,8,7), "reqps"),
        panel("Задержка API · p50 / p95 / p99", [target(p95.replace('0.95,', f'{q},'), legend, ref) for q,legend,ref in [(0.5,"p50","A"),(0.95,"p95","B"),(0.99,"p99","C")]], (16,18,8,7), "s"),
        panel("Успешные бизнес-операции в секунду", [target(f'sum(rate({metric}{{namespace="timepilot"}}[$__rate_interval]))', legend, ref) for metric,legend,ref in [('timepilot_tasks_created_total','Создание','A'),('timepilot_tasks_viewed_total','Чтение','B'),('timepilot_tasks_updated_total','Обновление','C'),('timepilot_tasks_deleted_total','Удаление','D')]], (0,25,12,7), "ops"),
        panel("Три бизнес-SLI · окно 2m", [target(f'100 * sum(increase(timepilot_tasks_{suffix}_total{{namespace="timepilot",sli_version="1"}}[2m])) / sum(increase(timepilot_task_attempts_total{{namespace="timepilot",sli_version="1",operation="{op}"}}[2m]))', op, ref) for suffix,op,ref in [('created','create','A'),('viewed','view','B'),('updated','update','C')]], (12,25,12,7), "percent"),
        row("Go runtime · ресурсы процесса и признаки насыщения",32),
        panel("Go · горутины по pod", [target('go_goroutines{namespace="timepilot",job="timepilot-web"}', '{{pod}}')], (0,33,8,7), description="Горутины — сигнал конкуренции/возможных зависаний, не прямой показатель использования CPU. Порог алерта прошлого ДЗ: >100 в течение 30s."),
        panel("Go · heap и RSS", [target('sum(go_memstats_heap_alloc_bytes{namespace="timepilot",job="timepilot-web"})', 'Go heap'), target('sum(process_resident_memory_bytes{namespace="timepilot",job="timepilot-web"})', 'RSS процесса', 'B')], (8,33,8,7), "bytes", description="Абсолютные байты, не проценты памяти ноды. RSS относится к процессу, а не только Go heap."),
        panel("CPU web-процессов · ядра", [target('sum(rate(process_cpu_seconds_total{namespace="timepilot",job="timepilot-web"}[$__rate_interval]))', 'Использование CPU')], (16,33,8,7), "cores", description="CPU-секунды в секунду. Не проценты лимита Kubernetes и не CPU всей ноды."),
        panel("HTTP · запросы в обработке", [target('timepilot_http_in_flight_requests{namespace="timepilot",job="timepilot-web"}', '{{pod}}')], (0,40,8,7), description="Gauge конкурентных запросов. При малом трафике scrape 10s может видеть 0. Это ожидаемо, а не No data."),
        panel("Go · GC паузы · max выборки runtime", [target('go_gc_duration_seconds{namespace="timepilot",job="timepilot-web",quantile="1"}', '{{pod}}')], (8,40,8,7), "s", description="Готовый summary Go collector, quantile=1: максимум сохранённой runtime-выборки GC-пауз. Это не глобальный максимум за выбранное окно Grafana и не histogram_quantile."),
        panel("Цепочка мониторинга · scrape UP", [target('up{namespace="timepilot",job=~"timepilot-web|timepilot-prober|timepilot-database"}', '{{job}} · {{pod}}')], (16,40,8,7), description="Scrape UP=1 не доказывает полноту месячной истории. Нет ряда/пробел означает отсутствие наблюдений, а не успех."),
    ])
    d["links"] = [{"title":"PostgreSQL Exporter", "type":"link", "url":"/d/timepilot-postgres", "targetBlank":False}]
    save("timepilot.json", d)


def postgres():
    original = json.loads((ROOT / "vendor/postgres-9628-rev8.json").read_text())
    d = base("timepilot-postgres", "PostgreSQL Exporter · импорт #9628",
             "Импорт Grafana.com #9628 revision 8, адаптирован для текущих labels, PostgreSQL 16 и Grafana. Original: https://grafana.com/grafana/dashboards/9628-postgresql-database/")
    d["gnetId"] = 9628
    d["templating"]["list"] = [
        {"name":"namespace", "label":"Namespace", "type":"query", "datasource":DS, "query":"label_values(pg_up{job=\"timepilot-database\"}, namespace)", "refresh":1, "current":{"text":"timepilot","value":"timepilot"}, "options":[]},
        {"name":"instance", "label":"Instance", "type":"query", "datasource":DS, "query":"label_values(pg_up{namespace=\"$namespace\",job=\"timepilot-database\"}, instance)", "refresh":1, "options":[]},
        {"name":"datname", "label":"Database", "type":"query", "datasource":DS, "query":"label_values(pg_stat_database_xact_commit{namespace=\"$namespace\",instance=\"$instance\"}, datname)", "refresh":1, "multi":True, "includeAll":True, "allValue":".*", "current":{"text":"timepilot","value":["timepilot"]}, "options":[]},
        {"name":"mode", "label":"Lock mode", "type":"custom", "query":"accessexclusivelock,accesssharelock,exclusivelock,rowexclusivelock,rowsharelock,sharelock,sharerowexclusivelock,shareupdateexclusivelock,sireadlock", "multi":True, "includeAll":True, "allValue":".*", "current":{"text":"All","value":"$__all"}, "options":[]},
    ]
    # Retain the catalog's main operational panels. Extra settings/version
    # panels are omitted to keep the imported view compact.
    keep = [38,10,11,12,22,24,26,1,60,8,5,6,3,14,4,7,62,64,66,68,70]
    by_id = {p["id"]:p for p in original["panels"]}
    d["panels"].append(panel("PostgreSQL · UP", [target('pg_up{namespace="$namespace",job="timepilot-database",instance="$instance"}', 'PG', instant=True)], (0,0,6,4), kind="stat"))
    for n, old_id in enumerate(keep):
        old = copy.deepcopy(by_id[old_id])
        kind = "stat" if old["type"]=="singlestat" else "timeseries"
        if n<3:
            pos = ((n+1)*6,0,6,4)
        else:
            pos = (((n-3)%3)*8,4+((n-3)//3)*7,8,7)
        targets = []
        for i, t in enumerate(old.get("targets",[])):
            expr = t["expr"].replace('release="$release", ', 'namespace="$namespace", job="timepilot-database", ')
            expr = expr.replace('instance=', 'namespace="$namespace", job="timepilot-database", instance=') if 'namespace=' not in expr else expr
            # Gauge levels must not be differentiated; zero sessions/locks are
            # legitimate values and must remain visible.
            expr = expr.replace(' !=0','').replace(' != 0','')
            if old_id==22: expr=expr.replace(' * 1000','')
            if old_id==24:
                expr = f'avg({"process_resident_memory_bytes" if i==0 else "process_virtual_memory_bytes"}{{namespace="$namespace",job="timepilot-database",instance="$instance"}})'
            if old_id in (1,4):
                expr = 'sum by(datname,state)(' + expr + ')'
            targets.append(target(expr, t.get("legendFormat") or old["title"], chr(65+i), kind=="stat"))
        unit = old.get("format", old.get("yaxes", [{}])[0].get("format", "short"))
        if old_id==22: unit="cores"
        if old_id in (10,11,12,8,5,6,14,7): unit="short"
        if old_id in (60,64,66): unit="ops"
        if old_id==68: unit="Bps"
        if old_id==70: unit="suffix: ms/s"
        title = {22:"Exporter · CPU, не PostgreSQL",24:"Exporter · память, не PostgreSQL",26:"Exporter · открытые FD"}.get(old_id,old["title"])
        d["panels"].append(panel(title, targets, pos, unit, kind,
                                   f'Импортированная панель #{old_id} каталога Grafana #9628 rev8. Метки адаптированы к Pod service discovery TimePilot.'))
    d["links"]=[{"title":"TimePilot", "type":"link", "url":"/d/timepilot-overview", "targetBlank":False}]
    save("postgres-exporter.json", d)


if __name__ == "__main__":
    application()
    postgres()
