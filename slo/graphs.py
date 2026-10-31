#!/usr/bin/env python3
"""Print ready-to-open VMUI graph URLs, or one named query for copying."""
import argparse
import json
from pathlib import Path
from urllib.parse import urlencode

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base-url', default='http://127.0.0.1:8428')
parser.add_argument('--query', help='Print one expression: business_sli, probe_sla, compliance, raw_metrics, demo_mode, probe_coverage')
parser.add_argument('--markdown', action='store_true', help='Produce a page of clickable Markdown links')
parser.add_argument('--output', help='Write the generated links to this file')
args = parser.parse_args()
queries = json.loads(Path(__file__).with_name('queries.json').read_text())
if args.query:
    if args.query not in queries:
        parser.error('Unknown query name')
    print(queries[args.query]['query'])
else:
    lines = ['# Графики SLI, SLO и SLA\n', 'Откройте port-forward на 8428, затем нужный график. Все данные отображаются в vmui.\n', 'Окно расчёта — 2 минуты; это демонстрация, не оценка фактического SLA за 30 дней.\n'] if args.markdown else []
    for name, item in queries.items():
        params = urlencode({'g0.expr': item['query'], 'g0.tab': '0', 'g0.range_input': '15m', 'g0.relative_time': 'last_15_minutes', 'g0.step_input': '10s'})
        url = f"{args.base_url.rstrip('/')}/vmui/#/?{params}"
        lines.append(f"- [{item['title']}]({url})\n" if args.markdown else f"{name}: {item['title']}\n{url}\n")
    result = '\n'.join(lines).rstrip() + '\n'
    if args.output:
        Path(args.output).write_text(result)
    else:
        print(result)
