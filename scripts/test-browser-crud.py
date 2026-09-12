#!/usr/bin/env python3
"""Run browser CRUD against an isolated MySQL database and private RPC registry."""
import os
import pathlib
import signal
import sys
import subprocess
import tempfile
import time

root = pathlib.Path(__file__).resolve().parents[1]
env = os.environ.copy()
env.setdefault('KERTHUS_ENV_FILE', str(root / '.local/saas.env'))
for line in pathlib.Path(env['KERTHUS_ENV_FILE']).read_text().splitlines():
    if line and not line.startswith('#'):
        key, value = line.split('=', 1)
        env.setdefault(key, value)
if 'KERTHUS_TEST_MYSQL_DSN' not in env:
    env['KERTHUS_TEST_MYSQL_DSN'] = ('root:' + env['KERTHUS_DB_ROOT_PASSWORD'] +
        '@tcp(127.0.0.1:13306)/?charset=utf8mb4&parseTime=true&clientFoundRows=true')

with tempfile.TemporaryDirectory(prefix='browser-acceptance-', dir=root / '.local') as tmp:
    executable = str(pathlib.Path(tmp) / 'server')
    env['KERTHUS_ACCEPTANCE_CONFIG'] = str(pathlib.Path(tmp) / 'connection.json')
    subprocess.run(['go', 'build', '-o', executable, './scripts/acceptance-server'], cwd=root, check=True)
    server = subprocess.Popen([executable], cwd=root, env=env)
    try:
        deadline = time.monotonic() + 40
        while not pathlib.Path(env['KERTHUS_ACCEPTANCE_CONFIG']).exists():
            if server.poll() is not None or time.monotonic() > deadline:
                raise RuntimeError('Acceptance server failed to start')
            time.sleep(0.2)
        result = subprocess.run(['node', sys.argv[1] if len(sys.argv) > 1 else 'scripts/smoke-crud.cjs'], cwd=root, env=env)
    finally:
        if server.poll() is None:
            server.send_signal(signal.SIGTERM)
        server.wait(timeout=20)
raise SystemExit(result.returncode)
