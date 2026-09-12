#!/usr/bin/env python3
"""Creates one public test pixel in local dev storage; never prints credentials."""
import os,sys,urllib.request,json,base64,uuid,pathlib
os.chdir(pathlib.Path(__file__).resolve().parents[1])
for line in open('.local/saas.env'):
 k,v=line.strip().split('=',1);os.environ[k]=v
password=os.environ.get('KERTHUS_LOGIN_PASSWORD') or os.environ.get('KERTHUS_ADMIN_PASSWORD')
if not password:raise SystemExit('Set KERTHUS_LOGIN_PASSWORD to test uploads with an original account; imported passwords are unchanged.')
base='http://127.0.0.1:18080'
def req(path,data=None,headers={}):
 r=urllib.request.Request(base+path,data=data,headers=headers)
 return urllib.request.urlopen(r,timeout=20)
login=json.load(req('/system/user/login',json.dumps({'username':os.environ['KERTHUS_ADMIN_PHONE'],'password':password,'scene':'phone'}).encode(),{'Content-Type':'application/json'}))['data']
headers={'access-token':login['access_token'],'tenant-id':str(login['tenant_id']),'app-id':str(login['app_id'])}
png=base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==')
boundary='----kerthus'+uuid.uuid4().hex
body=('--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="pixel.png"\r\nContent-Type: image/png\r\n\r\n').encode()+png+('\r\n--'+boundary+'--\r\n').encode()
res=json.load(req('/app/file/upload',body,{**headers,'Content-Type':'multipart/form-data; boundary='+boundary}));assert res['code']==0,res
key=res['data'];assert isinstance(key,str), 'Legacy upload must return a string key'
obj=req('/files/'+key);assert obj.headers['Content-Type']=='image/png' and obj.read()==png
json.load(req('/system/user/logout',headers=headers))
print('PASS: authenticated multipart → RPC permission → MinIO → confirm → public image fetch')
