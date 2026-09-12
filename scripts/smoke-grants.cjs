// Run only through test-browser-crud.py: all browser API traffic targets its disposable server.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.KERTHUS_PLAYWRIGHT_PATH || 'playwright');

(async () => {
  const config = JSON.parse(fs.readFileSync(process.env.KERTHUS_ACCEPTANCE_CONFIG, 'utf8'));
  const browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 1200 } });
  page.setDefaultTimeout(15000);
  const errors = [], failedAPI = [], checked = [], submissions = [];
  let token, step = 'login';
  page.on('pageerror', error => errors.push(error.message));
  await page.route('http://127.0.0.1:18080/**', async route => {
    const request = route.request(), target = new URL(request.url());
    const response = await route.fetch({ url: config.base + target.pathname + target.search });
    const body = await response.json();
    if (body.code) failedAPI.push({ path: target.pathname, code: body.code, msg: body.msg });
    if (target.pathname === '/system/user/login' && body.code === 0) token = body.data;
    if (request.method() === 'POST') submissions.push({ path: target.pathname, data: request.postDataJSON() });
    await route.fulfill({ response });
  });
  const base = process.env.KERTHUS_ADMIN_URL || 'http://127.0.0.1:15173';
  const go = path => page.goto(base + '/#' + path);
  const api = async (path, data) => {
    const response = await fetch(config.base + path, {
      method: data === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', 'access-token': token.access_token,
        'tenant-id': '1', 'app-id': '1', 'unit-id': '0', 'section-id': '0' },
      body: data === undefined ? undefined : JSON.stringify(data),
    });
    const body = await response.json();
    assert.equal(body.code, 0, `${path}: ${body.msg}`);
    return body.data;
  };
  const row = name => page.locator('tr.ant-table-row').filter({ hasText: name });
  const appPanel = () => page.locator('.app-resources').filter({ hasText: '企业管理' });
  const node = name => appPanel().locator('.ant-tree-treenode').filter({ hasText: name });
  const save = async endpoint => {
    const response = page.waitForResponse(r => new URL(r.url()).pathname === endpoint && r.request().method() === 'POST');
    await page.getByRole('button', { name: /保\s*存/, exact: true }).click();
    const body = await (await response).json();
    assert.equal(body.code, 0, body.msg);
  };
  const toolbar = async text => {
    await appPanel().locator('.ant-dropdown-trigger').last().hover();
    await page.getByRole('menuitem', { name: text, exact: true }).click();
  };
  const flatten = nodes => nodes.flatMap(node => [node, ...flatten(node.children || [])]);
  try {
    await go('/login');
    await page.locator('input').first().fill(config.phone);
    await page.locator('input[type=password]').fill(config.password);
    await page.locator('button.ant-btn-primary').first().click();
    await page.waitForURL('**/#/system/dashboard');
    step = 'create isolated grant fixtures';
    const tenantId = await api('/system/tenant/save', {
      name: '授权验收租户', contact_person: '授权管理员', contact_phone: '13900007777',
      contact_email: 'grants@example.invalid', expiration_time: 0,
    });
    await api('/system/tenant/approve', { id: tenantId, verify_status: 1, admin_password: config.password });
    const resources = await api('/system/app/getGlobalResource');
    const basic = Object.values(resources).find(app => app.code === 'basic');
    const leaves = flatten(basic.resources).filter(resource => resource.type === 'action' && !resource.children?.length);
    assert.ok(leaves.length >= 2);
    const [first, second] = leaves;
    const roleId = await api('/system/role/save', { tenant_id: tenantId, name: '范围验收角色', status: 1 });
    const otherRoleId = await api('/system/role/save', { tenant_id: tenantId, name: '另一个验收角色', status: 1 });
    await api('/system/role/authRoleResource', { tenant_id: tenantId, role_id: roleId,
      resource_map: { [basic.id]: { ids: [first.id], scope: { [first.id]: 2 } } } });
    await api('/system/role/authRoleResource', { tenant_id: tenantId, role_id: otherRoleId,
      resource_map: { [basic.id]: { ids: [second.id], scope: { [second.id]: 4 } } } });
    const expiry = Math.floor(Date.now() / 1000) + 200 * 86400;
    await api('/system/app/authorizeApp', { tenant_ids: [tenantId], resource_map: { [basic.id]: { ids: basic.ids, ttl: expiry } } });
    const disabledId = await api('/system/app/save', { code: 'disabled-fixture', name: '停用验收应用', version: '1.0.0' });
    await api('/system/app/saveResource', { app_id: disabledId, code: 'disabled-fixture:read', name: '停用应用资源', type: 'action', status: 1 });

    step = 'role selection restores checked resources and data scope';
    await go('/system/tenant/detail/' + tenantId);
    await page.getByRole('tab', { name: '角色', exact: true }).click();
    await row('范围验收角色').locator('input[type=radio]').check();
    await page.getByRole('button', { name: /保\s*存/, exact: true }).waitFor();
    await toolbar('展开全部');
    await node(first.name).locator('.ant-tree-checkbox-checked').waitFor();
    assert.match(await node(first.name).innerText(), /本单位/);
    checked.push(step);

    step = 'role user-tab round trip preserves resource checks';
    await page.getByRole('tab', { name: '用户', exact: true }).click();
    await page.getByRole('tab', { name: '应用资源', exact: true }).click();
    await toolbar('展开全部');
    await node(first.name).locator('.ant-tree-checkbox-checked').waitFor();
    assert.match(await node(first.name).innerText(), /本单位/);
    await row('另一个验收角色').locator('input[type=radio]').check();
    await toolbar('展开全部');
    await node(second.name).locator('.ant-tree-checkbox-checked').waitFor();
    assert.match(await node(second.name).innerText(), /本部门/);
    await row('范围验收角色').locator('input[type=radio]').check();
    await toolbar('展开全部');
    await node(first.name).locator('.ant-tree-checkbox-checked').waitFor();
    checked.push(step);

    step = 'checking a new resource preserves existing nonzero scope';
    await node(second.name).locator('.ant-tree-checkbox').click();
    await save('/system/role/authRoleResource');
    let role = await api('/system/role/queryRoleResources?tenant_id=' + tenantId + '&role_id=' + roleId);
    assert.equal(role.resource_map[basic.id][first.id], 2);
    assert.equal(role.resource_map[basic.id][second.id], 5);
    checked.push(step);

    step = 'tree toolbar select-all persists the selected resource set';
    await toolbar('选择全部');
    await save('/system/role/authRoleResource');
    role = await api('/system/role/queryRoleResources?tenant_id=' + tenantId + '&role_id=' + roleId);
    assert.deepEqual([...role.resource_ids[basic.id]].sort((a,b) => a-b), [...basic.ids].sort((a,b) => a-b));
    assert.equal(role.resource_map[basic.id][first.id], 2);
    checked.push(step);

    step = 'tenant application form excludes platform and disabled apps';
    await go('/system/application/auth');
    await row('授权验收租户').locator('input[type=radio]').check();
    await appPanel().getByRole('switch').waitFor();
    assert.equal(await page.locator('.app-resources').count(), 1);
    assert.equal(await page.locator('.app-resources').filter({ hasText: '停用验收应用' }).count(), 0);
    assert.equal(await appPanel().getByRole('switch').getAttribute('aria-checked'), 'true');
    checked.push(step);

    step = 'saving tenant application resources preserves original expiry';
    await save('/system/app/authorizeApp');
    const request = submissions.filter(item => item.path === '/system/app/authorizeApp').at(-1).data;
    assert.deepEqual(Object.keys(request.resource_map), [String(basic.id)]);
    assert.equal(Number(request.resource_map[basic.id].ttl), expiry);
    const grants = await api('/system/app/queryTeantAuthorizes?tenant_id=' + tenantId);
    assert.equal(grants.items.find(grant => grant.app_id === basic.id).expiration_time, expiry);
    checked.push(step);

    await api('/system/user/logout');
    assert.deepEqual(failedAPI, []);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ result: 'PASS', checked, pageErrors: errors, failedAPI }));
  } catch (error) {
    await page.screenshot({ path: '.local/grants-failure.png', fullPage: true });
    fs.writeFileSync('.local/grants-failure.html', await page.content());
    console.error(JSON.stringify({ step, checked, errors, failedAPI }));
    throw error;
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exit(1); });
