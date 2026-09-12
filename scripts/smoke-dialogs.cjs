// Controlled confirmation regression checks plus real application-switch UI on a disposable backend.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');

(async () => {
  const config = JSON.parse(fs.readFileSync(process.env.KERTHUS_ACCEPTANCE_CONFIG, 'utf8'));
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } });
  page.setDefaultTimeout(12000);
  const errors = [], failures = [], checked = [];
  let step = 'initialize dialogs', token;
  page.on('pageerror', error => errors.push(error.message));
  await page.route('http://127.0.0.1:18080/**', async route => {
    const target = new URL(route.request().url());
    const response = await route.fetch({ url: config.base + target.pathname + target.search });
    const body = await response.json();
    if (target.pathname === '/system/user/login' && body.code === 0) token = body.data;
    if (body.code) failures.push({ path: target.pathname, code: body.code });
    await route.fulfill({ response });
  });
  const base = process.env.KERTHUS_ADMIN_URL || 'http://127.0.0.1:15173';
  const modal = () => page.locator('.ant-modal-content:visible');
  const ok = () => modal().getByRole('button', { name: /确.*[定认]/ });
  const api = async (path, data) => {
    const response = await fetch(config.base + path, {
      method: data === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', 'access-token': token.access_token, 'tenant-id': '1', 'app-id': '1' },
      body: data === undefined ? undefined : JSON.stringify(data),
    });
    const body = await response.json();
    assert.equal(body.code, 0, `${path}: ${body.msg}`);
    return body.data;
  };
  const resetAction = () => page.locator('tr.ant-table-row').filter({ hasText: '弹窗密码验收员' }).locator('[class*=basic-table-action] button').last();
  const resetAndVerify = async suffix => {
    await resetAction().click();
    await modal().locator('input[type=password]').fill(config.password + suffix);
    const response = page.waitForResponse(r => new URL(r.url()).pathname === '/system/user/resetPassword');
    await ok().click();
    const result = await response;
    assert.equal(result.request().postDataJSON().password.length, config.password.length + suffix.length);
    assert.equal((await result.json()).code, 0);
    await modal().waitFor({ state: 'hidden' });
    await api('/system/user/login', { scene: 'phone', username: '13900005555', password: config.password + suffix });
  };
  try {
    await page.goto(base + '/#/login');
    await page.evaluate(async () => {
      window.dialogModule = await import('/src/components/Modal/src/confirmDialog.ts');
      window.messages = (await import('/src/hooks/web/useMessage.tsx')).useMessage();
    });
    step = 'rejected confirmation stays open and successful retry closes';
    await page.evaluate(() => {
      window.attempts = 0;
      window.dialogModule.createConfirmDialog({ title: '重试验收', content: '验收内容',
        async onOk() { if (++window.attempts === 1) throw new Error('expected validation failure'); },
      });
    });
    await ok().click();
    await page.waitForFunction(() => window.attempts === 1);
    assert.equal(await modal().count(), 1);
    await ok().click();
    await modal().waitFor({ state: 'hidden' });
    assert.equal(await page.evaluate(() => window.attempts), 2);
    checked.push(step);

    step = 'cancel closes dialog and permits reopening';
    await page.evaluate(() => {
      window.canceled = 0;
      window.dialogModule.createConfirmDialog({ title: '取消验收', content: '可取消', onCancel() { window.canceled++; } });
    });
    await modal().getByRole('button', { name: /取\s*消/ }).click();
    await modal().waitFor({ state: 'hidden' });
    assert.equal(await page.evaluate(() => window.canceled), 1);
    await page.evaluate(() => window.dialogModule.createConfirmDialog({ title: '再次打开', content: '已重开' }));
    await ok().click();
    await modal().waitFor({ state: 'hidden' });
    checked.push(step);

    step = 'update changes content and destroy removes overlay exactly once';
    await page.evaluate(() => {
      window.dialogCloseCount = 0;
      window.handle = window.dialogModule.createConfirmDialog({ title: '旧标题', content: '旧内容', afterClose() { window.dialogCloseCount++; } });
      window.handle.update(current => ({ ...current, title: '新标题', content: '新内容' }));
    });
    await modal().getByText('新标题', { exact: true }).waitFor();
    await modal().getByText('新内容', { exact: true }).waitFor();
    await page.evaluate(() => { window.handle.destroy(); window.handle.destroy(); });
    await modal().waitFor({ state: 'hidden' });
    assert.equal(await page.evaluate(() => window.dialogCloseCount), 1);
    checked.push(step);

    step = 'error message has one dismiss button';
    await page.evaluate(() => window.messages.createErrorModal({ title: '错误验收', content: '可关闭的错误' }));
    await modal().waitFor();
    assert.equal(await modal().getByRole('button').count(), 1);
    await ok().click();
    await modal().waitFor({ state: 'hidden' });
    checked.push(step);

    step = 'pending promise prevents double submission and closes after resolution';
    await page.evaluate(() => {
      window.submits = 0;
      window.dialogModule.createConfirmDialog({ title: '等待验收', content: '正在等待',
        onOk() { window.submits++; return new Promise(resolve => { window.resolveDialog = resolve; }); },
      });
    });
    await ok().click();
    assert.equal(await modal().getByRole('button', { name: /取\s*消/ }).isDisabled(), true);
    assert.equal(await page.evaluate(() => window.submits), 1);
    await page.evaluate(() => window.resolveDialog());
    await modal().waitFor({ state: 'hidden' });
    checked.push(step);

    step = 'destroy-all clears route-blocking overlays';
    await page.evaluate(() => {
      window.dialogModule.createConfirmDialog({ title: '清理一', content: '待清理' });
      window.dialogModule.createConfirmDialog({ title: '清理二', content: '待清理' });
      window.dialogModule.destroyAllDialogs();
    });
    assert.equal(await page.locator('.ant-modal-wrap:visible').count(), 0);
    checked.push(step);

    step = 'real application switch cancel and confirm both remove dialog';
    await page.locator('input').first().fill(config.phone);
    await page.locator('input[type=password]').fill(config.password);
    await page.locator('button.ant-btn-primary').first().click();
    await page.waitForURL('**/#/system/dashboard');
    await page.locator('.ant-card-grid').filter({ hasText: '企业管理' }).click();
    await modal().getByRole('button', { name: /取\s*消/ }).click();
    await modal().waitFor({ state: 'hidden' });
    assert.ok(page.url().endsWith('/system/dashboard'));
    await page.locator('.ant-card-grid').filter({ hasText: '企业管理' }).click();
    await ok().click();
    await page.waitForURL('**/#/basic/dashboard');
    await modal().waitFor({ state: 'hidden' });
    assert.equal(await page.locator('.ant-modal-wrap:visible').count(), 0);
    checked.push(step);

    step = 'member password dialog keeps failed input and accepts corrected password';
    const orgs = await api('/system/org/query');
    await api('/system/employee/save', {
      tenant_id: 1, name: '弹窗密码验收员', phone: '13900005555', email: 'dialog@example.invalid',
      password: config.password, app_id: 2, org_ids: [orgs[0].id], position_ids: [], sex: 1,
    });
    await page.goto(base + '/#/basic/members');
    await resetAction().click();
    await modal().locator('input[type=password]').fill('short');
    const rejected = page.waitForResponse(r => new URL(r.url()).pathname === '/system/user/resetPassword');
    await ok().click();
    assert.equal((await (await rejected).json()).code, 400);
    await page.getByText('密码长度应为10至72字节', { exact: true }).waitFor();
    assert.equal(await modal().locator('input[type=password]').inputValue(), 'short');
    await modal().getByRole('button', { name: /取\s*消/ }).click();
    await modal().waitFor({ state: 'hidden' });
    await resetAndVerify('R');
    checked.push(step);

    step = 'tenant-detail password dialog submits entered value and closes';
    await page.goto(base + '/#/basic/dashboard');
    await page.locator('.ant-card-grid').filter({ hasText: '平台管理' }).click();
    await ok().click();
    await page.waitForURL('**/#/system/dashboard');
    await modal().waitFor({ state: 'hidden' });
    await page.goto(base + '/#/system/tenant/detail/1');
    await page.getByRole('tab', { name: '员工', exact: true }).click();
    await resetAndVerify('S');
    checked.push(step);

    assert.deepEqual(errors, []);
    assert.deepEqual(failures, [{ path: '/system/user/resetPassword', code: 400 }]);
    console.log(JSON.stringify({ result: 'PASS', checked, pageErrors: errors, expectedRejectedAPI: failures }));
  } catch (error) {
    await page.screenshot({ path: '.local/dialogs-failure.png', fullPage: true });
    console.error(JSON.stringify({ step, checked, errors, failures }));
    throw error;
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exit(1); });
