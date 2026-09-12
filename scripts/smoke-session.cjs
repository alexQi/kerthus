// Run against the Vite dev server; all API responses are isolated browser fixtures.
// Install playwright or set KERTHUS_PLAYWRIGHT_PATH. No database credentials needed.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');

(async () => {
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage();
  const errors = [];
  const logoutTokens = [];
  let releaseLogout;
  page.on('pageerror', e => errors.push(e.message));
  await page.route('**/system/user/logout*', async route => {
    logoutTokens.push(route.request().headers()['access-token']);
    if (releaseLogout) await releaseLogout;
    await route.fulfill({ json: { code: 401, msg: 'expired logout', data: null } });
  });
  try {
    await page.goto((process.env.KERTHUS_ADMIN_URL || 'http://127.0.0.1:15173') + '/#/login');
    await page.locator('input[type=password]').waitFor();
    await page.evaluate(async () => {
      const { useUserStoreWithOut } = await import('/src/store/modules/user.ts');
      const { defHttp } = await import('/src/utils/http/axios/index.ts');
      const { createMessage } = (await import('/src/hooks/web/useMessage.tsx')).useMessage();
      window.sessionTest = { store: useUserStoreWithOut(), http: defHttp, messages: createMessage };
    });
    const platformPrivilege = await page.evaluate(async () => {
      const { usePermissionStoreWithOut } = await import('/src/store/modules/permission.ts');
      const { usePermission } = await import('/src/hooks/web/usePermission.ts');
      const permission = usePermissionStoreWithOut();
      permission.resetState();
      permission.setRoles(['admin']);
      const roleNameOnly = usePermission().hasPermission('ungranted:operation');
      permission.platformAdmin = true;
      const explicitPlatform = usePermission().hasPermission('ungranted:operation');
      permission.resetState();
      return { roleNameOnly, explicitPlatform, cleared: permission.getIsPlatformAdmin };
    });
    assert.deepEqual(platformPrivilege, { roleNameOnly: false, explicitPlatform: true, cleared: false });
    const seed = token => page.evaluate(value => {
      const { store, messages } = window.sessionTest;
      messages.destroy();
      store.setTokenInfo({ access_token: value, user_id: 1, tenant_id: 1, app_id: 1 });
      store.setSaasConf({ tenantId: 1, appId: 1 });
    }, token);

    // Mixed HTTP/envelope failures arrive together. Only the first may expire the session.
    const pending = [];
    await page.route('**/__session-test/burst*', async route => {
      pending.push(route);
      if (pending.length === 12) {
        await Promise.all(pending.map((r, i) => r.fulfill({
          status: i % 2 ? 401 : 200,
          json: { code: 401, msg: 'expired', data: null },
        })));
      }
    });
    await seed('expired-session');
    await page.evaluate(async () => {
      await Promise.allSettled(Array.from({ length: 12 }, (_, i) =>
        window.sessionTest.http.get({ url: '/__session-test/burst', params: { i } }),
      ));
    });
    assert.equal(await page.evaluate(() => window.sessionTest.store.getTokenInfo.access_token), undefined);
    assert.equal(await page.locator('.ant-message-notice').count(), 1);
    assert.equal(logoutTokens.length, 0, 'expiry must not call logout');
    assert.ok(page.url().endsWith('#/login'));

    // A public login failure reports bad credentials without touching a session.
    let loginToken;
    await page.route('**/system/user/login', async route => {
      loginToken = route.request().headers()['access-token'];
      await route.fulfill({ json: { code: 401, msg: '账号或密码不正确', data: null } });
    });
    await seed('still-valid-session');
    const loginError = await page.evaluate(async () => {
      const { loginApi } = await import('/src/api/common/user.ts');
      try { await loginApi({ username: 'test', password: 'invalid', scene: 'phone' }, 'message'); }
      catch (e) { return e.message; }
    });
    assert.equal(loginToken, undefined);
    assert.equal(loginError, '账号或密码不正确');
    assert.equal(await page.evaluate(() => window.sessionTest.store.getTokenInfo.access_token), 'still-valid-session');
    assert.equal(await page.locator('.ant-message-notice').count(), 1);
    assert.ok((await page.locator('.ant-message').innerText()).includes('账号或密码不正确'));
    assert.equal(logoutTokens.length, 0);

    // Old requests completing after re-login cannot invalidate the new token.
    for (const status of [200, 401]) {
      let arrived;
      const request = new Promise(resolve => { arrived = resolve; });
      let reply;
      const response = new Promise(resolve => { reply = resolve; });
      await page.route('**/__session-test/late*', async route => {
        arrived();
        await response;
        await route.fulfill({ status, json: { code: 401, msg: 'old session', data: null } });
      });
      await seed('old-session');
      const result = page.evaluate(() => window.sessionTest.http.get({ url: '/__session-test/late' }).catch(() => null));
      await request;
      await seed('new-session');
      reply();
      await result;
      assert.equal(await page.evaluate(() => window.sessionTest.store.getTokenInfo.access_token), 'new-session');
      assert.equal(await page.locator('.ant-message-notice').count(), 0);
      await page.unroute('**/__session-test/late*');
    }

    // Manual logout invalidates the captured token once and clears locally immediately.
    let finishLogout;
    releaseLogout = new Promise(resolve => { finishLogout = resolve; });
    await seed('manual-session');
    await page.evaluate(() => {
      window.logoutResult = Promise.all([
        window.sessionTest.store.logout(), window.sessionTest.store.logout(),
      ]);
    });
    await page.waitForFunction(() => !window.sessionTest.store.getTokenInfo.access_token);
    await seed('login-during-logout');
    finishLogout();
    await page.evaluate(() => window.logoutResult);
    assert.deepEqual(logoutTokens, ['manual-session']);
    assert.equal(await page.evaluate(() => window.sessionTest.store.getTokenInfo.access_token), 'login-during-logout');
    assert.equal(await page.locator('.ant-message-notice').count(), 0);
    assert.deepEqual(errors, []);
    console.log('PASS: concurrent expiry, bad login, stale 401, and repeated/delayed logout; no recursive logout or page errors');
  } finally {
    await browser.close();
  }
})().catch(e => { console.error(e); process.exit(1); });
