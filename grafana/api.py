#!/usr/bin/env python3
"""Import/export dashboards and verify each panel against real VM samples.

Credentials are read from the Helm Secret, never printed or committed.
Uses the Python standard library; no pip dependencies.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent
URL = os.environ.get("GRAFANA_URL", "http://127.0.0.1:3000").rstrip("/")
VM = os.environ.get("VM_URL", "http://127.0.0.1:8428").rstrip("/")


def request(path, body=None):
    headers = {}
    if body is not None:
        secret = json.loads(subprocess.check_output(["kubectl", "get", "secret", "grafana", "-n", "monitoring", "-o", "json"]))["data"]
        credentials = base64.b64decode(secret["admin-user"]) + b":" + base64.b64decode(secret["admin-password"])
        headers["Authorization"] = "Basic " + base64.b64encode(credentials).decode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(URL + path, data=json.dumps(body).encode() if body is not None else None, headers=headers)
    with urllib.request.urlopen(req, timeout=60) as response:
        return json.load(response)


def export():
    (ROOT / "exports").mkdir(exist_ok=True)
    for uid, filename in [("timepilot-overview", "timepilot.json"), ("timepilot-postgres", "postgres-exporter.json")]:
        d = request("/api/dashboards/uid/" + uid)["dashboard"]
        d["id"] = None
        (ROOT / "exports" / filename).write_text(json.dumps(d, ensure_ascii=False, indent=2) + "\n")
        print("Exported actual Grafana dashboard:", filename)


def verify():
    health = request("/api/datasources/uid/victoriametrics/health")
    if health.get("status") != "OK":
        raise RuntimeError("Datasource health failed: " + str(health))
    print("Datasource VictoriaMetrics: OK")
    # Use live label values, not an ephemeral Pod IP stored in source.
    with urllib.request.urlopen(VM + '/api/v1/query?' + urllib.parse.urlencode({"query":'pg_up{namespace="timepilot",job="timepilot-database"}'}), timeout=30) as response:
        series = json.load(response)["data"]["result"]
    instance = series[0]["metric"]["instance"]
    failures = []
    for uid in ["timepilot-overview", "timepilot-postgres"]:
        d = request("/api/dashboards/uid/" + uid)["dashboard"]
        for p in d["panels"]:
            for t in p.get("targets", []):
                expr = t.get("expr", "")
                expr = expr.replace("$__rate_interval", "1m").replace("$namespace", "timepilot").replace("$instance", instance).replace("$datname", "timepilot").replace("$mode", ".*")
                with urllib.request.urlopen(VM + "/api/v1/query?" + urllib.parse.urlencode({"query":expr}), timeout=60) as response:
                    result = json.load(response)
                values = result.get("data", {}).get("result", [])
                if result.get("status") != "success" or not values or all(v["value"][1] in ["NaN", "+Inf", "-Inf"] for v in values):
                    failures.append((p["title"], expr))
                else:
                    print("OK:", p["title"], t.get("refId", ""), "=", values[0]["value"][1])
    if failures:
        for title, expr in failures:
            print("NO DATA:", title, expr, file=sys.stderr)
        raise RuntimeError(f"{len(failures)} panel queries have no finite data")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["import-postgres", "export", "check"])
    args = parser.parse_args()
    try:
        if args.command == "import-postgres":
            result = request("/api/dashboards/db", {"dashboard":json.loads((ROOT / "dashboards/postgres-exporter.json").read_text()), "overwrite":True, "message":"Import catalog #9628 rev8 for TimePilot"})
            if result.get("status") != "success":
                raise RuntimeError(str(result))
            print("Imported PostgreSQL catalog #9628:", result["url"])
        elif args.command == "export":
            export()
        else:
            verify()
    except (urllib.error.URLError, RuntimeError, IndexError, KeyError) as exc:
        sys.exit("Grafana check failed: " + str(exc))
