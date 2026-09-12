// Optional browser acceptance test: install playwright or set KERTHUS_PLAYWRIGHT_PATH.
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');
const root = path.resolve(__dirname, '..');
(async () => {
  const env = Object.fromEntries(fs.readFileSync(path.join(root, '.local/saas.env'), 'utf8').trim().split('\n').filter(x => x && !x.startsWith('#')).map(x => [x.slice(0, x.indexOf('=')), x.slice(x.indexOf('=') + 1)]));
  const password = process.env.KERTHUS_LOGIN_PASSWORD || env.KERTHUS_ADMIN_PASSWORD;
  if (!password) throw new Error('Set KERTHUS_LOGIN_PASSWORD to use an original account; imported passwords are unchanged.');
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1440, height: 960 } });
  const errors = []; const apiErrors = []; const checked = [];
  const imported = env.KERTHUS_DATASET === 'beehive_saas';
  page.on('pageerror', e => errors.push(e.message));
  page.on('response', async r => {
    if (r.url().startsWith('http://127.0.0.1:18080/') && !r.url().includes('/files/')) {
      try { const data = await r.json(); if (data.code) apiErrors.push({ path: new URL(r.url()).pathname, code: data.code, msg: data.msg }); } catch {}
    }
  });
  try {
    const base = 'http://127.0.0.1:15173';
    await page.goto(base + '/#/login');
    await page.locator('input').first().fill(env.KERTHUS_ADMIN_PHONE);
    await page.locator('input[type="password"]').fill(password);
    await page.locator('button.ant-btn-primary').first().click();
    await page.waitForURL('**/#/system/dashboard', { timeout: 30000 });
    await page.getByText('我的应用', { exact: true }).waitFor();
    for (const route of (imported ? ['tenant/index', 'application/index', 'application/resource', 'application/authorize', 'tenant/user', 'tenant/detail/1', 'application/auth'] : ['tenants', 'applications', 'resources', 'authorizations', 'accounts', 'tenant/detail/1', 'application/auth'])) {
      await page.goto(base + '/#/system/' + route);
      await page.waitForTimeout(1200);
      const body = await page.locator('body').innerText();
      if (body.includes('抱歉，您访问的页面不存在')) throw new Error('Missing page: ' + route);
      checked.push('system/' + route);
    }
    await page.goto(base + '/#/system/dashboard');
    await page.locator('.ant-card-grid').filter({ hasText: /企业管理|基础/ }).first().click();
    await page.locator('.ant-modal-confirm .ant-btn-primary').click();
    await page.waitForURL('**/#/basic/dashboard', { timeout: 15000 });
    for (const route of (imported ? ['user/index', 'user/org', 'user/position', 'system/role'] : ['members', 'organizations', 'positions', 'roles', 'audit'])) {
      await page.goto(base + '/#/basic/' + route);
      await page.waitForTimeout(1000);
      checked.push('basic/' + route);
    }
    await page.goto(base + '/#/basic/dashboard');
    await page.getByText('我的应用', { exact: true }).waitFor();
    await page.waitForTimeout(700);
    await page.screenshot({ path: path.join(root, '.local/dashboard-basic.png'), fullPage: true });
    if (errors.length || apiErrors.length) throw new Error(JSON.stringify({ errors, apiErrors }));
    console.log(JSON.stringify({ result: 'PASS', pages: checked, pageErrors: errors, apiErrors }));
  } catch (e) {
    await page.screenshot({ path: path.join(root, '.local/browser-failure.png'), fullPage: true });
    console.error(JSON.stringify({ url: page.url(), errors, apiErrors }));
    throw e;
  } finally { await browser.close(); }
})().catch(e => { console.error(e.message); process.exit(1); });
