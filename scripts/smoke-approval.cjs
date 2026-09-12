// Used by test-browser-crud.py: real UI requests are forwarded only to its disposable server.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');

(async () => {
  const config = JSON.parse(fs.readFileSync(process.env.KERTHUS_ACCEPTANCE_CONFIG, 'utf8'));
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } });
  page.setDefaultTimeout(12000);
  const errors = [], failedAPI = [], checked = [];
  let token, step = 'login';
  page.on('pageerror', e => errors.push(e.message));
  page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text().replaceAll(config.password, '[redacted]')); });
  await page.route('http://127.0.0.1:18080/**', async route => {
    const request = route.request();
    const target = new URL(request.url());
    const response = await route.fetch({ url: config.base + target.pathname + target.search });
    const body = await response.json();
    if (body.code) failedAPI.push({ path: target.pathname, code: body.code, msg: body.msg });
    if (target.pathname === '/system/user/login' && body.code === 0) token = body.data;
    await route.fulfill({ response });
  });
  const base = process.env.KERTHUS_ADMIN_URL || 'http://127.0.0.1:15173';
  const go = path => page.goto(base + '/#' + path);
  const drawer = () => page.locator('.ant-drawer-content:visible');
  const modal = () => page.locator('.ant-modal-content:visible');
  const field = (container, name) => container.locator('.ant-form-item').filter({ has: page.locator('label').filter({ hasText: new RegExp('^' + name + '$') }) }).locator('input').first();
  const row = name => page.locator('tr.ant-table-row').filter({ hasText: name });
  const actions = name => row(name).locator('[class*="basic-table-action"] button');
  const save = async (container, endpoint) => {
    const response = page.waitForResponse(r => new URL(r.url()).pathname === endpoint && r.request().method() === 'POST');
    await container.locator('button.ant-btn-primary').last().click();
    const body = await (await response).json();
    assert.equal(body.code, 0, body.msg);
    await container.waitFor({ state: 'hidden' });
    return body.data;
  };
  const api = async (path, data, tenantId = 1, appId = 1) => {
    const response = await fetch(config.base + path, {
      method: data === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', 'access-token': token.access_token,
        'tenant-id': String(tenantId), 'app-id': String(appId), 'unit-id': '0', 'section-id': '0' },
      body: data === undefined ? undefined : JSON.stringify(data),
    });
    const body = await response.json();
    assert.equal(body.code, 0, `${path}: ${body.msg}`);
    return body.data;
  };
  try {
    await go('/login');
    await page.locator('input').first().fill(config.phone);
    await page.locator('input[type=password]').fill(config.password);
    await page.locator('button.ant-btn-primary').first().click();
    await page.waitForURL('**/#/system/dashboard');
    assert.equal((await api('/system/user/auth')).platform_admin, true);

    step = 'tenant create';
    await go('/system/tenants');
    await page.getByRole('button', { name: /新\s*增/, exact: true }).click();
    await field(drawer(), '企业名称').fill('验收租户');
    await field(drawer(), '联系人').fill('验收管理员');
    await field(drawer(), '联系方式').fill('13900008888');
    await field(drawer(), '联系邮箱').fill('acceptance@example.invalid');
    await field(drawer(), '详细地址').fill('仅用于隔离测试');
    await field(drawer(), '地区').click();
    for (const name of ['验收省', '验收市', '验收区']) await page.getByText(name, { exact: true }).last().click();
    const tenantId = await save(drawer(), '/system/tenant/save');
    await row('验收租户').waitFor();
    assert.equal((await api('/system/tenant/info?id=' + tenantId)).logo || '', '');
    checked.push(step);

    step = 'tenant edit preserves unlimited expiry';
    await actions('验收租户').nth(2).click();
    assert.equal(await field(drawer(), '有效期').inputValue(), '');
    await field(drawer(), '企业名称').fill('验收租户修改');
    await save(drawer(), '/system/tenant/save');
    const tenant = await api('/system/tenant/info?id=' + tenantId);
    assert.equal(tenant.name, '验收租户修改');
    assert.equal(tenant.expiration_time, 0);
    checked.push(step);

    step = 'tenant approval';
    await actions('验收租户修改').first().click();
    await modal().locator('input[type=password]').fill('canceled-value');
    await modal().getByRole('button', { name: /取\s*消/ }).click();
    await modal().waitFor({ state: 'hidden' });
    await actions('验收租户修改').first().click();
    assert.equal(await modal().locator('input[type=password]').inputValue(), '');
    await modal().locator('input[type=password]').fill('short');
    await modal().getByRole('button', { name: /确.*[定认]/ }).click();
    await page.getByText('密码长度应为 10–72 字节', { exact: true }).waitFor();
    assert.equal(await modal().count(), 1);
    await modal().locator('input[type=password]').fill('');
    await modal().locator('input[type=password]').pressSequentially(config.password);
    const approved = page.waitForResponse(r => new URL(r.url()).pathname === '/system/tenant/approve');
    await modal().getByRole('button', { name: /确.*[定认]/ }).click();
    const approvalResponse = await approved;
    assert.equal(approvalResponse.request().postDataJSON().admin_password?.length, config.password.length, 'approval password input must reach the request');
    assert.equal((await approvalResponse.json()).code, 0);
    await modal().waitFor({ state: 'hidden' });
    assert.equal((await api('/system/tenant/info?id=' + tenantId)).verify_status, 1);
    checked.push(step);

    step = 'bulk password input and successful modal cleanup';
    const bulkId = await api('/system/tenant/save', {
      name: '批量输入验收', contact_person: '批量管理员', contact_phone: '13900006666',
      contact_email: 'bulk@example.invalid', expiration_time: 0,
    });
    await page.reload();
    await actions('批量输入验收').first().click();
    await modal().locator('input[type=password]').fill(config.password);
    const bulkApproved = page.waitForResponse(r => new URL(r.url()).pathname === '/system/tenant/approve');
    await modal().getByRole('button', { name: /确.*[定认]/ }).click();
    const bulkResponse = await bulkApproved;
    assert.equal(bulkResponse.request().postDataJSON().admin_password.length, config.password.length);
    assert.equal((await bulkResponse.json()).code, 0);
    await modal().waitFor({ state: 'hidden' });
    assert.equal((await api('/system/tenant/info?id=' + bulkId)).verify_status, 1);
    assert.equal(await page.locator('.ant-modal-wrap:visible').count(), 0);
    checked.push(step);

    await api('/system/user/logout');
    assert.deepEqual(failedAPI, []);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ result: 'PASS', checked, pageErrors: errors, failedAPI }));
  } catch (error) {
    await page.screenshot({ path: '.local/approval-failure.png', fullPage: true });
    fs.writeFileSync('.local/approval-failure.html', await page.content());
    console.error(JSON.stringify({ step, checked, errors, failedAPI }));
    throw error;
  } finally { await browser.close(); }
})().catch(e => { console.error(e.message); process.exit(1); });
