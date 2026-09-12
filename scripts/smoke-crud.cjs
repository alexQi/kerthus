// Used by test-browser-crud.py: real UI requests are forwarded only to its disposable server.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');

(async () => {
  const config = JSON.parse(fs.readFileSync(process.env.KERTHUS_ACCEPTANCE_CONFIG, 'utf8'));
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } });
  page.setDefaultTimeout(12000);
  const errors = [], failedAPI = [], checked = [], optionRequests = [];
  let token, step = 'login';
  page.on('pageerror', e => errors.push(e.message));
  await page.route('http://127.0.0.1:18080/**', async route => {
    const request = route.request();
    const target = new URL(request.url());
    if (['/system/org/query', '/system/employee/getTenantApps', '/system/position/getItems'].includes(target.pathname)) optionRequests.push(target.pathname + target.search);
    const response = await route.fetch({ url: config.base + target.pathname + target.search });
    const body = await response.json();
    if (body.code) failedAPI.push({ path: target.pathname, code: body.code, msg: body.msg });
    if (target.pathname === '/system/user/login' && body.code === 0) token = body.data;
    await route.fulfill({ response });
  });
  const base = process.env.KERTHUS_ADMIN_URL || 'http://127.0.0.1:15173';
  const go = path => page.goto(base + '/#' + path);
  const drawer = () => page.locator('.ant-drawer-open .ant-drawer-content');
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
    await modal().locator('input[type=password]').fill(config.password);
    const approved = page.waitForResponse(r => new URL(r.url()).pathname === '/system/tenant/approve');
    await modal().getByRole('button', { name: /确.*[定认]/ }).click();
    const approvalResponse = await approved;
    assert.equal(approvalResponse.request().postDataJSON().admin_password?.length, config.password.length, 'approval password input must reach the request');
    assert.equal((await approvalResponse.json()).code, 0);
    await modal().waitFor({ state: 'hidden' });
    assert.equal((await api('/system/tenant/info?id=' + tenantId)).verify_status, 1);
    checked.push(step);

    step = 'cross-tenant member edit';
    await go('/system/tenant/detail/' + tenantId);
    await page.getByRole('tab', { name: '员工', exact: true }).click();
    await actions('验收管理员').first().click();
    await field(drawer(), '姓名').fill('验收管理员修改');
    await save(drawer(), '/system/employee/save');
    const members = await api('/system/employee/query?tenant_id=' + tenantId);
    assert.equal(members.items[0].name, '验收管理员修改');
    checked.push(step);

    step = 'member create and disable in target tenant';
    await page.getByRole('button', { name: /新\s*增/, exact: true }).click();
    await field(drawer(), '手机号码').fill('13900007777');
    await field(drawer(), '密码').fill(config.password);
    await field(drawer(), '姓名').fill('验收普通成员');
    for (const [label, selection] of [['默认应用', '企业管理'], ['所属机构', '验收租户修改'], ['所属岗位', '管理员']]) {
      await field(drawer(), label).click();
      await page.locator('.ant-select-dropdown:visible').getByText(selection, { exact: true }).click();
      await field(drawer(), '姓名').click();
    }
    const memberId = await save(drawer(), '/system/employee/save');
    const memberStatus = page.waitForResponse(r => new URL(r.url()).pathname === '/system/employee/setStatus');
    await row('验收普通成员').getByRole('switch').click();
    assert.equal((await (await memberStatus).json()).code, 0);
    const targetMembers = (await api('/system/employee/query?tenant_id=' + tenantId)).items;
    assert.equal(targetMembers.find(x => x.id === memberId).status, 0);
    assert.equal(targetMembers.find(x => x.name === '验收管理员修改').status, 1);
    checked.push(step);

    step = 'global account edit persists';
    await go('/system/accounts');
    await page.getByRole('button', { name: '管理租户成员', exact: true }).waitFor();
    await page.waitForFunction(() => !document.querySelector('[class*="-leave-active"]'));
    await actions('验收管理员修改').first().click();
    await field(drawer(), '姓名').fill('验收全局账号修改');
    await save(drawer(), '/system/user/modifyInfo');
    assert.equal((await api('/system/employee/query?tenant_id=' + tenantId)).items[0].name, '验收全局账号修改');
    checked.push(step);

    step = 'application switch';
    await go('/system/dashboard');
    await page.locator('.ant-card-grid').filter({ hasText: '企业管理' }).click();
    await modal().getByRole('button', { name: /确.*[定认]/ }).click();
    await page.waitForURL('**/#/basic/dashboard');
    checked.push(step);

    step = 'organization create';
    await go('/basic/organizations');
    await page.getByRole('button', { name: /新增节点/ }).click();
    await field(page, '名称').fill('验收组织');
    await field(page, '简称').fill('验收');
    const orgResponse = page.waitForResponse(r => new URL(r.url()).pathname === '/system/org/save');
    await page.getByRole('button', { name: /保.*存/ }).click();
    const orgBody = await (await orgResponse).json();
    assert.equal(orgBody.code, 0, orgBody.msg);
    await field(page, '名称').fill('验收组织修改');
    const orgUpdated = page.waitForResponse(r => new URL(r.url()).pathname === '/system/org/save');
    await page.getByRole('button', { name: /保.*存/ }).click();
    assert.equal((await (await orgUpdated).json()).data, orgBody.data, 'saving an organization twice must update the same row');
    checked.push(step);

    step = 'position create and edit';
    await go('/basic/positions');
    await page.getByText('岗位列表', { exact: true }).waitFor();
    await page.waitForFunction(() => !document.querySelector('[class*="-leave-active"]'));
    await page.getByRole('button', { name: /新\s*增/, exact: true }).click();
    await field(drawer(), '名称').fill('验收岗位');
    await field(drawer(), '所属机构').click();
    await page.locator('.ant-select-dropdown:visible').getByText('验收组织修改', { exact: true }).click();
    const positionId = await save(drawer(), '/system/position/save');
    await actions('验收岗位').first().click();
    await field(drawer(), '名称').fill('验收岗位修改');
    await save(drawer(), '/system/position/save');
    assert.ok((await api('/system/position/query')).items.some(x => x.id === positionId && x.name === '验收岗位修改'));
    const positionStatus = page.waitForResponse(r => new URL(r.url()).pathname === '/system/position/setStatus');
    await row('验收岗位修改').getByRole('switch').click();
    assert.equal((await (await positionStatus).json()).code, 0);
    await actions('验收岗位修改').last().click();
    const positionDeleted = page.waitForResponse(r => new URL(r.url()).pathname === '/system/position/delete');
    await page.locator('.ant-popover:visible').getByRole('button', { name: /确.*[定认]/ }).click();
    assert.equal((await (await positionDeleted).json()).code, 0);
    assert.ok(!(await api('/system/position/query')).items.some(x => x.id === positionId));
    checked.push(step);

    step = 'role create edit disable delete';
    await go('/basic/roles');
    await page.getByText('角色列表', { exact: true }).waitFor();
    await page.waitForFunction(() => !document.querySelector('[class*="-leave-active"]'));
    await page.getByRole('button', { name: /新\s*增/, exact: true }).click();
    await field(modal(), '名称').fill('验收角色');
    const roleId = await save(modal(), '/system/role/save');
    await actions('验收角色').first().click();
    await field(modal(), '名称').fill('验收角色修改');
    await save(modal(), '/system/role/save');
    const disabled = page.waitForResponse(r => new URL(r.url()).pathname === '/system/role/setStatus');
    await row('验收角色修改').getByRole('switch').click();
    assert.equal((await (await disabled).json()).code, 0);
    await actions('验收角色修改').last().click();
    const deleted = page.waitForResponse(r => new URL(r.url()).pathname === '/system/role/delete');
    await page.locator('.ant-popover:visible').getByRole('button', { name: /确.*[定认]/ }).click();
    assert.equal((await (await deleted).json()).code, 0);
    assert.ok(!(await api('/system/role/query')).items.some(x => x.id === roleId));
    checked.push(step);

    await api('/system/user/logout');
    assert.deepEqual(failedAPI, []);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ result: 'PASS', checked, pageErrors: errors, failedAPI }));
  } catch (error) {
    await page.screenshot({ path: '.local/foundation-crud-failure.png', fullPage: true });
    fs.writeFileSync('.local/foundation-crud-failure.html', await page.content());
    console.error(JSON.stringify({ step, checked, errors, failedAPI, optionRequests }));
    throw error;
  } finally { await browser.close(); }
})().catch(e => { console.error(e.message); process.exit(1); });
