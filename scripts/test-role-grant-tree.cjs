#!/usr/bin/env node
// Exercises the real role state loader and Ant Tree rendering without a browser or database.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '..');
const adminRequire = createRequire(path.join(root, 'web/admin/package.json'));
const vue = adminRequire('vue');
const { renderToString } = adminRequire('@vue/server-renderer');
const { Tree } = adminRequire('ant-design-vue');
const { parse } = adminRequire('@vue/compiler-sfc');
const { baseParse } = adminRequire('@vue/compiler-dom');
const esbuild = adminRequire('esbuild');
const file = path.join(root, 'web/admin/src/views/basic/system/role/resources.vue');
const { descriptor } = parse(fs.readFileSync(file, 'utf8'));
const granted = [10, 11, 12];
const allIds = [10, 11, 12, 13, 14, 15];
const fixtureTree = [{ key: 10, id: 10, title: '企业管理', children: [
  { key: 11, id: 11, title: '首页' }, { key: 12, id: 12, title: '组织' },
  { key: 13, id: 13, title: '成员' }, { key: 14, id: 14, title: '岗位' },
  { key: 15, id: 15, title: '角色' },
] }];
let loading;
const emitted = [];
const moduleRef = { exports: {} };
const dependencies = {
  vue: { ...vue, watch: (_source, callback) => { loading = callback(); } },
  'ant-design-vue': adminRequire('ant-design-vue'),
  '/@/components/Container': { CollapseContainer: {} },
  '/@/components/Tree': { BasicTree: {} },
  '/@/api/application/application': { getTenantResources: async () => ({ 2: { id: 2, ids: allIds, resources: fixtureTree } }) },
  '/@/api/tenant/role': { queryRoleResources: async () => ({ resource_ids: { 2: granted }, resource_map: { 2: { 10: 1, 11: 5, 12: 2 } } }) },
  '/@/constants/employ': { scopeOptions: {} },
  '/@/store/modules/user': { useUserStore: () => ({ getSaasConf: { tenant_id: 1 } }) },
  '/@/utils/helper/treeHelper': { getChildrenIds: () => [] },
};
const compiled = esbuild.transformSync(descriptor.script.content, { loader: 'ts', format: 'cjs' }).code;
vm.runInNewContext(compiled, { exports: moduleRef.exports, module: moduleRef, require: (id) => {
  assert.ok(id in dependencies, `Unmapped component dependency: ${id}`);
  return dependencies[id];
} }, { filename: file });
const component = moduleRef.exports.default;
const state = component.setup({ roleId: 3, tenantId: 1 }, { emit: (...event) => emitted.push(event) });
function findTree(node) {
  if (node.tag === 'BasicTree') return node;
  for (const child of node.children || []) { const result = findTree(child); if (result) return result; }
}
const treeNode = findTree(baseParse(descriptor.template.content));
const strict = treeNode.props.find((prop) => prop.name === 'checkStrictly');
const lock = treeNode.props.find((prop) => prop.name === 'bind' && prop.arg?.content === 'allowCheckStrictlyChange');
async function checkedCount(checkStrictly, checkedKeys) {
  const html = await renderToString(vue.createSSRApp({ render: () => vue.h(Tree, {
    checkable: true, checkStrictly, checkedKeys, defaultExpandAll: true, treeData: fixtureTree,
  }) }));
  return (html.match(/class="ant-tree-checkbox ant-tree-checkbox-checked"/g) || []).length;
}
(async () => {
  await loading;
  const saved = () => Array.from(state.roleState.value[2].checkedList);
  assert.deepEqual(saved(), granted, 'Loading a role preserves its exact grants');
  assert.equal(await checkedCount(false, saved()), allIds.length, 'Regression fixture reproduces cascading parent expansion');
  assert.ok(strict, 'Role resource Tree must use independent checks');
  assert.equal(lock?.exp?.content, 'false', 'Role toolbar cannot re-enable cascading grants');
  assert.equal(await checkedCount(true, saved()), granted.length, 'Only the three stored grants appear checked');
  state.handleCheck({ checked: granted, halfChecked: [] }, null, 2);
  assert.deepEqual(saved(), granted, 'Saving unchanged state cannot grant descendants');
  assert.equal(state.roleState.value[2].scope[12], 2, 'Existing data scope survives a no-op');
  state.handleCheck({ checked: [10, 11, 12, 13], halfChecked: [] }, null, 2);
  assert.deepEqual(saved(), [10, 11, 12, 13], 'Explicitly selecting one resource adds only that resource');
  assert.equal(state.roleState.value[2].scope[13], 5, 'New grant defaults to personal data');
  state.onCheckAllChange({ target: { value: 2, checked: true } });
  assert.deepEqual(saved(), allIds, 'Explicit app-level select all remains available');
  assert.equal(await checkedCount(true, saved()), allIds.length);
  state.onCheckAllChange({ target: { value: 2, checked: false } });
  assert.deepEqual(saved(), [], 'Explicit clear all remains available');
  const appFile = path.join(root, 'web/admin/src/views/system/application/auth/resources.vue');
  const appDescriptor = parse(fs.readFileSync(appFile, 'utf8')).descriptor;
  const appTree = findTree(baseParse(appDescriptor.template.content));
  assert.ok(appTree.props.find((prop) => prop.name === 'checkStrictly'), 'Tenant application grants must use independent checks');
  assert.equal(appTree.props.find((prop) => prop.name === 'bind' && prop.arg?.content === 'allowCheckStrictlyChange')?.exp?.content, 'false');
  const activeTree = fixtureTree.map((node) => ({ ...node, status: 1, children: node.children.map((child) => ({ ...child, status: 1 })) }));
  const expiry = 2000000000;
  const appDependencies = {
    ...dependencies,
    dayjs: adminRequire('dayjs'),
    '/@/api/application/application': { getGlobalResource: async () => ({
      1: { id: 1, code: 'system', status: 1, resources: activeTree },
      2: { id: 2, code: 'basic', status: 1, resources: activeTree },
      3: { id: 3, code: 'disabled', status: 0, resources: activeTree },
    }) },
    '/@/api/application/authorize': { getList: async () => ({ items: [{ id: 42, app_id: 2, resource_ids: granted, expiration_time: expiry }] }) },
  };
  const appModule = { exports: {} };
  vm.runInNewContext(esbuild.transformSync(appDescriptor.script.content, { loader: 'ts', format: 'cjs' }).code,
    { exports: appModule.exports, module: appModule, require: (id) => {
      assert.ok(id in appDependencies, `Unmapped app component dependency: ${id}`);
      return appDependencies[id];
    } }, { filename: appFile });
  const app = appModule.exports.default.setup({ tenantId: 7 }, { emit: () => {} });
  await loading;
  assert.deepEqual(Object.keys(app.appResources.value), ['2'], 'System and disabled apps remain excluded');
  const appSaved = () => Array.from(app.appState.value[2].checkedList);
  assert.deepEqual(appSaved(), granted, 'Loading app grants preserves the exact resource set');
  assert.equal(await checkedCount(true, appSaved()), granted.length, 'App parent grant does not select descendants');
  app.handleCheck({ checked: granted, halfChecked: [] }, null, 2);
  assert.deepEqual(appSaved(), granted, 'Unchanged app grants cannot expand on save');
  assert.equal(app.appState.value[2].ttl, expiry, 'Existing app expiration is preserved');
  assert.equal(app.appState.value[2].hasTTL, true);
  app.onCheckAllChange({ target: { value: 2, checked: true } });
  assert.deepEqual(appSaved(), allIds, 'Only explicit select all grants all app resources');
  assert.equal(app.appState.value[2].ttl, expiry);
  app.onCheckAllChange({ target: { value: 2, checked: false } });
  assert.deepEqual(appSaved(), []);
  const tenantRoleWrapper = fs.readFileSync(path.join(root, 'web/admin/src/views/system/tenant/detail/components/roles/resources.vue'), 'utf8');
  assert.ok(tenantRoleWrapper.includes("from '/@/views/basic/system/role/resources.vue'"), 'Tenant detail roles reuse the protected role grant tree');
  console.log('PASS: exact role and tenant-app grants, non-cascading rendering, preserved scope/TTL, explicit selection and select/clear all');
})().catch((error) => { console.error(error); process.exitCode = 1; });
