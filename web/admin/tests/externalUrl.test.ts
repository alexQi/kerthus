import assert from 'node:assert/strict';
import {
  externalUrl,
  isExternalDestination,
  isThirdPartyApp,
  normalizeAppType,
  openExternalUrl,
} from '../src/utils/externalUrl';

for (const value of [
  'https://example.com/path?q=a%20b#part',
  'http://127.0.0.1:8080/x',
  'HTTPS://example.com',
  'https://例子.测试/文档',
]) {
  assert.equal(externalUrl(value), value);
  assert.equal(isExternalDestination(value), true);
}
for (const value of [
  '',
  'javascript:alert(1)',
  'data:text/html,<script>x</script>',
  'file:///tmp/a',
  '//example.com',
  '/local',
  'https://user:secret@example.com',
  'https://example.com\\@evil.test',
  'https://exa\nmple.com',
  'https://example.com/path with spaces',
  'https://',
  'https:////example.com',
  'https://@example.com',
  'https://%00.example.com',
]) {
  assert.equal(externalUrl(value), '', `must reject ${JSON.stringify(value)}`);
}
assert.equal(
  externalUrl('  https://example.com/a?existing=1  '),
  'https://example.com/a?existing=1',
);
assert.equal(normalizeAppType('thrid'), 'third');
assert.equal(isThirdPartyApp({ type: 'thrid' }), true);
assert.equal(isThirdPartyApp({ type: 'third' }), true);
assert.equal(isThirdPartyApp({ type: 'self' }), false);
assert.equal(isExternalDestination('/basic/dashboard'), false);
assert.equal(isExternalDestination('//evil.test'), true);
assert.equal(isExternalDestination('javascript:alert(1)'), true);
let clicked: Record<string, unknown> | undefined;
globalThis.document = {
  createElement: () => {
    const link = {
      href: '',
      target: '',
      rel: '',
      referrerPolicy: '',
      click() {
        clicked = { ...this };
      },
    };
    return link;
  },
} as unknown as Document;
assert.equal(openExternalUrl('https://example.com/path?existing=1'), true);
assert.equal(clicked?.href, 'https://example.com/path?existing=1');
assert.equal(clicked?.target, '_blank');
assert.equal(clicked?.rel, 'noopener noreferrer');
assert.equal(clicked?.referrerPolicy, 'no-referrer');
clicked = undefined;
assert.equal(openExternalUrl('javascript:alert(1)'), false);
assert.equal(clicked, undefined);
console.log('external URL boundary and isolated link checks passed');
