#!/usr/bin/env python3
"""Create isolated development credentials without overwriting existing state."""
from pathlib import Path
import secrets
import os
root=Path(__file__).resolve().parents[1]
p=root/'.local/saas.env'
if p.exists():
    print('Development configuration already exists: .local/saas.env')
    raise SystemExit(0)
p.parent.mkdir(exist_ok=True)
db=secrets.token_hex(20)
values={
 'KERTHUS_DB_PASSWORD':db,'KERTHUS_DB_ROOT_PASSWORD':secrets.token_hex(20),
 'KERTHUS_MYSQL_DSN':f'kerthus:{db}@tcp(127.0.0.1:13306)/kerthus?charset=utf8mb4&parseTime=true&loc=UTC&clientFoundRows=true',
 'KERTHUS_REDIS_ADDRESS':'127.0.0.1:16379','KERTHUS_REDIS_PASSWORD':secrets.token_hex(20),
 'KERTHUS_CONSUL_ADDRESS':'127.0.0.1:18500','KERTHUS_RPC_ADDRESS':'127.0.0.1:19090',
 'KERTHUS_RPC_GATEWAY_KEY':secrets.token_hex(32),'KERTHUS_HTTP_ADDRESS':'127.0.0.1:18080',
 'KERTHUS_STORAGE_ENDPOINT':'127.0.0.1:19000','KERTHUS_STORAGE_ACCESS_KEY':'kerthus-dev',
 'KERTHUS_STORAGE_SECRET_KEY':secrets.token_hex(24),'KERTHUS_STORAGE_BUCKET':'kerthus',
 'KERTHUS_ADMIN_PHONE':'13800000000','KERTHUS_ADMIN_PASSWORD':secrets.token_urlsafe(18),
 'KERTHUS_STATIC_URL':'http://127.0.0.1:18080/files',
 'KERTHUS_CORS_ORIGINS':'http://127.0.0.1:15173,http://localhost:15173',
}
fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
with os.fdopen(fd,'w') as f:
    f.write('\n'.join(k+'='+v for k,v in values.items())+'\n')
print('Created .local/saas.env (0600). Credentials were not printed.')
