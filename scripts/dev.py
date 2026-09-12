#!/usr/bin/env python3
"""Supervise development processes; Ctrl-C stops only these child processes."""
import os,pathlib,signal,subprocess,time
root=pathlib.Path(__file__).resolve().parents[1];env=os.environ.copy();env.setdefault('KERTHUS_ENV_FILE',str(root/'.local/saas.env'));children=[]
try:
 for command,cwd in [(['./bin/saas'],root),(['./bin/gateway'],root),(['yarn','dev'],root/'web/admin')]:
  children.append(subprocess.Popen(command,cwd=cwd,env=env,start_new_session=True))
 while all(p.poll() is None for p in children):time.sleep(.5)
 if any(p.returncode for p in children if p.returncode is not None):raise SystemExit('A development process exited; check the output above.')
except KeyboardInterrupt:pass
finally:
 for p in children:
  if p.poll() is None:os.killpg(p.pid,signal.SIGTERM)
 for p in children:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait()
