#!/usr/bin/env python3
"""Exercise attachments only against a running disposable visual fixture.

Usage: python3 scripts/smoke-attachments.py .local/visual-acceptance-*/connection.json
Uses fixture A/B admin accounts, never the browser's platform account. The
supervisor removes created fixture objects/database and revokes sessions on exit.
"""
import argparse
import base64
from email.message import Message
import json
import pathlib
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('connection', type=pathlib.Path)
    args = parser.parse_args()
    path = args.connection.resolve()
    if not path.parent.name.startswith('visual-acceptance-') or path.name != 'connection.json':
        raise SystemExit('Only a disposable visual-acceptance connection.json is supported.')
    cfg = json.loads(path.read_text())
    base = cfg['base'].rstrip('/')
    parsed = urllib.parse.urlparse(base)
    if parsed.hostname != '127.0.0.1' or parsed.scheme != 'http' or parsed.port in (18080, 19090, 15173):
        raise SystemExit('Refusing to use a non-fixture service address.')
    fixtures = cfg.get('fixtures', [])
    if len(fixtures) < 2 or any(f['admin'] == cfg['phone'] for f in fixtures[:2]):
        raise SystemExit('Expected distinct A/B synthetic tenant accounts.')
    sessions = []
    checks = []

    def request(route, headers=None, data=None, method=None):
        req = urllib.request.Request(base + route, data=data, headers=headers or {}, method=method)
        try:
            response = urllib.request.urlopen(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, response.headers, response.read()

    def payload(route, headers=None, data=None):
        status, _, body = request(route, headers, data)
        result = json.loads(body)
        if status != 200 or result.get('code') != 0:
            raise AssertionError(f'Fixture request failed: {route.split("?")[0]}, HTTP {status}, code {result.get("code")}')
        return result

    def upload(headers, name, body, purpose=''):
        boundary = 'kerthus-' + uuid.uuid4().hex
        data = bytearray()
        fields = {'directory': '../../private/other-tenant', 'param': '../../escape', 'purpose': purpose}
        for key, value in fields.items():
            data.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode())
        data.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\nContent-Type: application/octet-stream\r\n\r\n'.encode())
        data.extend(body)
        data.extend(f'\r\n--{boundary}--\r\n'.encode())
        return payload('/app/file/upload', {**headers, 'Content-Type': 'multipart/form-data; boundary=' + boundary}, bytes(data))

    def denied(route, headers, allowed=(401, 403, 404)):
        status, response_headers, _ = request(route, headers)
        if status not in allowed or response_headers.get('Location'):
            raise AssertionError(f'Unauthorized file request was not rejected: HTTP {status}')

    try:
        for fixture in fixtures[:2]:
            login = payload('/system/user/login', {'Content-Type': 'application/json'}, json.dumps({
                'username': fixture['admin'], 'password': cfg['password'], 'scene': 'phone'
            }).encode())['data']
            if login['tenant_id'] != fixture['tenant_id']:
                raise AssertionError('Synthetic account selected an unexpected tenant.')
            sessions.append({'access-token': login['access_token'], 'tenant-id': str(login['tenant_id']), 'app-id': str(login['app_id'])})
        a, b = sessions
        limits = payload('/app/file/limits')['data']
        assert limits['image_max_bytes'] == 10 << 20 and limits['attachment_max_bytes'] >= 1 << 20
        checks.append('configured upload limits returned in bytes')
        contents = b'%PDF-1.4\n' + ('\u9694\u79bb\u9a8c\u6536\u00b7\u9644\u4ef6\u5b57\u8282\u5bf9\u7b49\n' * 50).encode('utf-8')
        name = '\u5b63\u5ea6\u62a5\u544a-\u9644\u4ef6\u9a8c\u6536.pdf'
        uploaded = upload(a, name, contents, 'attachment')
        key = uploaded['data']
        assert isinstance(key, str) and key.startswith('private/') and '..' not in key
        assert uploaded['file']['private'] and uploaded['file']['filename'] == name
        assert uploaded['file']['size'] == len(contents)
        download = '/app/file/download?key=' + urllib.parse.quote(key, safe='')
        status, headers, returned = request(download, a)
        message = Message()
        message['Content-Disposition'] = headers.get('Content-Disposition', '')
        assert status == 200 and returned == contents and message.get_filename() == name
        assert message.get_content_disposition() == 'attachment'
        assert headers.get('Cache-Control') == 'private, no-store'
        checks.append('Chinese filename attachment: authenticated upload/download byte equality')
        status, headers, returned = request(download, a, method='HEAD')
        assert status == 200 and returned == b'' and int(headers['Content-Length']) == len(contents)
        checks.append('HEAD returns metadata without body')
        denied(download, b)
        denied(download, {**a, 'app-id': '999999999'})
        denied(download, {})
        denied('/files/' + key, {}, (404,))
        checks.append('cross-tenant, incorrect app, anonymous, and public file routes rejected')
        png = base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==')
        image = upload(a, 'fixture.png', png)
        assert image['data'].startswith('public/') and not image['file']['private']
        status, headers, returned = request('/files/' + image['data'])
        assert status == 200 and headers['Content-Type'] == 'image/png' and returned == png
        checks.append('legacy image upload returns string key and remains publicly readable')
        report = {'result': 'PASS', 'base': base, 'checks': checks, 'created_private_files': 1, 'created_public_images': 1,
                  'cleanup': 'Disposable supervisor removes fixture objects and database; synthetic sessions logged out.'}
        (path.parent / 'attachments-http-report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
        print('PASS: ' + '; '.join(checks))
    finally:
        for headers in sessions:
            request('/system/user/logout', headers)


if __name__ == '__main__':
    main()
