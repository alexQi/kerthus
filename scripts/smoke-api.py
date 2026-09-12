#!/usr/bin/env python3
"""Read-only RPC/HTTP smoke plus one create/delete draft tenant. Never prints credentials."""
import json,os,pathlib,urllib.request,uuid
root=pathlib.Path(__file__).resolve().parents[1]
for line in (root/'.local/saas.env').read_text().splitlines():
 if line and not line.startswith('#'):
  k,v=line.split('=',1);os.environ.setdefault(k,v)
password=os.environ.get('KERTHUS_LOGIN_PASSWORD') or os.environ.get('KERTHUS_ADMIN_PASSWORD')
if not password:raise SystemExit('Imported data keeps original account passwords. Set KERTHUS_LOGIN_PASSWORD to run password-based smoke tests.')
base='http://'+os.environ['KERTHUS_HTTP_ADDRESS'];headers={}
def call(path,data=None,expect=0):
 req=urllib.request.Request(base+path,headers=headers,data=None if data is None else json.dumps(data).encode())
 if data is not None:req.add_header('Content-Type','application/json')
 with urllib.request.urlopen(req,timeout=25) as r:body=json.load(r)
 assert body['code']==expect, (path,body['code'],body.get('msg'))
 return body['data']
call('/healthz');call('/app/info/init')
login=call('/system/user/login',{'username':os.environ['KERTHUS_ADMIN_PHONE'],'password':password,'scene':'phone'})
headers={'access-token':login['access_token'],'tenant-id':str(login['tenant_id']),'app-id':str(login['app_id']),'unit-id':str(login['unit_id']),'section-id':str(login['section_id'])}
call('/system/user/profile');auth=call('/system/user/auth');assert 'admin' in auth['roles'];assert auth.get('routes')
for path in ['/system/tenant/query','/system/app/query','/system/app/getGlobalResource','/system/org/query','/system/employee/query','/system/role/query','/app/info/servers','/system/log/query']:call(path)
name='Smoke draft '+uuid.uuid4().hex[:8]
created=call('/system/tenant/save',{'name':name,'contact_person':'Smoke','contact_phone':'139'+str(int(uuid.uuid4().hex[:10],16)%100000000).zfill(8),'address':[]})
if isinstance(created,dict):created=created.get('id')
assert created
call('/system/tenant/delete?id='+str(created))
call('/system/user/logout');call('/system/user/profile',expect=401)
print('PASS: real HTTP → go-micro RPC → MySQL/Redis, menu contract, admin queries, draft transaction, logout revocation')
