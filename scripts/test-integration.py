#!/usr/bin/env python3
"""Run isolated MySQL integration tests; credentials stay in process environment."""
import os,pathlib,subprocess
root=pathlib.Path(__file__).resolve().parents[1]
env=os.environ.copy()
if 'KERTHUS_TEST_MYSQL_DSN' not in env:
 for line in (root/'.local/saas.env').read_text().splitlines():
  if line and not line.startswith('#'):
   k,v=line.split('=',1);env.setdefault(k,v)
 env['KERTHUS_TEST_MYSQL_DSN']='root:'+env['KERTHUS_DB_ROOT_PASSWORD']+'@tcp(127.0.0.1:13306)/?charset=utf8mb4&parseTime=true&loc=UTC&clientFoundRows=true'
raise SystemExit(subprocess.run(['go','test','-race','-count=1','./internal/saas/...'],cwd=root,env=env).returncode)
