"""Bounded acceptance against an EMPTY selector connected to mihomo-mock only.

Example: python tools/check-multi-monitor.py --url http://127.0.0.1:18878
  --groups 3 --candidates 6 --seconds 145 --output /tmp/monitor-result.json
Never point this at a production selector. The mock-version and empty-task
preconditions are checked before creating any monitoring configuration.
"""
import argparse
import json
import time
import urllib.request
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--url', required=True)
    parser.add_argument('--groups', type=int, default=3)
    parser.add_argument('--candidates', type=int, default=6)
    parser.add_argument('--seconds', type=int, default=145)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    if not 2 <= args.groups <= 10 or not 1 <= args.candidates <= 30 or not 125 <= args.seconds <= 600:
        parser.error('groups 2-10, candidates 1-30, seconds 125-600')
    base = args.url.rstrip('/') + '/api/v1'

    def call(path, data=None, method=None):
        payload = None if data is None else json.dumps(data).encode()
        request = urllib.request.Request(base + path, data=payload, method=method or ('POST' if data is not None else 'GET'), headers={'Content-Type': 'application/json'})
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)

    health = call('/health')
    if 'mock' not in health.get('mihomo_version', '').lower():
        raise SystemExit('Refusing to create tasks: connected Controller is not mihomo-mock')
    if call('/monitor/tasks'):
        raise SystemExit('Refusing to alter existing monitoring tasks; use an isolated empty database')
    groups = call('/groups')[:args.groups]
    if len(groups) != args.groups:
        raise SystemExit('Insufficient mock groups')
    plans = []
    for group in groups:
        from urllib.parse import urlencode
        catalog = call('/monitor/catalog?' + urlencode({'group': group['name'], 'profile_id': 'chatgpt'}))
        nodes = catalog['nodes'][:args.candidates]
        if len(nodes) != args.candidates:
            raise SystemExit('Insufficient mock candidates')
        plans.append(call('/monitor/tasks', {'revision': 0, 'enabled': True, 'auto_switch': False, 'group': group['name'], 'profile_id': 'chatgpt', 'candidate_limit': args.candidates, 'nodes': [n['name'] for n in nodes]}))
    started = time.time()
    print(json.dumps({'stage': 'sampling', 'groups': len(plans), 'candidates_per_group': args.candidates}), flush=True)
    checkpoints = []
    while time.time() - started < args.seconds:
        time.sleep(min(20, max(0, args.seconds - (time.time() - started))))
        snapshot = call('/monitor/scheduler')
        if snapshot['suspended'] or snapshot['running_workers'] > snapshot['workers'] or snapshot['requests_used'] > snapshot['max_requests_per_minute']:
            raise RuntimeError('Scheduler violated a capacity constraint')
        checkpoints.append({'elapsed': round(time.time() - started, 2), **snapshot})
        print(json.dumps({'stage': 'sampling', **checkpoints[-1]}), flush=True)
    observations = []
    all_series = set()
    for plan in plans:
        overview = call('/monitor/tasks/' + plan['task_id'] + '/overview?window=1h')
        if overview['issue'] or overview['suspended'] or len(overview['rows']) != args.candidates:
            raise RuntimeError('Unexpected task runtime state')
        for row in overview['rows']:
            if row['series_id'] in all_series:
                raise RuntimeError('Evidence series shared across tasks')
            all_series.add(row['series_id'])
            if row['metrics']['samples'] < 1 or row['metrics']['coverage'] < .9:
                raise RuntimeError('Insufficient baseline coverage on the responsive mock')
        observations.append({'task_id': plan['task_id'], 'group': plan['group'], 'rows': [{'node': r['name'], 'samples': r['metrics']['samples'], 'expected': r['metrics']['expected'], 'coverage': r['metrics']['coverage']} for r in overview['rows']]})
    # Keep the service running for an external restart/readback check. No live
    # selector is ever changed because every task explicitly disables failover.
    result = {'status': 'passed', 'controller': health.get('mihomo_version'), 'duration_seconds': round(time.time() - started, 2), 'groups': args.groups, 'candidates_per_group': args.candidates, 'scheduler': checkpoints, 'observations': observations, 'storage': call('/monitor/storage')}
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps({'status': result['status'], 'groups': args.groups, 'candidates': args.groups * args.candidates, 'output': str(output)}), flush=True)


if __name__ == '__main__':
    main()
