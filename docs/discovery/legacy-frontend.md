# 旧前端 dating.saas 源码探索

探索日期：2026-09-11。源码根目录：`/Users/alex/html/dating.saas`。本报告基于当前工作区实际文件（包含已有未提交改动），只读检查旧项目；未安装依赖、构建、运行页面、连接后端或验证线上行为。项目及所查父目录未发现适用 `AGENTS.md`。文中“已实现”指能看到页面和调用链，不代表已通过运行验收；“模板/未完成”有具体源码证据；迁移建议属于推断。

## 1. 核心结论

这是基于 Vue Vben Admin 改造的 **SaaS 管理后台**，不是面向终端用户的婚恋网站。它已有可复用的登录、平台租户管理、应用及资源授权、租户组织/岗位/员工/角色管理、应用切换，同时混入 CRM 与 feelingRing 婚恋运营业务。后台壳和 SaaS 平台页面有较高复用价值，不能把全部目录直接等同于完整、通用、已运行的 SaaS 产品。

如果后端用 go-micro 重构、前端尽量原样搬迁，首先需要保留 **HTTP 网关契约**，尤其是自定义上下文请求头、`{code,data,msg}` 返回包、菜单/路由/权限下发、应用初始化、文件返回格式。前端不直接调用微服务 RPC；保留 HTTP 适配层后，内部服务如何拆分可以独立推进。

当前旧仓库有既有未提交修改：`package.json`、`yarn.lock`、入口/样式配置、CRM 表格页面等；`src/components/VxeTable` 整目录存在 staged 删除。搬迁应以经过确认的当前工作区快照为起点，不能默认从旧仓库 HEAD 导出。探索未修改这些文件。

## 2. 技术栈与入口

| 项目 | 实际情况与证据 |
| --- | --- |
| 框架 | Vue `^3.2.47`、Vue Router `^4.1.6`、Pinia `2.0.12`、TypeScript `^4.9.5`、Vite `^4.3.9`；见 [package.json](/Users/alex/html/dating.saas/package.json:1)。这是 manifest 范围，不是本机已安装版本。 |
| UI | Ant Design Vue `^3.2.15`、Windi CSS、Less/SCSS、SVG/Iconify、ECharts、Vue i18n；还有 Excel、Markdown、Tinymce、Cropper、LogicFlow 和 WebPhone 封装。 |
| 网络 | Axios `^0.26.1` + 自定义 VAxios；[HTTP 配置](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:30)。 |
| 引导顺序 | store → 本地 UI 配置 → 通用组件 → i18n → router/guard → directives/error handler → mount；[main.ts](/Users/alex/html/dating.saas/src/main.ts:25)。 |
| 根组件 | Ant ConfigProvider + AppProvider + RouterView；同时请求应用配置；[App.vue](/Users/alex/html/dating.saas/src/App.vue:1)、[useAppConfig](/Users/alex/html/dating.saas/src/hooks/setting/useAppConfig.ts:1)。 |
| 路由 | Hash history，静态登录/根页面/重定向/404/群组详情，业务路由主要由后端下发；[router/index.ts](/Users/alex/html/dating.saas/src/router/index.ts:18)、[基础路由集合](/Users/alex/html/dating.saas/src/router/routes/index.ts:19)。 |
| 体量 | 当前 `src` 有 363 个 `.vue`，其中 `views` 217 个；`src/api` 36 个 `.ts`（含模型）。这些是文件数，不是页面或功能数。 |

`/@/` 映射 `src/`，`/#/` 映射 `types/`，虚拟 Windi/SVG 模块、主题插件和 `build/` 脚本均参与编译。因此只搬 `views/` 不够；需要带上构建链、基础组件、hooks、工具、类型、constants、assets/locales 等依赖。[Vite aliases](/Users/alex/html/dating.saas/vite.config.ts:37)

## 3. 登录、权限、多租户与应用切换

1. 登录页发送 `username/password/scene`，当前 `scene` 固定为 `phone`，调用 `POST /system/user/login`。返回 token 对象包含 `user_id/access_token/expires_time/tenant_id/seat_id/app_id/unit_id/section_id/app_code`。[登录提交](/Users/alex/html/dating.saas/src/views/common/login/LoginForm.vue:133)、[模型](/Users/alex/html/dating.saas/src/api/common/model/userModel.ts:6)
2. 登录完成调用 `GET /system/user/profile`、`GET /system/user/auth`；auth 返回 `menus/routes/roles/permissions`。数据写入 Pinia 与本地缓存。[user store](/Users/alex/html/dating.saas/src/store/modules/user.ts:87)、[permission store](/Users/alex/html/dating.saas/src/store/modules/permission.ts:85)
3. Axios 在已存在 token 时发送 `access-token`，并发送 `tenant-id/unit-id/section-id/app-id`；优先用可变 `saasConf`，否则回退登录 token 的上下文。[请求头](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:154)
4. 权限模式为 `BACK`；菜单、按钮权限依赖服务端权限码，前端 `admin` 角色放行全部按钮。动态组件名精确映射 `src/views` 文件路径，支持 `LAYOUT/IFRAME` 和 `open_with`。后端必须返回兼容的 `component/path/name/meta/children` 结构；任意移动业务页面目录都可能破坏数据库中的组件路径。[权限设置](/Users/alex/html/dating.saas/src/settings/projectSetting.ts:26)、[权限判断](/Users/alex/html/dating.saas/src/hooks/web/usePermission.ts:29)、[路由解析](/Users/alex/html/dating.saas/src/router/helper/routeHelper.ts:20)
5. 顶部应用选择与工作台应用卡片调用 `getTenantApps`，切换前 `hasApp({app_id})`；更新 `saasConf.appId/appCode`，关闭 tab、重置 router、重新拉取 auth。应用首页拼成 `/{appCode}/dashboard`，而静态根 redirect 与登录 guard 另有 `/dashboard/main`，这些入口需一并核对。[应用切换](/Users/alex/html/dating.saas/src/layouts/default/header/components/apps/index.vue:73)、[重载流程](/Users/alex/html/dating.saas/src/hooks/web/usePermission.ts:20)、[首页](/Users/alex/html/dating.saas/src/store/modules/user.ts:174)、[PageEnum](/Users/alex/html/dating.saas/src/enums/pageEnum.ts:5)

**已有多租户 UI 证据**：平台租户列表和租户详情，详情有应用/组织/岗位/角色/员工 tab；业务请求有 tenant 等上下文，应用可切换。**未能由前端证明**：服务端数据隔离、跨租户权限验证、租户与组织的数据权限层级；不能信任客户端传来的 ID。[租户详情](/Users/alex/html/dating.saas/src/views/system/tenant/detail/index.vue:1)

**缓存和会话待修点**：默认用 localStorage 缓存权限/token；未找到 refresh-token 请求或刷新流程，HTTP 401 走清 token/退出。`getSaasConf` 的初始 state 是一个 truthy 空配置对象，可能遮蔽持久化缓存；刷新页面后首页与当前应用恢复需要实际回归。`logout` 未在自身函数里清 permission store 或移除动态路由，同一浏览器换账号需要重点验收。这些属于代码推断，不是已复现故障。[缓存适配](/Users/alex/html/dating.saas/src/utils/auth/index.ts:6)、[SaaS getter](/Users/alex/html/dating.saas/src/store/modules/user.ts:37)、[退出](/Users/alex/html/dating.saas/src/store/modules/user.ts:139)、[401](/Users/alex/html/dating.saas/src/utils/http/axios/checkStatus.ts:29)

## 4. 可复用的真实页面与 API 映射

以下为源码组件地址，**不是已验证的运行 URL**；实际 URL 和页面是否对用户开放依赖后端 auth 的 routes。

| 业务域 | 实际页面/组件 | 后端 API 入口与完成度 |
| --- | --- | --- |
| 平台租户 | `views/system/tenant/main`、`detail`、`user` | [tenant.ts](/Users/alex/html/dating.saas/src/api/tenant/tenant.ts:4)：query/info/save/delete/setStatus/verify/getItems；租户关联应用、组织、岗位、角色、员工均有页面。 |
| 平台应用/授权 | `views/system/application/main`、`resource`、`auth`、`authorize` | [application.ts](/Users/alex/html/dating.saas/src/api/application/application.ts:4)：应用 CRUD、资源树及资源 API；[authorize.ts](/Users/alex/html/dating.saas/src/api/application/authorize.ts:4)：租户应用授权/取消授权。 |
| 组织、岗位、员工 | `views/basic/user/org`、`position`、`main`；平台租户详情有一套对应组件 | `/system/org/*`、`/system/position/*`、`/system/employee/*`；[org](/Users/alex/html/dating.saas/src/api/basic/org.ts:4)、[position](/Users/alex/html/dating.saas/src/api/basic/position.ts:4)、[employee](/Users/alex/html/dating.saas/src/api/basic/employee.ts:4)。 |
| 角色与资源权限 | `views/basic/system/role`、`views/system/tenant/detail/components/roles` | `/system/role/query/save/delete/setStatus/queryRoleResources/authRoleResource/relateEmployee`；[role](/Users/alex/html/dating.saas/src/api/tenant/role.ts:4)。 |
| 应用工作台、个人资料 | `views/common/dashboard`、`profile`、`views/basic/application/main` | 应用卡片切换是真实请求；个人资料/密码修改调用 `/system/user/modifyInfo/modifyPassword`，管理员 resetPassword；[资料](/Users/alex/html/dating.saas/src/views/common/profile/BaseSetting.vue:33)、[用户 API](/Users/alex/html/dating.saas/src/api/user/user.ts:4)。 |
| 呼叫中心 | `views/basic/rcc/main`、`record`，`views/crm/rcc/main`、`components/WebPhone` | `/system/rcc/*` 包括供应配置、坐席同步/分配/释放、通知记录、代理呼叫、SIP 配置；CRM `/crm/rcc/query`；[rcc API](/Users/alex/html/dating.saas/src/api/rcc/rcc.ts:4)。 |
| CRM 客户、线索、公海 | `views/crm/customer/main`、`customer/pool`、`leads/main`、`leads/city`、`pool/main` | `/crm/customer/*`（过滤/详情/转移/领取/分配/入池/导入/照片）、`/crm/leads/*`（列表/派发/导入）、`/crm/pool/*`；详情有认证、择偶、照片、服务、跟进等，明显带婚恋领域数据。[客户页面](/Users/alex/html/dating.saas/src/views/crm/customer/main/index.vue:147)、[线索](/Users/alex/html/dating.saas/src/views/crm/leads/main/index.vue:159)。 |
| CRM 跟进、合同、申诉 | `views/crm/components/customerInfo`、`contract/main`、`contract/checking`、`appeal/main` | `/crm/event/query/save/delete`、`/crm/contract/*`（合同/明细/文件/审核）、`/crm/appeal/*`（申诉/审核）；[customer API](/Users/alex/html/dating.saas/src/api/customer/customer.ts:4)、[contract API](/Users/alex/html/dating.saas/src/api/contract/contract.ts:4)。 |
| CRM 数据看板 | `views/crm/dashboard` | `/crm/app/getStatistics/getFollowRatio/getCustomerTrend/getSaleRank/getConvertRatio`，统计组件确实取 API；[Statistic](/Users/alex/html/dating.saas/src/views/crm/dashboard/component/Statistic.vue:39)。 |
| feelingRing 会员/相册 | `views/feelingRing/member/main`、`member/album`、`components/member` | `/feelingRing/member/query/info/attr/queryPhoto/queryRelation/setPhotoStatus`；会员详情的基础信息、相册、订单、邀请关系有真实接线。[member API](/Users/alex/html/dating.saas/src/api/member/member.ts:4) |
| 活动、订单、群组匹配 | `views/feelingRing/activity/main`、`activity/order`、`components/activity`、`group/detail`、`components/order` | 活动列表/详情/报名处置/图表、订单和明细状态、群组 save/start/finish/partFinish/getResult；**活动新增编辑未完成**，见下一节。[activity API](/Users/alex/html/dating.saas/src/api/activity/activity.ts:4)、[group API](/Users/alex/html/dating.saas/src/api/group/group.ts:4)。 |
| 推广、话题、帮助 | `views/feelingRing/promotion/channel`、`moment/topic`、`help/category`、`help/main` | `/feelingRing/channel/*`、`/feelingRing/topic/*`、`/feelingRing/help/category/*`、`/feelingRing/help/question/*`；另有 `/app/wechat/getQrcode/getLink` 的推广二维码/链接。[微信 API](/Users/alex/html/dating.saas/src/api/app/wechat.ts:4) |

通用 SaaS 底座建议先复用 `system/basic/common` 中经过核实的平台能力；CRM、婚恋活动/会员/订单/推广和呼叫中心应视为可选业务模块，不是所有 SaaS 都需要的基础能力。前端现有源码未显示通用订阅计费、套餐配额、账单、开发者 API key、webhook 管理的完整实现；不能从“应用授权到期时间”推定存在这些能力。

完整逐接口清单见 [API 静态交叉映射](/Users/alex/golang/src/kerthus/docs/discovery/api-contract-map.md)。团队扫描 `src/api` 枚举 URL 对应的直接 defHttp 调用，共 168 个唯一 method/path，153 个与旧 PHP 声明匹配，15 个未匹配，另有 3 个枚举未直接调用。这不是实际页面调用数量或端到端通过率。未匹配包含全部 11 个 `/system/rcc/*`，说明呼叫中心至少需要核对不同部署版本、外部模块或代理；另有 `/app/info/location`、`/system/employee/getSeatItems`、`/getMenuList`、`/testRetry`。不能只保留当前旧 PHP 源码中的路由就断言前端所有功能均可接回。

## 5. HTTP 兼容协议细节

| 层面 | 前端当前要求 |
| --- | --- |
| 地址拼接 | 普通 API 最终为 `VITE_GLOB_API_URL + VITE_GLOB_API_URL_PREFIX + endpoint`；开发代理由 `VITE_PROXY` 控制。上传使用独立 `VITE_GLOB_UPLOAD_URL`。见 [beforeRequestHook](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:99)、[配置读取](/Users/alex/html/dating.saas/src/hooks/setting/index.ts:6)。 |
| JSON 包 | 默认成功仅认 `code === 0`，解包 `data`，消息字段 `msg`；业务 `code === 401` 与 HTTP 401 都有会话处理，但路径不同；[转换](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:34)、[枚举](/Users/alex/html/dating.saas/src/enums/httpEnum.ts:4)。 |
| 参数 | GET 为 query，默认增加时间戳；非 GET 未显式给 `data` 时把 `params` 转 JSON body，自动格式化日期。不可仅按 API wrapper 的 `params` 字面推断是 query。[参数转换](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:109) |
| 分页/排序 | `page/pageSize` → `data.items/data.total`；排序提交 `field/order`，order 直接来自表格，不在全局改成 asc/desc。部分页面自定义 fetch 参数需逐页记录。[componentSetting](/Users/alex/html/dating.saas/src/settings/componentSetting.ts:10) |
| ID/日期/枚举 | 现有接口大量 snake_case，页面依赖嵌套关系对象、整型 status 与 Unix 秒时间戳（显示前 `* 1000`）。Go 输出 RFC3339 或字段改名会破坏页面。[客户渲染](/Users/alex/html/dating.saas/src/views/crm/customer/main/index.vue:112) |
| 动作方法 | 旧接口有大量 GET 执行 delete/setStatus/领取/坐席操作，不是 REST 语义；兼容期要明确映射。全局启用了 GET retry 配置，而其重试处理未 return retry 的 promise，且 GET 本身未必幂等，需一起检查。[retry 调用](/Users/alex/html/dating.saas/src/utils/http/axios/index.ts:218)、[租户动作](/Users/alex/html/dating.saas/src/api/tenant/tenant.ts:26) |
| 上传 | `FormData` 默认文件字段 `file`，数组附加参数编码 `key[]`；uploadFile 直接返回 Axios response，没有走普通 request 的解包。UploadModal 取 `response.data.code/msg/data`，其中 data 是文件地址；现有类型声明却是 `message/code/url`。[上传实现](/Users/alex/html/dating.saas/src/utils/http/axios/Axios.ts:123)、[消费格式](/Users/alex/html/dating.saas/src/components/Upload/src/UploadModal.vue:176)、[类型](/Users/alex/html/dating.saas/src/api/app/model/uploadModel.ts:1)。 |
| 上传待修 | uploadFile 直接发 axios request 未附 requestOptions，而 request interceptor 直接解构 `config.requestOptions`，按代码可能上传前就报错，需实测；Tinymce 用 Ant Upload 直传，不共享上下文 header，并消费 `response.url`，需统一。[拦截器](/Users/alex/html/dating.saas/src/utils/http/axios/Axios.ts:80)、[Tinymce](/Users/alex/html/dating.saas/src/components/Tinymce/src/ImgUpload.vue:3)。 |
| 初始化资源 | 页面启动请求 `/app/info/init`，资源路径依赖返回 `static_url`；`/system/config/districts/industries` 供地区行业字典，`/app/info/servers/routes` 供资源管理页枚举后端接口。[初始化 store](/Users/alex/html/dating.saas/src/store/modules/app.ts:114)、[图片拼接](/Users/alex/html/dating.saas/src/utils/file/resource.ts:9)、[资源选择](/Users/alex/html/dating.saas/src/views/system/application/resource/data.ts:5)。 |

`/app/info/servers/routes` 与资源 API 的 `server/controller/uri/method/action` 模型还反映了旧后端框架的接口发现方式。go-micro 重构时即使 URL 不变，也需要用新的路由目录/权限资源注册机制兼容这些字段，或同时改资源编辑器。[资源 API 组装](/Users/alex/html/dating.saas/src/views/system/application/resource/apiForm.vue:36)

## 6. 模板遗留和未完成项，不能按目录名认定已存在

- `views/feelingRing/moment/main/index.vue` 是 Ant Upload 示例，直连 mocky action 并展示固定示例图，没有动态运营列表；[证据](/Users/alex/html/dating.saas/src/views/feelingRing/moment/main/index.vue:1)。同目录遗留表单及 `chat/main` 实际调用 channel API，不能据此认定已有聊天/动态服务。[chat](/Users/alex/html/dating.saas/src/views/feelingRing/chat/main/index.vue:61)
- 活动编辑抽屉 `handleSubmit` 只取表单并 `console.log`，无 save 请求；对应 activity API 文件也没有 save 函数。因此活动列表/运营能力存在，活动创建闭环不完整。[证据](/Users/alex/html/dating.saas/src/views/feelingRing/activity/main/component/form/index.vue:38)
- `system/log/main` 名为日志页，但调用用户列表、标题“应用列表”、删除仅 log；不能计为审计日志实现。[证据](/Users/alex/html/dating.saas/src/views/system/log/main/index.vue:34)
- `feelingRing/dashboard` 通过 timeout 结束 loading，属于示例图表框架；区别于 CRM dashboard 真实请求统计。[证据](/Users/alex/html/dating.saas/src/views/feelingRing/dashboard/index.vue:20)
- 优惠券发放弹窗查 member/query，确认只 log；不能计为券系统。会员 money/point 等文件存在也不能直接算账务实现。[证据](/Users/alex/html/dating.saas/src/views/feelingRing/components/member/coupon.vue:46)
- 登录记住我无逻辑，短信/二维码/注册入口被隐藏，忘记密码仅 resetFields，不能计为完整身份产品。[登录页](/Users/alex/html/dating.saas/src/views/common/login/LoginForm.vue:29)、[忘记密码](/Users/alex/html/dating.saas/src/views/common/login/ForgetPasswordForm.vue:59)
- README、package repository/author、GitHub Pages workflow 仍含 Vben 模板信息；`/getMenuList` API wrapper 存在，但当前 permission store 用 `/system/user/auth`，不可把旧 wrapper 误当当前菜单入口。

## 7. UI、状态和外部耦合

UI 壳已有侧栏/顶栏、面包屑、多标签/KeepAlive、主题颜色和深色模式、布局设置、全屏/锁屏/国际化。通用 BasicTable/BasicForm/Modal/Drawer/Tree/Upload/Excel 等是业务页主要依赖，可整体复用。[布局配置](/Users/alex/html/dating.saas/src/settings/projectSetting.ts:56)、[主题初始化](/Users/alex/html/dating.saas/src/logics/initAppConfig.ts:25)

Pinia 分 app/user/permission/locale/multipleTab/lock/errorLog/dcc；全局持久化既存 UI 配置也存用户权限和呼叫中心配置。可复用这些组织形式，但新项目需明确哪些缓存按用户、租户、应用划分并在切换时失效。

外部耦合重点：

- WebPhone 用 JsSIP，登录用户有 seat_id 会拉 `/system/rcc/getSipConfig`；浏览器连配置指定主机的 `wss://…/webrtc/`，需要 WebRTC/SIP 服务及音频权限，配置有 realm/user/ha1/pcConfig。SaaS 壳若先不启用呼叫中心，需要把该登录后依赖做成可选能力。[初始化](/Users/alex/html/dating.saas/src/store/modules/permission.ts:107)、[配置](/Users/alex/html/dating.saas/src/store/modules/dcc.ts:31)、[连接](/Users/alex/html/dating.saas/src/hooks/app/useDcc.ts:49)
- 活动地点组件使用高德，并存在写在源文件中的地图 key；迁移时改用环境配置并核查域名授权，报告不记录值。[位置](/Users/alex/html/dating.saas/src/views/feelingRing/activity/main/component/form/model/map.vue:19)
- 微信二维码/链接、静态文件存储域名、地区/行业字典依赖后端；当前图片/品牌素材具有婚恋项目特色，应当配置化。
- `tencentcloud-sdk-nodejs` 等依赖出现在 manifest，但本轮源码检索未看到它在 `src` 的实际导入；不能据依赖列表推定功能已使用。

## 8. 构建部署与验证边界

仓库有 `yarn.lock`，packageManager 写 Yarn 1.22.22，但 bootstrap/reinstall 等脚本混用 pnpm/npm；Node engines 标的是旧模板范围 `^12 || >=14`，部署 workflow 用 Node 16。迁移前应选定一套可复现 Node/包管理器版本并按锁安装，不能据 engines 判断新运行时已兼容。

开发：Vite HTTPS、host 全网卡、mkcert 插件、`.env.development.local` 可覆盖开发代理和接口；构建：`vite build` 再 `build/script/postBuild.ts` 输出配置，支持压缩、主题、PWA/legacy 等条件插件。[Vite server](/Users/alex/html/dating.saas/vite.config.ts:55)、[plugins](/Users/alex/html/dating.saas/build/vite/plugin/index.ts:18)、[postBuild](/Users/alex/html/dating.saas/build/script/postBuild.ts:8)

`.github/workflows/deploy.yml` 是沿用模板的自动部署配置，含删除 gh-pages 分支、模板 CNAME/身份等，搬迁时不能直接启用。[workflow](/Users/alex/html/dating.saas/.github/workflows/deploy.yml:57)

有 `type:check`、eslint/stylelint 与 `test:unit: jest` 脚本，但未找到 Jest 配置、实际单元/e2e 测试文件，manifest 也未声明 jest。现有名为 `test.html/tagstest.vue` 的文件并不是测试覆盖。当前没有 `node_modules`；本次未安装依赖或尝试构建，因此**不能宣称可直接构建成功**。静态字面 import 检查没有确认实际缺文件的活跃源码 import（初步候选分别是 `.d.ts` 与注释），也不等于通过 TypeScript/Vite 检查。

## 9. 直接迁移执行清单（下一阶段建议）

1. 固化旧前端当前工作区快照，保留已有 VxeTable → BasicTable 等改动；新位置以独立前端目录维护完整工具链，剔除 `dist`、历史压缩备份和不适用的模板发布 workflow。
2. 把 API/上传地址、品牌、地图配置、静态资源前缀改为新环境；隔离本地缓存命名空间，避免与旧站 token/菜单共用。
3. 先打通最小闭环：`app/info/init → user/login → profile → auth → 应用切换 → 租户/员工/角色 CRUD`。按实际前端契约做 HTTP 网关，再将请求转入 go-micro 服务。
4. 同步迁移/重建菜单资源数据：组件路径、权限字符串、应用 code、`server/controller/uri/method/action`；这与数据库初始化同样重要，仅迁移页面代码不能恢复菜单。
5. 为旧 HTTP DTO 建契约样本：成功/校验失败/HTTP 401、列表分页、关系对象与秒时间戳、文件上传、应用切换、空数据与 null；业务接口再逐模块对照。
6. 先补会话/应用上下文恢复、注销清理、上传链路等确定的兼容风险，再做依赖升级；不要把“整体复制”和“跨代升级 UI 框架”混成同一次交付。
7. 先启用已核实的 SaaS 平台页面；CRM/婚恋/呼叫中心按需要作为模块启用，隐藏或修复模板/未完成页面。
8. 在新副本做锁文件安装、type:check、build，再以测试后端验证登录/刷新/退出换账号、菜单按钮权限、租户上下文、应用切换、CRUD/分页/上传/静态资源。此阶段才给出“可运行迁移”结论。

## 10. 与后端探索需要对齐的问题

- 后端租户/单位/部门/应用四级上下文的业务含义和权限约束；平台管理员查看租户详情究竟用 payload tenant_id 还是 header 切换。
- 登录 token 的真实过期策略与 session 存储；前端不实现 refresh，并不能推断后端没有续期能力。
- auth 返回的资源 DTO、权限码/路由种子从哪里生成；应用 `code` 是否固定为 system/basic/crm/feelingRing。
- 上传 endpoint 与文件 DTO 是否与本前端版本匹配；历史是否存在另一个 axios 实现用于运行版本。
- 旧后端框架的 servers/routes 反射接口如何在新网关保留，避免资源管理失效。
- CRM/婚恋字段与枚举、通话集成及导入规则哪些应保持兼容，哪些不进入第一版 SaaS 底座。
