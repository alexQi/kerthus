# Kerthus 管理前端

复用 `/Users/alex/html/dating.saas` 的 Vue 3 / Vben / Ant Design Vue 页面、布局和共享组件，原始项目未改动。只保留 SaaS 基础平台：登录与个人资料、应用工作台、平台租户/应用/资源/开通管理，以及租户成员/组织/岗位/角色/审计。没有迁入 CRM、婚恋业务或呼叫中心。

## 本地运行

先按仓库根目录说明启动基础设施、迁移与初始化数据，再启动 SaaS RPC 和 HTTP 网关。

```sh
cd web/admin
yarn install --frozen-lockfile --ignore-scripts
yarn dev
```

开发地址 `http://127.0.0.1:15173`，API 和上传地址 `http://127.0.0.1:18080`。登录账号和本地生成的密码见仓库 `.local/saas.env`，不要复制到前端环境文件或提交版本库。

默认配置在 `.env.development`，生产示例在 `.env.production`。所有 `VITE_GLOB_*` 变量都会公开到浏览器，不能存放密钥。默认使用本地 HTTP，不会自动申请证书、初始化 SIP、调用旧业务服务器或运行旧部署脚本。

```sh
yarn type:check
yarn build
yarn preview:dist --host 127.0.0.1 --port 15173
```

构建产物是 `dist`；生产环境修改 `.env.production` 中的公开网关地址再构建。也可修改产物中的 `_app.config.js` 公开运行配置。部署静态资源时需保留该文件，页面采用 hash 路由。

## 新增应用

动态菜单和首页由 SaaS 接口及应用 manifest 提供。新增已打包应用的页面放入 `src/views/apps/<appCode>`，然后注册匹配 component 的 manifest，并重新构建前端。当前复用单一管理前端，没有引入微前端运行时。

## 接口请求规范

业务接口统一在 `src/api` 定义，由 Axios 请求拦截器注入公共鉴权和租户上下文。页面与组件只调用 API 方法，不直接拼接 URL 或请求头。Agent SSE 等浏览器原生流式请求也必须封装在 `src/api`，详情见[前端接口请求规范](../docs/architecture/frontend-api-request-standard.md)。

## 已修正的迁移差异

- 首页优先使用当前应用 manifest 声明且用户可访问的页面，否则进入首个授权页面；没有可访问页面时进入个人设置并提示联系管理员授权。切换应用与退出清理动态路由、菜单、权限和页签。
- 成员启停使用租户成员接口，避免误改跨租户的全局账号状态；租户普通管理员不能编辑已有全局账号资料。
- 平台管理员重置密码要求显式输入；租户审核使用 POST，可设置新管理员初始密码。
- 上传请求带会话和租户上下文；目前后端仅接收图片，用于头像和标志。
- 资源关联按 HTTP 方法和路径区分，取消关联表分页，避免保存时丢失其它页面的关联。
- 清除无后端实现的行业/定位/短信注册等接口和入口；地区数据来自 `/system/config/districts`，需要 `id/name/parent_id/has_children`。
- 移除未纳入基础底座模型的身份证、HR在职状态等表单项；保留原视觉设计。

生产构建和 `vue-tsc` 类型检查均已通过。登录页面已通过浏览器渲染检查，后端联调结果见仓库根目录运行说明。
