# SaaS 底座 HTTP 契约与前端复用复核

日期：2026-09-11。范围仅为 SaaS 管理底座；旧后端 `/Users/alex/wwwroot/service.beehive`，旧前端 `/Users/alex/html/dating.saas`。已复核 discovery 两份报告、API 对照表及下列实际源码。检查项目与父目录未发现适用 AGENTS.md。本轮只读旧项目、只写本报告，未启动服务、连接数据库、构建前端或实现 Go；下文数量是静态范围，不是运行通过率。未选择 go-micro 版本或插件。

## 结论与迁移边界

前端可以沿用现有 Vue 管理台和平台页面，Go 后端要优先提供兼容 HTTP 层，再调用 go-micro RPC。最容易漏掉的是：数据库中的页面组件路径和权限码、资源 API 编辑器使用的操作目录、以用户 ID 表示员工的 DTO、授权树的对象映射、分页与上传的不同解包方式。无需迁入 CRM、婚恋、呼叫中心即可完成底座闭环。

目录名不是范围标准：`basic/rcc` 属于呼叫中心；`system/tenant/user` 有真实用户查询，但新增编辑是 TODO；`common/dashboard` 夹有示例动态。因此建议以明确的平台页面/行为白名单构建种子菜单。旧仓库有既有改动，最终搬迁应固定工作树快照；本报告不修改这些改动。

## 端点范围与数量口径

建议首批 **57 个唯一 method + path**：覆盖登录资料、平台租户、应用/资源/授权、组织岗位员工、角色及数据范围、初始化/地区/文件、API 操作目录。它不是“System/App 的全部端点”，也不是全部 wrapper 数。

下表简写 `GET a,b` 表示相应前缀下多个 GET 路径。保留大小写以及已有拼写 `queryTeantAuthorizes`。

| 分组 | 数量 | 必需 HTTP 路径 |
| --- | ---: | --- |
| 账号、会话、个人资料 | 8 | `/system/user/`：POST `login,modifyInfo,modifyPassword,resetPassword`；GET `logout,profile,auth,setStatus` |
| 租户管理 | 6 | `/system/tenant/`：GET `query,info,delete,setStatus,verify`；POST `save` |
| 应用目录 | 5 | `/system/app/`：GET `query,delete,setStatus,getAppItems`；POST `save` |
| 应用资源及资源 API | 6 | `/system/app/`：GET `getGlobalResource,getResource,getAppResources,queryResourceApis,removeResource`；POST `saveResource` |
| 租户应用与资源授权 | 5 | `/system/app/`：GET `queryTeantAuthorizes,getTenantResourceIds,getTenantResources`；POST `authorizeApp,deauthorizeApp` |
| 组织 | 4 | `/system/org/`：GET `query,info,delete`；POST `save` |
| 岗位 | 5 | `/system/position/`：GET `query,delete,setStatus,getItems`；POST `save` |
| 员工及应用上下文 | 6 | `/system/employee/`：GET `query,info,getTenantApps,queryTenantApps,hasApp`；POST `save` |
| 角色、成员及数据范围 | 7 | `/system/role/`：GET `query,delete,setStatus,queryRoleResources`；POST `save,authRoleResource,relateEmployee` |
| 启动配置、地区 | 2 | GET `/app/info/init`、`/system/config/districts` |
| 资源 API 操作目录 | 2 | GET `/app/info/servers`、`/app/info/routes` |
| 文件上传 | 1 | POST `/app/file/upload`，实际前端由独立上传环境地址决定 |
| 合计 | **57** | 前 56 条在 discovery 对照表中静态匹配；上传另由组件/环境配置调用链确认 |

数量的可复核差异：此前 168 个直接 wrapper 调用里，取匹配的 `/system/` 且排除 rcc，再加 `/app/info/init,servers,routes`，是 **61 条**；另加上传得到 **62 条宽口径候选**。其中 5 条未纳入上述 57：

- `GET /system/user/query`：`system/tenant/user` 的只读列表确实调用。如保留全局账号浏览页，范围增为 **58**。其 wrapper 不接收分页/筛选参数，新增编辑 TODO、删除只打印；不能把页面当完整用户 CRUD 迁移。[用户页面](/Users/alex/html/dating.saas/src/views/system/tenant/user/index.vue:47)、[表单 TODO](/Users/alex/html/dating.saas/src/views/system/tenant/user/form.vue:47)、[wrapper](/Users/alex/html/dating.saas/src/api/user/user.ts:14)
- `GET /system/tenant/getItems,getItemsByAreaCode`：源码消费者是婚恋/CRM 租户筛选与线索派发，平台授权页面使用 tenant/query；不进入当前最小底座。[CRM 派发](/Users/alex/html/dating.saas/src/views/crm/components/dispatch/index.vue:20)
- `GET /system/employee/getItems`：消费者是 CRM/呼叫中心，角色成员选择用 employee/query。[角色成员选择](/Users/alex/html/dating.saas/src/views/basic/system/role/userRelate.vue:32)
- `GET /system/config/industries`：真实消费者在 CRM 筛选和客户表单；保留通用字典扩展边界，首批页面只需地区。[CRM 筛选](/Users/alex/html/dating.saas/src/views/crm/components/filter/data.ts:3)

不计入：`/getMenuList,/testRetry` 模板 wrapper；`/app/info/location`；`/system/employee/getSeatItems` 与全部 rcc；微信、CRM、FeelingRing；以及仅枚举未直接调用的 `/system/user/save,delete`、`/system/employee/delete`。后端独有的 `config/express`、`app/queryTenant`、`user/info` 等也不能仅因目录属于 System 就扩大首批 HTTP 兼容面。业务域内部仍可有必要的删除/离职等能力，须独立定义验收，不能当作旧前端已实现功能。

端点声明证据：[应用 API](/Users/alex/html/dating.saas/src/api/application/application.ts:3)、[授权 API](/Users/alex/html/dating.saas/src/api/application/authorize.ts:4)、[租户 API](/Users/alex/html/dating.saas/src/api/tenant/tenant.ts:3)、[员工 API](/Users/alex/html/dating.saas/src/api/basic/employee.ts:3)、[角色 API](/Users/alex/html/dating.saas/src/api/tenant/role.ts:3)、[完整静态对照表](/Users/alex/golang/src/kerthus/docs/discovery/api-contract-map.md:1)。

## HTTP 与 DTO 必须冻结的部分

| 协议面 | 已观察旧行为 | Go 迁移建议 |
| --- | --- | --- |
| envelope | 正常请求成功只认 `code === 0`，解包 `data`，提示使用 `msg`；旧后端还有 `mesc`。HTTP 401 和业务 code 401 有会话处理。[Axios](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:34) | HTTP 层统一 `{code,msg,data}`，需要时保留 `mesc`；RPC 使用明确错误类型，网关映射。空集合类型按每个接口约定，不让 Go nil 意外输出 null。 |
| 参数和分页 | GET query；普通 POST wrapper 的 params 被 Axios 搬到 JSON body。分页 `page/pageSize`，返回 `items/total`，后端另带 page；排序 `field/order`，`ascend` 被映射为升序。[请求转换](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:109)、[分页约定](/Users/alex/html/dating.saas/src/settings/componentSetting.ts:10) | 外部兼容这些字段，内部限制页长、排序字段白名单。不要复制后端 page 默认 0 导致 offset 异常的细节。 |
| 可信上下文 | 请求发送 `access-token,tenant-id,unit-id,section-id,app-id`，当前选择优先，回退登录上下文。[拦截器](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:154) | 兼容头名称；用会话身份验证所选租户、成员、应用、组织。平台管理员操作目标租户作为显式经授权目标，不能由 header tenant=1 推出管理权限。 |
| 登录/profile | `username,password,scene`；登录对象 `user_id,access_token,expires_time,tenant_id,app_id,unit_id,section_id,app_code`；TS 另声明 seat_id。profile 为 `id,phone,email,name,avatar,sex`。[模型](/Users/alex/html/dating.saas/src/api/common/model/userModel.ts:6) | 维持 snake_case、ID 类型与时间单位；seat_id 只属兼容冗余，不建设坐席域。新增可信身份字段只在内部。旧 token 形态无需作为内部实现约束。 |
| auth | `menus,routes,roles,permissions`；后端另返回 tenant。roles/permissions 是字符串数组，导航为树。[getAuth](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:151) | 返回实际被授权资源与必要父节点，不能仅菜单隐藏就认为 API 安全；授权撤销、过期或账号状态变化时同步失效会话/权限缓存。 |
| 账号维护 | 修改密码 `old_password,password`，重置密码传 `id`；员工页使用 user/setStatus 操作账号。[UserController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/UserController.php:107)、[员工列配置](/Users/alex/html/dating.saas/src/views/basic/user/main/data.ts:5) | 必须明确全局账号状态与租户成员状态的区别；租户管理员不能随意停用同一全局账号的其他租户身份。必要时仅这一按钮改接成员状态接口，不能以“兼容”复制越权语义。 |
| 租户 | 表单含 `id,logo,name,expiration_time,contact_person,contact_phone,contact_email,address_code,area_code,address,address_detail,credit_code,desc`；logo 经过上传组件/表单适配。[表单](/Users/alex/html/dating.saas/src/views/system/tenant/main/data.ts:109) | 保留页面字段、日期转换和展示结构；verify/status 不合并成一个泛化状态。真实枚举、默认值需要 schema/种子样本确认。 |
| 员工 | query item 是用户数据，含 `orgs[]/positions[]`；角色选择分支有 `has_add`。info 以 `id` 当 user_id，追加 `app_id,position_ids,org_ids`；save 返回 int。[EmployeeController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/EmployeeController.php:77) | 内部可有 membership_id，但兼容 DTO 的 id 仍按 user_id 处理，禁止混淆；org_ids/position_ids/app_id 必须属于目标租户。更新默认应用要按目标租户定位 membership。 |
| 组织、岗位 | 组织树依赖 id/key/title/children 及 `parent_id,unit_id,type(unit/section),name,short_name,status,sort,remark`；岗位 `id,org_id,name,status,remark`，列表带组织展示。[组织表单](/Users/alex/html/dating.saas/src/views/basic/user/org/data.ts:7)、[岗位表单](/Users/alex/html/dating.saas/src/views/basic/user/position/data.ts:89) | 保留树与数组/对象结构；内部验证循环、组织类型、父节点归属和成员引用。 |
| 应用授权 | authorizeApp 输入 `tenant_ids[]`、按 app_id 键控的 `resource_map`；取消用 `tenant_app_ids[]`。[AppController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/AppController.php:211) | 租户应用有效期和资源 entitlement 是底座能力；不把它推定为订阅/计费。获取全局/租户资源树需保留按 app_id 分组的 `id,name,ids,resources`。 |
| 角色授权 | query 返回 `resource_ids[app_id][]` 和 `resource_map[app_id][resource_id]=data_access`；保存格式却是 `resource_map[app_id]={ids:[],scope:{...}}`。[RoleController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/RoleController.php:113)、[前端提交](/Users/alex/html/dating.saas/src/views/basic/system/role/index.vue:68) | 两个 DTO 不要误合并。JSON 的动态 ID key 当对象处理；data_access 显式枚举。内部权限求值区分全部、空集和限定成员，事务保存后失效缓存。 |
| 角色成员 | `role_id,user_ids[],type,action`，删除为 `action=del`。[角色成员删除](/Users/alex/html/dating.saas/src/views/basic/system/role/users.vue:77) | 验证成员和角色同租户，操作幂等；不能只凭 user_id 批量跨租户关联。 |
| 初始化、地区 | App 根引导调用 init。旧 init 包含 static_url/mobile_url/user_status，以及媒体业务配置；平台图片由 static_url 拼接对象 key。地区请求 parent_id，表单用 name/id。[InfoController](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/InfoController.php:31)、[图片地址](/Users/alex/html/dating.saas/src/utils/file/resource.ts:18) | 底座初始化至少供静态资源地址与平台需要枚举；媒体配置不需要迁业务。地区字典需要版本化种子数据，不能仅生成空接口。 |
| 文件 | multipart `file,directory,param`，后端返回对象 key 字符串；uploadFile 返回原始 AxiosResponse，DefaultUpload 读取 `response.data.data`。[FileController](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/FileController.php:32)、[DefaultUpload](/Users/alex/html/dating.saas/src/components/Upload/src/DefaultUpload.vue:136) | HTTP 层仍返回 envelope 内 key，保持现有头像/logo展示链；内部负责鉴权、租户归属、路径生成和内容校验。不能让客户端 directory/param 决定真实存储权限。 |

## 动态菜单和操作目录是两个不同的兼容面

**旧导航行为。** `auth.routes` 中 `component` 经 `import.meta.glob('../../views/**/*.{vue,tsx}')` 精确匹配页面相对路径；支持 LAYOUT、IFRAME、open_with、meta、redirect、children。资源 `code` 被展开为 permissions，由 v-auth/hasPermission 用，例如 `basic:user:role:authRoleResource`。应用切换后首页拼为 `/{appCode}/dashboard`。因此迁移数据库种子时必须同时保留应用 code、菜单 path/name、component、按钮 code，前端文件路径不能随意整理。[路由解析](/Users/alex/html/dating.saas/src/router/helper/routeHelper.ts:20)、[按钮权限](/Users/alex/html/dating.saas/src/views/basic/system/role/index.vue:8)、[首页](/Users/alex/html/dating.saas/src/store/modules/user.ts:174)

**旧操作目录行为。** `/app/info/servers` 从 `server.servers` 只筛 HTTP listener，**不是旧 JSON-RPC 服务清单**。`routes?server=...&path=...` 从该 HTTP router 找有 AccessMiddleware 的 handler，按 PHP controller 分组，返回 `classes:[{name}]` 与 `actions:{[controller]:[{method:[],uri,controller,action}]}`。前端依次选 server/controller/uri，把 `method + '#' + action` 拆出后存为 `{server,controller,uri,method,action}`，随 saveResource 的 `apis[]` 一并提交。[Route](/Users/alex/wwwroot/service.beehive/app/Librarys/Route.php:30)、[操作目录输出](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/InfoController.php:68)、[前端选择器](/Users/alex/html/dating.saas/src/views/system/application/resource/data.ts:22)、[API 表单](/Users/alex/html/dating.saas/src/views/system/application/resource/apiForm.vue:36)

**Go 建议。** 建立受管理权限保护、由 HTTP 路由定义生成的稳定操作目录。保留上述 HTTP DTO，server 可用稳定 HTTP 入口名，controller/action 是兼容分组/操作标识，不需要复刻 PHP 类或映射为 Go 实际包名，也不直接返回 go-micro 注册中心内容。历史资源 API 记录中的 PHP controller/action 应按 method + URI 匹配新目录后迁移。授权内核绑定稳定操作 ID 或 method + 路由模板；RPC endpoint 名称是内部细节。

**已经确认的前端局部修补点。** API 表单以 URI 为对象 key，列表添加、删除也仅比较 URI，所以同 URI 的 GET/POST 会冲突；method[] 通过字符串拼接还会变成逗号字符串。应改为每 method 一项、以 method+URI 标识，兼容 HTTP 字段可保持。这不是重写管理台。[去重删除](/Users/alex/html/dating.saas/src/views/system/application/resource/apis.vue:80)

另一个提交风险：资源 API 表格分页默认 30，保存资源时只收集 getDataSource，再由旧 service 清空旧关联后重建。如果数据源只是当前页，就可能截断超过一页的 API 关联；这是静态调用链风险，未复现。迁移时把完整关联编辑为无分页集合或显式增删集，验收包含超过 30 条关联后保存不丢失。[分页配置](/Users/alex/html/dating.saas/src/views/system/application/resource/apis.vue:58)、[提交](/Users/alex/html/dating.saas/src/views/system/application/resource/index.vue:95)、[旧服务替换关联](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemApp.php:116)

## 前端最小改动集与 HTTP/RPC 职责

1. 搬迁原框架依赖链与选定平台页面，统一 API base/proxy/upload/static 配置；资源种子指向原页面路径。构建前先检查原工作树已有删除的 VxeTable 对全量 glob 导入的影响；没有经过构建就不能声称直接复制可运行。
2. 启动恢复 saasConf，退出清理权限 store、动态路由、tabs，切换应用后重新拉 auth 并进入存在的默认首页。当前 saasConf 初始对象始终 truthy，getter 的缓存分支可能被遮蔽；退出没有在自身实现中清动态权限，必须回归。[用户 state/getter](/Users/alex/html/dating.saas/src/store/modules/user.ts:37)、[退出](/Users/alex/html/dating.saas/src/store/modules/user.ts:139)
3. 移除或用能力开关隔离两处呼叫中心耦合：user store 的 dcc 退出清理，以及 permission store 在获取 auth 后、`seat_id > 0` 时执行的 `dccStore.initSipConfig()`。后者会调用 getSipConfig，登录后的 auth 流程确实可能主动请求 SIP；底座不能只隐藏 RCC 菜单而保留这条初始化链，也不因此迁入 rcc 后端。[退出清理](/Users/alex/html/dating.saas/src/store/modules/user.ts:146)、[auth 后条件初始化](/Users/alex/html/dating.saas/src/store/modules/permission.ts:107)、[SIP 请求](/Users/alex/html/dating.saas/src/store/modules/dcc.ts:31)
4. 修补资源 API 复合 key/完整集合保存，以及员工“全局账号禁用 vs 租户成员禁用”的按钮语义。可选的全局用户只读页面保留时补 getList(params) 透传，隐藏未完成新增编辑按钮。
5. 平台菜单不下发 CRM、FeelingRing、RCC、模板页面；通用工作台示例动态可隐藏。类型字段修补围绕 API 适配与权限，不重画现有页面。

HTTP gateway 承担旧 URL/GET 写动作兼容、JSON/分页转换、上传、身份解析、经过验证的租户/应用上下文、操作目录以及 profile/auth 聚合。内部 go-micro RPC 按 identity、tenant/membership/organization、application/access、platform-support 模块提供明确命令和查询 DTO；权限和租户校验在服务边界仍须执行，不能只在 gateway 校验。浏览器不用 RPC，controller 不等于独立微服务。成员、组织、角色等强事务模块首期可同服务进程，拆分依据数据/事务边界，不依据 PHP 类数。

## 平台闭环验收链

以下是后续执行验收条件，本轮尚未运行：

1. 空环境通过版本化 schema/种子建立平台管理员、平台应用、资源树/API目录、初始租户和地区数据；无 CRM/RCC/婚恋服务也能启动。
2. init → 登录 → profile/auth → 平台动态菜单加载；头像/logo 上传和预览可用；刷新后上下文和菜单一致。
3. 平台管理员创建租户 → 配置应用/资源和有效期 → 建立单位、部门、岗位 → 创建/关联员工并设置默认应用。
4. 创建角色 → 绑定员工 → 分配菜单/按钮/API与数据范围；资源授权编辑可保存再读取，覆盖取消授权、同 URI 不同 method、超过 30 个 API 项。
5. 使用普通成员登录 → 只见被授权应用/菜单 → 应用切换重新下发 auth → 组织/岗位/员工页面可读写；非法 tenant/app/org 参数、越权 API、跨租户对象 ID 和无数据范围结果均被拒绝。
6. 撤销角色或租户应用、禁用成员/账号、应用授权到期后，旧会话不能继续访问；切换账号不继承旧路由和按钮权限。成员禁用不误伤其他租户。
7. 修改密码/重置密码/退出、HTTP 与业务 401、网络失败回退正常；Go HTTP 返回结构与前端契约用固定脱敏样本比较，RPC 端另验事务与上下文传播。

待补证据：真实 DB schema/index/默认值、平台应用和资源种子、脱敏 HTTP 样本、部署覆盖配置。只凭 Model 和 TS any wrapper 无法冻结全部 DTO 类型与空值行为，57 个路径静态闭合不等于迁移规格已完全冻结。
