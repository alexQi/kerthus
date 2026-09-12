# 旧后端探索：service.beehive

探索日期：2026-09-11。源码根目录：`/Users/alex/wwwroot/service.beehive`。目标：为 kerthus 的 Go Micro SaaS 底座重构建立事实基础，本轮没有实施迁移。

## 结论

旧项目已经包含可复用的 SaaS 业务骨架：全局账号、租户、租户成员、组织/岗位、应用开通、资源菜单、角色/API/数据权限、配置和日志。它同时聚合了 CRM、婚恋活动、联盟投放、营销统计和 Noctua 工具等垂直业务。应迁移核心领域概念和前端契约，不宜照搬全部模块、权限信任链或历史部署结构。

当前工作树是 PHP >=8.0、Hyperf 3.0、Swoole 的 HTTP + JSON-RPC 服务集合。已经有 RPC 接口层和按域数据库连接，因此不是从纯单体开始拆分；但同一个运行配置启动多个域的监听，且服务注册、数据库配置与新增业务源码并未完全对齐。不能据此认定每个源码模块都已独立部署或能正常运行。

## 范围与证据规则

- 检查了源目录及 `/`、`/Users`、`/Users/alex`、`/Users/alex/wwwroot` 的 `AGENTS.md`，未发现适用文件；源目录内也未检出 `AGENTS.md`。
- 按**当前工作树**探索，不是只按 Git HEAD。`git status --short` 显示账号模型/表单/缓存、若干配置和脚本有修改，CRM、FeelingRing 及多个接口为未跟踪文件。没有改动这些文件。
- 只读源码、依赖中的路由/消息实现、配置结构和 Git 状态；没有读取 `.env`、启动 PHP 服务、访问数据库/Redis/Consul/Nacos、执行第三方调用或支付。
- 下面链接指向实际本地源码；“已实现”指可看到实现，“建议/风险/待验证”不代表经过运行验证。静态 API 属性数量不是运行时唯一路由数量。

## 技术与调用结构

依赖清单见 [composer.json](/Users/alex/wwwroot/service.beehive/composer.json:15)。实际声明 Hyperf 3.0 数据库、Redis、JSON-RPC、Consul、异步队列、定时任务、对象存储、追踪、配置中心等；另有自有 PHP 包封装微信、美团联盟、唯品会联盟、Vivo 营销、阿里云 VOD、LLM。存在 gRPC、AMQP、Elasticsearch 等依赖声明，并不等于其业务路径已启用。

主要分层：

```text
HTTP Controller（app/Http/Module/<域>/Controllers）
  → Interface（app/Reference）/ DI 调用
  → Service（app/Services/<域>/Service*.php，JSON-RPC provider）
  → Dao（继承 Model，处理查询/写入）
  → Model（表字段、类型、连接名）

跨域读取还会由 Controller 或 Service 组合多个 Interface。
Task → Queue → Job → Interface/第三方 API 为后台处理链。
```

入口为 `bin/hyperf.php`，HTTP 监听和 5 个 `beehive.*` TCP 域监听在 [server.php](/Users/alex/wwwroot/service.beehive/config/autoload/server.php:21)。TCP 使用 JSON-RPC、CRLF 分帧；Consul 注册/发现、连接池、超时和重试在 [services.php](/Users/alex/wwwroot/service.beehive/config/autoload/services.php:46)。[dependencies.php](/Users/alex/wwwroot/service.beehive/config/autoload/dependencies.php:25) 将命名 TCP server 绑定到 Hyperf JsonRpc TcpServer，并替换核心 HTTP 中间件等。

静态规模：60 个 `*Controller.php`（含抽象/基础控制器），346 处 HTTP Mapping 属性，50 个 Reference 文件，46 个 Service 文件，157 个 Model 文件。没有把这些数量当作独立微服务数。

| 域 | Service / Model 数 | 实际业务与代表证据 | 底座判断 |
| --- | --- | --- | --- |
| System | 10 / 25 | 用户、租户、组织、员工、岗位、角色、应用资源、配置、日志；[ServiceSystemUser](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:29) | 首批核心 |
| CRM | 6 / 15 | 线索分配、公海池、客户、合同、跟进事件、申诉/RCC 控制器；[LeadsController](/Users/alex/wwwroot/service.beehive/app/Http/Module/CRM/Controllers/LeadsController.php:29) | 可选业务包，不进入最小底座 |
| FeelingRing | 11 / 76 | 婚恋会员、活动、分组、话题/动态、渠道、订单退款、帮助、任务、举报；[ServiceFeelingRingOrder](/Users/alex/wwwroot/service.beehive/app/Services/FeelingRing/ServiceFeelingRingOrder.php:141) | 明确垂直业务 |
| Integration | 7 / 14 | 媒体/平台账号、授权 Token、联盟渠道/推广位；[ServiceIntegrationPlatformItem](/Users/alex/wwwroot/service.beehive/app/Services/Integration/ServiceIntegrationPlatformItem.php:21) | 抽取连接器机制，供应商实现可选 |
| Promotion | 3 / 4 | 广告计划、广告组；[ServicePromotionCampaign](/Users/alex/wwwroot/service.beehive/app/Services/Promotion/ServicePromotionCampaign.php:17) | 营销业务包 |
| Statistics | 4 / 14 | CPS 订单、归因、投放统计、日报/月报等；[ServiceStatisticsCpsOrder](/Users/alex/wwwroot/service.beehive/app/Services/Statistics/ServiceStatisticsCpsOrder.php:16) | 营销分析业务包 |
| Noctua | 5 / 9 | LLM、代理、工具、统计；[Noctua 控制器目录](/Users/alex/wwwroot/service.beehive/app/Http/Module/Noctua/Controllers) | 可选能力；需单独确认用途 |
| Psychology | 0 / 0 | 目录存在，未见实现 | 不作为现成功能 |

`App` HTTP 模块另外提供公共信息、文件上传和微信接口。[文件上传](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/FileController.php:21) 直接使用七牛 Filesystem，返回对象路径。

## 路由与前端兼容契约

`config/routes.php` 没有手工路由。路由来自 Controller 与 Mapping 属性，`ApiController` 只声明抽象 `query/save/delete`，不自动暴露全部 public 方法。[ApiController](/Users/alex/wwwroot/service.beehive/app/Http/Controller/ApiController.php:24)

本地安装的 Hyperf 实现明确说明：未写 Mapping.path 时为 `prefix + '/' + snake_case(methodName)`；显式相对 path 拼到 prefix；空 path 直接指向 prefix；绝对 path 覆盖 prefix。类与方法中间件合并。见 [DispatcherFactory](/Users/alex/wwwroot/service.beehive/vendor/hyperf/http-server/src/Router/DispatcherFactory.php:154)。因此不能单凭 PHP 方法名推断大小写或 HTTP 方法。

优先冻结以下外部协议：

| 协议面 | 当前行为 | 证据 |
| --- | --- | --- |
| 登录 | `POST /system/user/login`，`scene=phone/email`，`username/password`；返回 `user_id/access_token/expires_time/tenant_id/app_id/unit_id/section_id/app_code` | [UserController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/UserController.php:45)、[FormUser](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:100) |
| 请求上下文 | 自定义 `access-token`、`tenant-id`、`app-id`、`unit-id`、`section-id`，以及 client 元信息 | [AppMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:61)、[TokenMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/TokenMiddleware.php:42) |
| 权限初始化 | `GET /system/user/auth` → `roles/menus/routes/permissions/tenant` | [ServiceSystemUser](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:151) |
| 动态路由 | 资源树包含 `name/icon/path/component/is_public/open_with/redirect/meta/children`，按钮权限是 code 数组 | [User Logic](/Users/alex/wwwroot/service.beehive/app/Services/System/Logic/User.php:80) |
| 通用成功响应 | `{code:0,msg:"success",mesc:<耗时>,data:<返回值>}` | [AppMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:97) |
| 异常响应 | 同名 envelope；仅 code=401 特别设置 HTTP 401，其余沿用 response status | [AppExceptionHandler](/Users/alex/wwwroot/service.beehive/app/Exception/Handler/AppExceptionHandler.php:40) |
| 查询分页 | 输入 `field/order/page/pageSize`，`ascend` 转 asc，其余 order 转 desc；输出 `page/total/items` | [TraitApi](/Users/alex/wwwroot/service.beehive/app/Traits/TraitApi.php:44)、[TraitQuery](/Users/alex/wwwroot/service.beehive/app/Traits/TraitQuery.php:57) |
| 文件上传 | `POST /app/file/upload`，multipart `file` + `directory/param`；返回对象 key 字符串 | [FileController](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/FileController.php:32) |

现有方法并非统一 REST：例如 `GET /system/user/delete`、`GET /system/user/setStatus` 会写数据，而 CRM 的 `query` 使用 POST。若前端直接搬迁，应让网关显式适配旧协议，内部 Go API 使用明确 DTO 与读写语义。

分页也存在待冻结差异：TraitApi 默认 page=0，而 DAO 的 skip 使用 `(page-1)*pageSize`。应以实际前端传参和脱敏样本确定兼容行为，不把这一默认值当成正确规范。

## SaaS 领域模型与权限

核心关系从 Model 字段与 Service/DAO 调用可确认：

```text
User（全局账号） ← TenantEmployee → Tenant
TenantEmployee → 默认 App / Unit / Section
TenantOrg（unit/section 树） ← EmployeeOrg；EmployeePosition → TenantPosition
User ← TenantRoleItem → TenantRole
App → AppResource（menu/view/action/field） → AppResourceApi（URI/Method）
Tenant → TenantApp → TenantAppResource
TenantRole → TenantRoleResource（app/resource/data_access）
```

表字段证据：[TenantEmployee](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/TenantEmployee.php:10)、[TenantOrg](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/TenantOrg.php:10)、[AppResource](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/AppResource.php:10)、[TenantRoleResource](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/TenantRoleResource.php:10)。用户能关联多个租户，存在默认租户/应用上下文；租户与租户应用有过期字段。

认证流程：按手机/邮箱检索账号，验证密码和账号状态，读取默认租户并检查租户有效期，生成 opaque token，Redis hash 保存身份与过期时间。按 `is_signal_login` 可清除前一 token；不是 JWT。见 [FormUser](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:41)、[CacheUser](/Users/alex/wwwroot/service.beehive/app/Services/System/Cache/CacheUser.php:26)。TokenMiddleware 只把 Redis 身份中的 user_id/expires_time 写回请求，租户和应用仍来自请求头。

授权分三层：

1. AuthMiddleware 判断 user_id 是否存在。
2. AccessMiddleware 读取角色关联 API，以路径对应单个 HTTP method 验证权限；菜单、路由和按钮权限由资源树生成。
3. 标注 DataScopeInterceptor 的动作通过 AOP 计算 owner user_id 集合，DAO 显式添加 where/whereIn。数据范围 0 全部、1 单位含下级、2 本单位、3 部门含下级、4 本部门、5 自己。见 [DataScope](/Users/alex/wwwroot/service.beehive/app/Aspect/DataScope.php:39)、[getEmployByScope](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemTenantEmloyee.php:157)、[DaoCustomer](/Users/alex/wwwroot/service.beehive/app/Services/CRM/Dao/DaoCustomer.php:132)。

租户隔离属于显式列过滤，不是每租户单独数据库，也未见一个覆盖所有模型的全局隔离器。`ScopeAll` 返回空数组，DAO 对空 user_ids 不添加 owner 过滤；Go DTO 必须区分“所有人”与“没人”，否则很容易意外放宽访问。

## 数据与迁移完整性

配置文件声明 5 个连接：default、integration、statistics、promotion、noctua。[databases.php](/Users/alex/wwwroot/service.beehive/config/autoload/databases.php:18) 各域模型分别选择命名连接；System 使用 default。

CRM 模型需要 `crm`，FeelingRing 模型需要 `dating` 和 `user`，这些没有出现在当前 `databases.php` 的顶层连接配置中。新增 Service 的 RPC server 名称 `dating.crm`、`dating.feelingRing` 也没有出现在当前 `server.php` 监听清单，相关接口未列入 `services.php` consumers。**这是源码/默认配置不闭合的事实；可能另有配置覆盖或其他部署项目，未验证。**代表证据：[CRM Customer](/Users/alex/wwwroot/service.beehive/app/Services/CRM/Model/Customer.php:85)、[FeelingRing Order](/Users/alex/wwwroot/service.beehive/app/Services/FeelingRing/Model/Dating/Order.php:34)、[FeelingRing User](/Users/alex/wwwroot/service.beehive/app/Services/FeelingRing/Model/User/User.php:36)、[CRM Service](/Users/alex/wwwroot/service.beehive/app/Services/CRM/ServiceCRMCustomer.php:24)。

未发现版本化数据库迁移目录；`sqls/test.sql` 是 128 行 SQL，未检出 CREATE TABLE，不能作为完整初始化 schema。Model 描述字段，但不足以证明数据库真实类型、索引、约束、默认值、触发器和视图。菜单/资源/API/字典大量来自数据库，单拷贝前端和 PHP 常量无法重建相同权限导航。

正式迁移需要额外得到脱敏 schema/字典与必要种子数据，确认 ID 保留方式、时区/秒与毫秒、JSON 字段编码、软删除语义、金额单位、旧账号哈希升级策略；本轮没有假设这些信息已经存在。

## 异步、集成和运行

- Redis AsyncQueue 配置了 9 个队列：Token 刷新、VIP 账号同步、OCPX 回传、广告计划/组同步、CPS 订单/归因和投放统计。每队列配置 30 秒超时、重试与并发限制。[async_queue.php](/Users/alex/wwwroot/service.beehive/config/autoload/async_queue.php:13)
- JobBase 统一接收 params、写日志、执行任务，默认不做生产环境限制，失败允许重试。[JobBase](/Users/alex/wwwroot/service.beehive/app/Jobs/JobBase.php:32) 本地 Hyperf 队列消息有 PHP 对象序列化实现，不能让 Go worker 直接把现有积压当成 JSON 消费。[Message](/Users/alex/wwwroot/service.beehive/vendor/hyperf/async-queue/src/Message.php:68)
- Crontab 包含 VIP 订单和 Vivo 投放同步；其中一条 VIP 任务显式关闭，其余任务存在启用路径，启动旧服务可能触发外部调用。[crontab.php](/Users/alex/wwwroot/service.beehive/config/autoload/crontab.php:19)
- 微信支付下单、通知处理、退款有封装；FeelingRing 调用退款，但这不等于已有可复用 SaaS 订阅计费系统。[Payment](/Users/alex/wwwroot/service.beehive/app/Logic/LogicWechat/Payment.php:33)
- 租户存在余额/充值 Model，但未发现与完整套餐、周期订阅、额度、账单、续费状态机对应的服务闭环；目前能确认的 entitlement 更接近“租户开通应用/资源 + 有效期”。[TenantAssetCharge](/Users/alex/wwwroot/service.beehive/app/Services/System/Model/TenantAssetCharge.php:10)
- Nacos/etcd 配置中心有实现结构，默认是否启用取决于环境；Consul 服务发现启用。运行配置未加载验证。[config_center.php](/Users/alex/wwwroot/service.beehive/config/autoload/config_center.php:17)
- TraceMiddleware 全局开启，存在 SQL、API、队列日志；登录还有异步 access log。健康检查 `/live.probe` 只返回 ok，不检查下游 readiness。[HealthController](/Users/alex/wwwroot/service.beehive/app/Http/Controller/HealthController.php:29)
- Dockerfile 使用 Hyperf PHP8/Swoole5 镜像、Composer 安装、启动 Hyperf；历史 `deploy.test.yml` 的端口和配置挂载目录与当前 Dockerfile/server.php 不一致，不能直接视作可部署清单。[Dockerfile](/Users/alex/wwwroot/service.beehive/Dockerfile:43)、[deploy.test.yml](/Users/alex/wwwroot/service.beehive/deploy.test.yml:8)
- 测试只有示例用例，包含 assertTrue(true) 和 GET `/` 是否数组；没有证明租户隔离、权限、业务事务正确的覆盖。[ExampleTest](/Users/alex/wwwroot/service.beehive/test/Cases/ExampleTest.php:20)

## 重构前必须处理的具体风险

以下是静态源码证据，不是线上漏洞复现；优先级用于设计迁移阻断条件。

| 优先级 | 已观察事实 | Go 重构处理 |
| --- | --- | --- |
| 高 | AppMiddleware 把请求头 tenant-id=1 标作管理员；AccessMiddleware 直接跳过权限；TraitApi 在此分支让 request body 覆盖 tenant/user 参数。[AppMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/AppMiddleware.php:65)、[AccessMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/AccessMiddleware.php:49)、[TraitApi](/Users/alex/wwwroot/service.beehive/app/Traits/TraitApi.php:25) | 只信任认证后验证过的成员身份/租户/应用上下文；平台管理员独立能力，不能由租户编号推导 |
| 高 | 密码是无盐 SHA1/MD5 组合；Token 用 mt_rand/time 经 MD5；日志记录全量 request 和 access-token。[FormUser](/Users/alex/wwwroot/service.beehive/app/Services/System/Form/FormUser.php:78)、[DebugMiddleware](/Users/alex/wwwroot/service.beehive/app/Middleware/DebugMiddleware.php:54) | 旧哈希仅用于迁移期校验后升级；新口令使用现代密码哈希，新 Token 用安全随机源；日志字段白名单和脱敏 |
| 高 | `/app/file/upload` 没有 Auth 属性，允许客户端传 directory/param，未见该动作的类型/大小/租户归属校验。[FileController](/Users/alex/wwwroot/service.beehive/app/Http/Module/App/Controllers/FileController.php:32) | 文件域集中处理租户归属、授权、大小和内容校验 |
| 高 | 退款方法在 crm 连接开启事务，在 dating 提交/回滚；方法声明 bool 却没有 return，含空余额退款分支。[ServiceFeelingRingOrder](/Users/alex/wwwroot/service.beehive/app/Services/FeelingRing/ServiceFeelingRingOrder.php:156) | 退款业务不能照搬；明确状态机、幂等键、供应商调用与本地事务/补偿边界 |
| 中 | getAuthApis 读取 `$resource_api`，但判断 `empty($user_apis)`，变量不一致。[ServiceSystemUser](/Users/alex/wwwroot/service.beehive/app/Services/System/ServiceSystemUser.php:205) | 重建权限缓存并测试失效，不沿用此实现 |
| 中 | API 权限用 `pluck(method,uri)`，同一路径多个 method 无法自然表达。[DaoAppResourceApi](/Users/alex/wwwroot/service.beehive/app/Services/System/Dao/DaoAppResourceApi.php:57) | 权限键按 resource/action 或 method+route 明确建模 |
| 中 | modifyInfo/modifyPassword 没有 Auth 属性，UserController 类也未挂 Auth；匿名请求可能进入修改逻辑，但实际结果待运行验证。[UserController](/Users/alex/wwwroot/service.beehive/app/Http/Module/System/Controllers/UserController.php:91) | 默认拒绝、公开接口显式 allowlist |
| 中 | DAO 手工租户过滤、AOP 将 scope 写到控制器实例属性；Swoole 常驻实例下并发隔离需要额外确认。[DataScope](/Users/alex/wwwroot/service.beehive/app/Aspect/DataScope.php:80) | Go context 传递不可变身份和 scope；仓储强制 tenant 参数，不用可变共享控制器状态 |
| 中 | 新域缺少匹配的默认 DB/RPC 配置；仅示例测试；部分方法 TODO | 以可运行纵向用例界定“迁移完成”，不用文件数量判断完成度 |

## 面向 Go Micro 的初步边界建议

此处是领域建议，不是本轮已实现架构，也没有在本报告锁定 Go Micro 版本、插件或部署方式。

1. **gateway / admin API**：承接旧前端路径与 envelope，负责输入校验、会话验证、可信租户上下文、限流、追踪；逐项保留动态菜单/权限 DTO。RPC 不直接暴露给浏览器。
2. **identity**：全局用户、口令、登录会话、撤销、账号状态；旧用户表与 token 迁移边界清晰。账号与婚恋 C 端会员应区别建模。
3. **tenant / organization**：租户生命周期、成员、单位部门、岗位、默认上下文；角色与组织先保持紧密模块关系，避免一次查询跨多个 RPC。
4. **access / application**：应用目录、租户开通、资源树、角色、策略/数据范围、前端导航；首期可以与 tenant 共一个服务进程，但代码和表归属明确。未来的订阅/计费在这里定义 entitlement 接口，不直接混入支付供应商细节。
5. **platform support**：字典配置、文件、审计作为独立模块；有实际负载和所有权需求时再拆服务。队列/调度属于运行基础能力，领域 Job 留在所属业务。
6. **optional domains**：CRM、FeelingRing、营销集成/投放/统计、Noctua 各自隔离；首版 SaaS 底座可完全不启用。

推荐首个可验收纵向切片：登录 → 获取 profile/auth → 动态路由加载 → 租户/应用切换 → 角色授权 → 一个带租户和部门范围的 CRUD → 审计记录。对这一切片补兼容测试和跨租户负例，再决定前端整体搬迁顺序。

切换机制建议：先冻结 HTTP 契约与必要种子数据；Go gateway 可先整体接管 System/App；其余业务暂经明确兼容适配继续路由到旧实现。队列需要排空或单独桥接，数据库每阶段指定唯一写入方，避免假定 PHP JSON-RPC/序列化消息可以无改动替换为 Go Micro RPC。

## 尚未验证

没有编译/运行旧后端或调用测试；不能确认线上实际版本、环境覆盖、第三方包可安装性、外部服务拓扑、数据库真实 schema/数据、菜单种子完整性、所有 HTTP/RPC 路由可达性。没有得到生产流量样本或业务验收标准。上述边界已经足以开展 Go 底座设计，但不等于完成逐个业务流程的迁移规格。
