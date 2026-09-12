#!/usr/bin/env python3
"""Keep application implementations independent of the SaaS implementation."""
import json,subprocess
raw=subprocess.check_output(['go','list','-json','./internal/...'],text=True)
decoder=json.JSONDecoder();errors=[]
while raw.strip():
 package,n=decoder.raw_decode(raw.lstrip());raw=raw.lstrip()[n:]
 source=package['ImportPath']
 for target in package.get('Imports',[]):
  if '/internal/apps/' in source:
   owner=source.split('/internal/apps/')[1].split('/')[0]
   if '/internal/saas/' in target or ('/internal/apps/' in target and target.split('/internal/apps/')[1].split('/')[0]!=owner):errors.append(source+' → '+target)
  if '/internal/saas/' in source and '/internal/apps/' in target:errors.append(source+' → '+target)
  if '/internal/appkit/' in source and ('/internal/saas/' in target or '/internal/apps/' in target):errors.append(source+' → '+target)
if errors:raise SystemExit('Forbidden implementation imports:\n'+'\n'.join(errors))
print('Application import boundaries passed')
