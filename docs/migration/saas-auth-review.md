# SaaS 认证与授权迁移复核

日期：2026-09-11。旧源码：`/Users/alex/wwwroot/service.beehive` 当前工作树。仅讨论 SaaS 基础底座；CRM、Noctua 仅作为通用数据范围被消费的证据，不迁移其业务。只读检查源码与配置，未启动服务、访问数据库、读取环境密钥或修改旧项目。源项目、目标项目及其父目录未找到适用 `AGENTS.md`。以下明确区分源码事实与 Go 实现建议。

## 1. 结论与边界

保留全局账号、租户成员、组织岗位、应用开通、角色资源/API 权限、会话和审计这些领域概念，并保留旧前端需要的 HTTP 数据形状。重新实现服务端身份与租户信任链，不能把旧中间件逐行翻译为 Go。

初期建议 HTTP gateway + 一个 go-micro SaaS 核心服务，内部划分 Identity、Tenant、Organization、Application、Authorization 模块，先保持同库授权变更事务。这里是逻辑边界建议，不要求每个模块独立进程，也不依赖未经核实的 go-micro 插件 API。

数据权限不是纯 CRM：System 提供策略存储、组织范围计算；CRM 大量消费，Noctua 也消费。首批底座保留范围枚举、组织树查询与策略判定扩展点，业务记录的 owner 字段、客户转移、公海、合同等过滤与写入规则全部留在业务模块。

## 2. 要保留的外部契约

| 契约 | 保留方式与边界 | 源码证据 |
| --- | --- | --- |
| 登录手机号/邮箱场景与 opaque token | 保留 `scene/username/password` 输入、`user_id/access_token/expires_time/tenant_id/app_id/unit_id/section_id/app_code` 输出。token 无需变成 JWT；更换随机生成与存储实现不影响前端 | [FormUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:100) |
| `access-token`、`tenant-id`、`app-id`、组织上下文请求头 | gateway 接受旧名称；tenant/app/org 只是用户选择，必须服务端验证后形成上下文 | [AppMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:61)、[TokenMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/TokenMiddleware.php:42) |
| `/system/user/auth` 的角色、菜单、动态路由、权限码与租户信息 | 保留 `roles/menus/routes/permissions/tenant`，树保留 `name/icon/path/component/is_public/open_with/redirect/meta/children`；修正为返回当前已验证上下文，而非无条件默认租户 | [ServiceSystemUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:151)、[User.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Logic/User.php:80) |
| 角色资源及范围编辑 | 保留 resource map、`data_access` 0–5 编码供现有管理页显示；无权授予的资源必须拒绝，不照单写入 | [Role.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Logic/Role.php:20) |
| 统一 envelope | 保留 `{code,msg,mesc,data}` 与现有错误映射，防止前端请求层误判；内部错误使用明确种类 | [AppMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:97) |

兼容前端不等于保留越权行为。涉及全局账号、员工编辑、默认应用切换的旧接口仍可保留 URL，但输入白名单与目标对象授权必须收紧；对应前端可能需要少量表单调整。

## 3. 已核实的旧行为与必须修正点

### 3.1 身份、租户与平台权限

- `tenant-id=1` 直接设置管理员标记，AccessMiddleware 遇到该标记跳过 API 授权；TraitApi 又允许输入覆盖 tenant/user/scopes。这条信任链完全来自请求头，不代表真实平台管理员资格。[AppMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:70)、[AccessMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/AccessMiddleware.php:49)、[TraitApi.php](/Users/alex/wwwroot/service.beehive/app/Traits/TraitApi.php:17)
- TokenMiddleware 只取缓存的 user_id、expires_time 和 token，未使用缓存中的 tenant/app/org 校验当前请求头；AuthMiddleware 仅判断 user_id 非零。[TokenMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/TokenMiddleware.php:47)、[AuthMiddleware.php](/Users/alex/wwwroot/service.beehive/app/Middleware/AuthMiddleware.php:40)
- 登录检查全局账号 status、默认租户状态/审核/删除/到期，但默认成员查询漏 `tenant_employee.is_del=0`。成员模型目前没有独立停用状态，只有 `is_del`；“成员停用”是 Go 新模型需明确的状态，不能误称旧字段已实现。[FormUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:102)、[DaoTenantEmployee.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Dao/DaoTenantEmployee.php:44)、[TenantEmployee.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/TenantEmployee.php:10)
- 租户应用 `expiration_time` 在查询中返回，但上述登录与 API 授权路径未执行应用到期校验；必须在上下文解析与实际服务操作中生效。[DaoTenantApp.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Dao/DaoTenantApp.php:39)
- `getAuth` 的菜单来自租户资源与角色资源的过滤组合，API 列表直接来自角色资源，没有与租户开通资源求交；`getAuthApis` 同样只取角色资源。取消租户开通不能只隐藏菜单，必须同步禁止 API。[ServiceSystemUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:153)
- 部分个人资料/密码动作没有 AuthMiddleware，默认配置也未全局启用它；Go 应默认认证、显式声明匿名登录等例外。[UserController.php](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/UserController.php:91)、[middlewares.php](/Users/alex/wwwroot/service.beehive/config/autoload/middlewares.php:21)

### 3.2 全局账号与租户成员不能混写

`saveEmployee` 在成员事务之前调用全局 `User.saveData`；现存用户仅排除 phone/password 更新，其他可写字段包含 email、status、is_del、is_signal_login、姓名与头像。因此有员工编辑权限不应自动获得多租户共享账号的登录邮箱、全局启停与删除权限。[ServiceSystemTenantEmloyee.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemTenantEmloyee.php:50)、[ServiceSystemUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:66)、[User.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/User.php:33)

Go 应分开命令：本人账号资料/凭据修改、平台账号管理、租户成员资料/组织岗位管理。租户成员编辑必须先确认 `(tenant_id, user_id)` 关系，再更新租户归属字段；已有共享账号只建立/恢复成员关系，不因租户管理员提交 `id` 就覆盖全局身份。新增账号与成员若同库，使用一个事务；登录标识绑定/变更有独立验证流程。姓名/头像等是否采用租户展示覆盖值可以后续决定，但不能开放全部账号字段批量赋值。

员工 info 当前按全局 user_id 取资料，再取该用户默认租户应用；没有先确认其属于当前租户。Go DTO 必须把 actor 与 target_user_id 分开，详情、编辑、删除、绑定角色都验证目标关系。[EmployeeController.php](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/EmployeeController.php:122)

### 3.3 权限缓存与会话撤销

旧缓存存在多处互相遮蔽的缺陷，不能简单写成“旧权限一直从缓存读取”：

1. `getAuthApis` 读取 `$resource_api` 后判断未定义的 `$user_apis`，按当前 PHP 语义进入重算分支，所以当前请求事实上每次重新取角色/API。[ServiceSystemUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:205)
2. 权限 hash 用 HMSET 合并，未删除本次已撤销的字段；只有 `!$ttls` 才设置 expireAt，Redis 缺 key 时 TTL 是负数而非 0，因此新建 key 的过期设置存在缺陷。修复第 1 点却保留这里，会重新暴露旧授权缓存问题。[CachePermitApi.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Cache/CachePermitApi.php:27)
3. `clearUserApis` 仅在 logout 路径调用；角色绑定、角色资源修改与删除没有统一失效链，清理还使用全 key 扫描。[CachePermitApi.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Cache/CachePermitApi.php:63)、[ServiceSystemTenantRole.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemTenantRole.php:100)
4. token user 索引只保存最后一个 token，而账号可允许多登录。logout/账号停用/租户停用清理该指针对应 token，不能保证所有历史会话撤销；成员删除不撤销会话或清理角色绑定。密码修改/重置也未在服务中撤销会话。[CacheUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Cache/CacheUser.php:40)、[ServiceSystemTenantEmloyee.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemTenantEmloyee.php:91)、[ServiceSystemUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:100)

建议初版以服务端权威状态判定为基准，可暂不缓存计算结果。引入缓存时按 user/tenant/app 与 account、membership、entitlement、role policy、org 版本标识快照；授权变更与版本递增同事务完成，授权前从权威状态验证版本。事件用于主动淘汰，不能把异步事件作为唯一撤权保障。删除/重建完整缓存值、设置有限 TTL、负授权可缓存但有版本、后端不可用时拒绝受保护操作。

会话明确 current-session logout 与 all-sessions revoke：旧 logout URL 可以注销调用它的会话；改密、全局停用应撤销所有会话。租户成员停用只阻断该租户能力，仍允许用户进入其他有效租户；租户停用不需要注销该用户其他租户身份。单点登录需原子替换会话，而不是非事务的先删后写。密码哈希采用可升级版本存储，旧哈希只用于过渡验证，验证成功升级；token 使用安全随机源，日志不写凭据/token。旧生成位置见 [FormUser.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:78)。

## 4. Go 服务端上下文与不变量

HTTP gateway 把外部 token 与 tenant/app/org 选择发给 SaaS 核心解析。核心生成含 actor_user_id、session_id、actor_kind、tenant_id、membership_id、app_id、有效 org 选择、策略版本、request_id 的请求级可信上下文；它不能由请求 body、query 或未经验证的 RPC metadata 覆盖。

初期可以在同一核心进程中完成解析、授权与操作；若 gateway 带上下文走 RPC，核心仍需验证其来源、会话/策略当前有效性与目标资源归属。服务发现地址和任意 `x-user-id` metadata 都不构成可信身份。服务间连接鉴权、目标 service/method 授权与用户操作授权是三层不同检查；后台服务身份使用明确的 service principal，不能构造一个假的用户或 tenant-id=1。

必须保持以下不变量：

1. 用户操作成立前，会话/账号有效；租户操作额外要求租户有效、当前成员有效、应用已启用且租户开通有效、角色有效、操作被授权。到期采用统一服务端时间，建议 `now >= expires_at` 即失效，0 为不过期的历史语义须经数据确认。
2. 有效资源是“应用可用资源 ∩ 租户开通资源 ∩ 有效角色授权资源”。菜单可补父节点用于导航，但补父节点不能自动授予父节点挂载的业务 API。
3. API 权限以明确的 action 或 `(HTTP method, normalized route)` 区分。同一 URI 多方法不能用单个 map 值覆盖。旧实现按 URI pluck 单 method，数据范围反查资源也仅按 URI。[DaoAppResourceApi.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Dao/DaoAppResourceApi.php:43)
4. 租户对象的详情/更新/删除/批量关系写入一律包含 tenant_id。actor 与 target 分开；资源 id、role id、org id、member id 的租户和应用归属必须一起验证。无数据范围限制仍然有租户限制。
5. 平台管理员由独立的服务端平台角色/能力授予；租户管理员仅管理自己的租户。平台跨租户操作必须进入明确的 platform command，传 target_tenant_id，记录 actor 与 target，不能通过更换“当前租户”偷偷扩权。开通/停用/审核租户属于平台控制面，普通租户角色不能授予这些能力。
6. 角色资源写入不能超出租户 entitlement，也不能经租户角色配置产生平台权限；全局应用资源目录的修改属于平台操作。租户管理员、普通成员、平台人员需要各自种子权限，不使用固定租户主键推导特权。
7. 撤权事务提交后发起的请求必须看到撤权；已经在途的写入在敏感状态变更下应在事务提交前重新验证授权版本，避免旧快照成功提交。权限初始化接口与实际操作共享判定逻辑。

## 5. 数据范围：底座扩展点，不迁移业务 DAO

范围编码：0 全租户、1 单位及下级、2 本单位、3 部门及下级、4 本部门、5 自己。[ServiceSystemTenantEmloyee.php](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemTenantEmloyee.php:157)

当前 `DataScopeInterceptor` 使用分布：System 控制器未标注；CRM 多个控制器标注；[Noctua/AppController.php](/Users/alex/wwwroot/service.beehive/app/Http/Module/Noctua/Controllers/AppController.php:179) 也标注。说明范围是底座机制，业务是否落地过滤必须由业务 handler/repository 实现，不能把“有注解”当作数据已经安全过滤的证据。

需改进的语义：

- `ScopeAll` 返回空数组，而无组织/无匹配成员也可能产生空数组；Go 用 `None | AllWithinTenant | Self | OrgSet` 等显式类型表达，`None` 绝不代表全部。[ScopeAll.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Scopes/ScopeAll.php:20)
- 旧多角色策略按 data_access 降序取一个数值，偏向较窄枚举；但跨多组织的单位/部门范围不是一个可靠的数字全序。Go 建议对已授权角色的允许范围求并集，再与租户和用户主动筛选求交；若业务要更严格的组合，作为明确策略配置。这里是建议，改变旧多角色结果，实施前需写入迁移验收规则。[DaoTenantRoleResource.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Dao/DaoTenantRoleResource.php:67)
- 旧 data_scope 用数值大小判断能否收窄，Go 应对集合求交；伪造 employee_id/org_id/data_scope 只能缩小结果，不能扩大授权集合。[DataScope.php](/Users/alex/wwwroot/service.beehive/app/Aspect/DataScope.php:71)
- 无角色或找不到 API 资源的旧范围回退为“自己”；这不能作为未授权 API 的放行依据。先判断操作权限，再判断行级范围。[DataScope.php](/Users/alex/wwwroot/service.beehive/app/Aspect/DataScope.php:45)
- 组织树需支持多部门归属、空组织、跨单位节点及递归子树，并拒绝环；旧 ScopeSection 把 org_id 数组传给单 id 递归比较，不能作为正确参考算法直接迁移。[ScopeSection.php](/Users/alex/wwwroot/service.beehive/app/Services/System/Scopes/ScopeSection.php:25)

底座首批验收只需使用合成租户/组织/资源数据验证上述 policy 输出，不需要搭建 CRM 或 Noctua 表。

## 6. 可直接转为验收测试的场景

| 场景 | 预期 |
| --- | --- |
| 普通账号将 tenant-id 改为 1，或 body 注入 tenant_admin/scopes/user_id | 不能成为平台管理员；请求身份不可覆盖 |
| A 租户成员将头改为不属于自己的 B 租户；同时提交 B 的 role/org/member id | 列表、详情、更新、删除、批量关联全部拒绝，无 B 数据返回 |
| 账号属于 A/B，切到 B 并调用 auth | 返回 B 的菜单、成员信息和默认应用；不返回 A 的默认上下文 |
| 成员从 A 删除/停用，但 token 仍有效且旧角色关联残留 | A 操作拒绝；仍可切换到有效 B；旧角色不会恢复成员权限 |
| 账号全局停用/删除，已有两个设备 token | 两个会话后续操作均拒绝；不是仅撤销最近一个 token |
| 单点登录并发登录、旧会话注销 | 满足选定单会话规则；旧会话注销不能误删新会话 |
| 改密/重置密码后继续使用旧 token | 按全会话撤销规则拒绝；日志中无密码/token |
| 租户或应用开通时间从未来推进至过期瞬间 | 无需重新登录即拒绝对应操作；续期后重新判定恢复 |
| 租户取消资源开通，角色仍持有该资源 | 菜单/权限码/API 同时失效，无法凭缓存或直接 HTTP 调用继续使用 |
| 撤销角色、停用角色、移除成员角色、删除 API 映射 | 提交后的新请求全部用最新策略；缓存开启与关闭结果一致 |
| 同 URI 同时存在 GET 和 POST，不同角色分别授权 | 两种动作互不覆盖，只有获授权方法可执行 |
| 只有子菜单权限，父节点为导航补齐 | 可显示导航结构，不自动取得父节点业务 API 权限 |
| 给角色提交其他租户或未开通 app/resource | 整次命令拒绝，不部分保存、不扩大 entitlement |
| 多角色分别授予本部门与本单位、用户属于多个组织 | 结果等于明确配置的集合组合；不依赖枚举数值排序 |
| 无组织/无匹配成员、显式 None、显式 AllWithinTenant | 前两者零结果；All 只包含当前租户，绝不跨租户 |
| 客户端给出更宽 data_scope/伪造组织或 owner | 结果最多是授权集合的子集 |
| A 租户管理员编辑同时属于 B 的用户，提交全局 email/status/is_del | 拒绝越权账号字段；A 的成员资料变更不改变 B 的身份与状态 |
| 平台管理员审核/停用 B 租户；普通租户管理员重复同请求 | 平台请求经显式能力通过且审计记录 actor/target；普通管理员拒绝 |
| 绕过 gateway 直连 RPC 或伪造身份 metadata | 无有效服务身份与可信用户上下文即拒绝 |
| 撤权事件丢失、权限缓存旧版本、Redis 不可用 | 仍不允许旧权限；权威判定不可用时受保护操作失败关闭 |

实现前尚需的事实材料是脱敏 schema、平台与租户权限种子、当前应用资源/API 映射，以及实际前端切换租户/应用的调用样本。它们用于冻结数据和外部契约，不妨碍先落实上述隔离与授权设计。
