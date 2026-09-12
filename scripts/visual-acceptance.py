#!/usr/bin/env python3
"""Long-running isolated fixture for manual in-app browser visual acceptance.

Stop with Ctrl-C or SIGTERM; the server revokes its sessions and drops only its
random test database. The normal frontend and imported datasets are untouched.
"""
import json
import os
import pathlib
import signal
import socket
import struct
import subprocess
import tempfile
import time
import urllib.request
import zlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
PASSWORD = 'VisualAcceptance2026!'
PORT = 15174


def create_logo():
    """Generate an original synthetic PNG without image or network dependencies."""
    rows = []
    for y in range(256):
        row = bytearray()
        for x in range(256):
            color = (25, 69, 164)
            if 42 < x < 214 and 42 < y < 214:
                color = (235, 245, 250)
            if (64 < x < 111 and 65 < y < 190) or (112 < x < 194 and 65 < y < 105):
                color = (35, 155, 127)
            if 112 < x < 181 and 113 < y < 155:
                color = (56, 182, 158)
            row.extend(color)
        rows.append(b'\x00' + row)

    def chunk(kind, data):
        return struct.pack('!I', len(data)) + kind + data + struct.pack('!I', zlib.crc32(kind + data) & 0xffffffff)

    path = ROOT / '.local/visual-acceptance-logo.png'
    path.write_bytes(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('!2I5B', 256, 256, 8, 2, 0, 0, 0)) +
                     chunk(b'IDAT', zlib.compress(b''.join(rows))) + chunk(b'IEND', b''))
    return str(path)


def seed(config):
    token = None

    def api(path, data=None, tenant=1, app=1):
        headers = {'Content-Type': 'application/json', 'tenant-id': str(tenant),
                   'app-id': str(app), 'unit-id': '0', 'section-id': '0'}
        if token:
            headers['access-token'] = token
        request = urllib.request.Request(config['base'] + path, headers=headers,
                                         data=None if data is None else json.dumps(data).encode())
        with urllib.request.urlopen(request, timeout=20) as response:
            body = json.load(response)
        if body.get('code') != 0:
            raise RuntimeError(f"Fixture API {path}: {body.get('msg')}")
        return body['data']

    token = api('/system/user/login', {'username': config['phone'], 'password': PASSWORD, 'scene': 'phone'})['access_token']
    resources = api('/system/app/getGlobalResource')
    basic = next(app for app in resources.values() if app['code'] == 'basic')

    def flatten(nodes):
        return [node for item in nodes for node in [item, *flatten(item.get('children', []))]]

    readonly_ids = [item['id'] for item in flatten(basic['resources'])
                    if item['code'] in ('basic:root', 'basic:home', 'basic:organizations')]
    fixtures = []
    for suffix, title, admin_phone, member_phone in [
            ('A', '视觉验收·青禾科技', '13900001111', '13900001112'),
            ('B', '视觉验收·远山工作室', '13900002221', '13900002222')]:
        tenant = api('/system/tenant/save', {
            'name': title, 'contact_person': f'租户{suffix}管理员',
            'contact_phone': admin_phone, 'contact_email': f'admin-{suffix.lower()}@example.invalid',
            'address': [110000, 110100, 110101], 'address_detail': '隔离验收数据，无真实用户',
            'description': '本租户仅用于基础底座视觉与交互验收', 'expiration_time': 0})
        api('/system/tenant/approve', {'id': tenant, 'verify_status': 1, 'admin_password': PASSWORD})
        orgs = api(f'/system/org/query?tenant_id={tenant}')
        root_org = (orgs['items'] if isinstance(orgs, dict) and 'items' in orgs else orgs)[0]['id']
        department = api('/system/org/save', {'tenant_id': tenant, 'parent_id': root_org,
                            'type': 'section', 'name': f'{suffix}产品研发部', 'short_name': '研发', 'status': 1})
        api('/system/org/save', {'tenant_id': tenant, 'parent_id': root_org,
                                'type': 'section', 'name': f'{suffix}运营支持部', 'short_name': '运营', 'status': 1})
        position = api('/system/position/save', {'tenant_id': tenant, 'org_id': department,
                                                'name': f'{suffix}验收专员', 'status': 1})
        member = api('/system/employee/save', {'tenant_id': tenant, 'phone': member_phone,
                           'password': PASSWORD, 'name': f'租户{suffix}受限成员',
                           'email': f'member-{suffix.lower()}@example.invalid', 'sex': 0,
                           'app_id': basic['id'], 'org_ids': [department], 'position_ids': [position]})
        role = api('/system/role/save', {'tenant_id': tenant, 'name': f'{suffix}组织只读角色', 'status': 1})
        api('/system/role/authRoleResource', {'tenant_id': tenant, 'role_id': role,
            'resource_map': {str(basic['id']): {'ids': readonly_ids, 'scope': {str(i): 0 for i in readonly_ids}}}})
        api('/system/role/relateEmployee', {'tenant_id': tenant, 'role_id': role, 'user_ids': [member], 'action': 'add'})
        fixtures.append({'name': title, 'tenant_id': tenant, 'admin': admin_phone, 'restricted': member_phone,
                         'role_id': role, 'department_id': department, 'member_id': member})
    for name, phone, rejected in [('视觉验收·待审核企业', '13900003331', False),
                                  ('视觉验收·审核驳回企业', '13900003332', True)]:
        tenant = api('/system/tenant/save', {'name': name, 'contact_person': '验收联系人',
                         'contact_phone': phone, 'contact_email': f'{phone}@example.invalid'})
        if rejected:
            api('/system/tenant/approve', {'id': tenant, 'verify_status': 2})
    api('/system/user/logout')
    config['fixtures'] = fixtures
    config['browser'] = f'http://127.0.0.1:{PORT}/#/login'
    config['password'] = PASSWORD
    return config


def main():
    with socket.socket() as probe:
        probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        probe.bind(('127.0.0.1', PORT))
    env = os.environ.copy()
    env.setdefault('KERTHUS_ENV_FILE', str(ROOT / '.local/saas.env'))
    for line in pathlib.Path(env['KERTHUS_ENV_FILE']).read_text().splitlines():
        if line and not line.startswith('#'):
            key, value = line.split('=', 1)
            env.setdefault(key, value)
    env.setdefault('KERTHUS_TEST_MYSQL_DSN', 'root:' + env['KERTHUS_DB_ROOT_PASSWORD'] +
                   '@tcp(127.0.0.1:13306)/?charset=utf8mb4&parseTime=true&clientFoundRows=true')
    env['KERTHUS_ACCEPTANCE_PASSWORD'] = PASSWORD
    env['KERTHUS_ACCEPTANCE_STORAGE'] = 'true'
    env['KERTHUS_ADMIN_URL'] = f'http://127.0.0.1:{PORT}'
    children = []

    def stop(_signum, _frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, stop)
    with tempfile.TemporaryDirectory(prefix='visual-acceptance-', dir=ROOT / '.local') as tmp:
        path = pathlib.Path(tmp)
        env['KERTHUS_ACCEPTANCE_CONFIG'] = str(path / 'connection.json')
        subprocess.run(['go', 'build', '-o', str(path / 'server'), './scripts/acceptance-server'], cwd=ROOT, check=True)
        try:
            server = subprocess.Popen([str(path / 'server')], cwd=ROOT, env=env)
            children.append(server)
            deadline = time.monotonic() + 40
            config_path = pathlib.Path(env['KERTHUS_ACCEPTANCE_CONFIG'])
            while not config_path.exists():
                if server.poll() is not None or time.monotonic() > deadline:
                    raise RuntimeError('Acceptance server failed to start')
                time.sleep(0.2)
            config = seed(json.loads(config_path.read_text()))
            config['supervisor_pid'] = os.getpid()
            config['logo_path'] = create_logo()
            config_path.write_text(json.dumps(config, ensure_ascii=False, indent=2))
            frontend_env = env.copy()
            frontend_env.update({'VITE_PORT': str(PORT), 'VITE_GLOB_API_URL': config['base'],
                'VITE_GLOB_UPLOAD_URL': config['base'] + '/app/file/upload',
                'VITE_GLOB_APP_SHORT_NAME': 'kerthus_visual_acceptance',
                'VITE_GLOB_APP_TITLE': 'Kerthus 基础视觉验收'})
            frontend = subprocess.Popen(['node', 'node_modules/vite/bin/vite.js', '--host', '127.0.0.1',
                                          '--port', str(PORT), '--strictPort'], cwd=ROOT / 'web/admin', env=frontend_env)
            children.append(frontend)
            print(json.dumps({'config_path': str(config_path), **config}, ensure_ascii=False, indent=2), flush=True)
            while all(child.poll() is None for child in children):
                time.sleep(1)
            raise RuntimeError('An acceptance process stopped unexpectedly')
        except KeyboardInterrupt:
            print('Cleaning up isolated visual acceptance environment...', flush=True)
        finally:
            for child in reversed(children):
                if child.poll() is None:
                    child.terminate()
                child.wait(timeout=25)


if __name__ == '__main__':
    main()
