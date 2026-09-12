# SaaS 底座前端原项目对照复查（2026-09-11）

原项目：`/Users/alex/html/dating.saas/src`。当前：`/Users/alex/golang/src/kerthus/web/admin/src`。
本报告是静态源码和接口契约对照，不将静态检查标作浏览器验收通过。复查期间未操作浏览器、未读取凭据、未修改真实数据。

## 已核对范围

| 范围 | 核对内容 | 结论 |
| --- | --- | --- |
| 基础应用 | 应用列表/详情、组织树、成员列表/详情/状态/默认应用、岗位、角色信息/人员关联/资源授权入口、审计日志 | 基础结构复用；本轮补表单异步初始化顺序。权限树和确认框交互由主任务继续核对，未修改 |
| 平台系统 | 租户列表/筛选/新增/审核入口、详情信息/成员/组织/岗位/角色/应用；全局账号；应用 CRUD/状态/资源 CRUD/API 关联/授权/撤权 | 本轮修撤权标识、批量勾选和应用元数据；筛选及资源 redirect/remark 契约交主任务补齐 |
| 公共功能 | 登录、会话 API/store、个人资料/头像/邮箱/密码、安全设置、锁屏、工作台应用卡片、顶栏切换应用/帮助/通知、动态导航 | 改密与锁屏弹窗补每次打开清空；使用当前授权 home 的导航逻辑已存在 |
| API 与路由 | `api/basic`、`api/tenant`、`api/application`、`api/user`、`api/common` 的原新差异，router/store 的保留与删除项，导入目录与基础 manifest | 业务接口删减未误计为底座缺失；`queryTenantApps` 与授权列表接口的 ID 语义不同，已分别处理 |

文件清单对照：旧 `views/basic` 36 文件、当前 24；旧 `views/system` 54、当前 49；旧个人中心 7、当前 5。主要减少来自旧两个仪表盘转为公共工作台、呼叫中心移出底座、无后端的个人绑定与通知设置移除。上述数量仅作目录清单，不代表逐项运行通过。

## 本轮发现并修复

1. **P1 租户详情撤权把应用 ID 当开通记录 ID。** 路径：平台管理 → 租户详情 → 应用 → 取消授权。原文件 `views/system/tenant/detail/components/apps/index.vue:122` 使用 `record.id`；当前同接口 `queryTenantApps` 行仍以应用为主体，撤权接口则接受 TenantApp ID，碰号可能撤掉其他开通记录。当前 [处理函数](/Users/alex/golang/src/kerthus/web/admin/src/views/system/tenant/detail/components/apps/index.vue:125) 只读 `tenant_app_id`，缺失/非正安全整数时禁用入口并提示，不回退 `id`。后端同轮补回该字段。平台授权总表使用 `queryTenantAuthorizes`，其 `id` 本就是开通 ID，维持原语义；基础应用列表只有查看行为。

2. **P1 批量撤权显示勾选和提交集合可能脱节。** 路径：租户应用授权 → 勾选记录 → 批量取消授权。原授权页 `index.vue:82` 把 `unref(checkedKeys)` 的首次空数组传入 `selectedRowKeys`，后续替换 ref；共享 Table 合并配置时会覆盖内部选择状态（[useRowSelection](/Users/alex/golang/src/kerthus/web/admin/src/components/Table/src/hooks/useRowSelection.ts:23)）。当前 [授权列表](/Users/alex/golang/src/kerthus/web/admin/src/views/system/application/authorize/index.vue:80) 以 Table 的 `onChange` 同步集合，成功撤权后清空选择，空选择禁用批量按钮。没有改授权资源树。

3. **P2 应用图标、简介、备注在迁移表单中缺失。** 原 `views/system/application/main/data.ts:116/174/179` 有这些字段；迁移表单此前仅 code/name/version。当前 [应用 schema](/Users/alex/golang/src/kerthus/web/admin/src/views/system/application/main/data.ts:103) 恢复三项，列表显示图标；[表单](/Users/alex/golang/src/kerthus/web/admin/src/views/system/application/main/form.vue:40) 将图标路径在上传组件数组与 API 字符串之间转换并回填文本。基础/租户应用只读详情及工作台原本已有这些显示字段，后端同轮补存储、导入和 DTO。未恢复无持久化支持的应用 type/public/url 编辑。

4. **P2 取消密码弹窗后重开仍保留输入。** 原个人中心 `SecureSetting.vue:45` 无参数 `openModal()`；共享 [useModal](/Users/alex/golang/src/kerthus/web/admin/src/components/Modal/src/hooks/useModal.ts:78) 没有 data 就不触发 inner callback，导致 Password.vue 已有 reset 不执行。当前 [调用](/Users/alex/golang/src/kerthus/web/admin/src/views/common/profile/SecureSetting.vue:60) 传空对象触发每次打开清空。锁屏有同类遗留，当前 [LockModal](/Users/alex/golang/src/kerthus/web/admin/src/layouts/default/header/components/lock/LockModal.vue:50) 增加打开清空，顶栏 [调用](/Users/alex/golang/src/kerthus/web/admin/src/layouts/default/header/components/user-dropdown/index.vue:91) 同样传 data；不改锁屏认证逻辑。

5. **P2 四个旧表单的异步清空与回填未串行。** 路径：岗位/基础角色/租户详情角色/租户表单，连续打开新增或编辑。共享 resetFields 是异步（[定义](/Users/alex/golang/src/kerthus/web/admin/src/components/Form/src/hooks/useFormEvents.ts:40)），旧代码不 await 就回填，存在新值被稍后 reset 覆盖的顺序风险。当前分别在 [岗位](/Users/alex/golang/src/kerthus/web/admin/src/views/basic/user/position/form.vue:35)、[基础角色](/Users/alex/golang/src/kerthus/web/admin/src/views/basic/system/role/form.vue:28)、[租户角色](/Users/alex/golang/src/kerthus/web/admin/src/views/system/tenant/detail/components/roles/form.vue:36)、[租户表单](/Users/alex/golang/src/kerthus/web/admin/src/views/system/tenant/main/form.vue:34) 串行 await reset/set。此项属于代码顺序修正，未宣称每个页面已有浏览器复现。

## 同轮接口差异，由主任务或后端代理补齐

- 应用列表 `code`、授权列表 `tenant_name`/`app_name` 有搜索字段但原迁移 queryContext 未读取。已反馈主任务；报告撰写时 gateway 已出现对应解析，最终行为以主任务测试为准。
- 资源表单有 redirect，原迁移 DTO/domain 未保存；旧资源 remark 也应完整保留。主任务负责模型、导入、网关和前端字段。
- App 的 icon/desc/remark、`queryTenantApps.tenant_app_id`/expiration_time，以及 AvailableApps 分页、有效期、有效岗位列表由后端代理负责。前端已按其确认的字段接入，仍需集成回归。

## 明确边界与未实现项

- 旧呼叫中心 `basic/rcc`、CRM、婚恋等业务应用不属于本次底座复用范围。导入器保留其数据并禁用执行，不把移除业务页算缺陷。
- 旧 `system/log/main` 是用户表复制的占位代码，新增/编辑无抽屉、删除仅 console.log。导入器在 [catalog.go](/Users/alex/golang/src/kerthus/internal/platform/legacy/catalog.go:89) 明确禁用 `system:log*`；当前真实审计页为 `basic/system/audit/index.vue`。不恢复旧假 CRUD。
- 个人中心旧 AccountBind/MsgNotify、短信/扫码/第三方登录、通知/消息/待办没有本次后端契约。顶栏通知目前读取空静态数组；应描述为未接入，不能宣称完整消息系统。旧示例通知备份数组不参与显示。
- 应用 type/public/url、资源 inside/outside 的完整编辑与执行尚未实现。当前资源编辑仅组件路由，并显示说明；不支持的旧路由形态应保留/禁用并说明，不能悄悄转成组件。
- 新应用要先注册前后端 manifest 与 HTTP 契约，才能出现可关联接口。不能任意用已有应用接口冒充新应用契约。
- 旧请求日志载荷、额外用户扩展信息、未建模字典的迁移范围由后端报告说明；前端没有据此虚构编辑入口。

## 验证

- 本轮 11 个前端修改文件已定点 Prettier。
- `cd web/admin && yarn type:check` 通过，耗时 10.96 秒。
- 此处未运行浏览器或写入测试数据；危险撤权、图标上传和保存后的回显需由主任务在隔离验收库继续回归。
- 未重复生产 build，留给同轮所有前后端修改收敛后的主任务统一检查。
