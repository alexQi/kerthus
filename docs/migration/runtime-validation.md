> 后续更新：本地 Beehive 数据已迁入并切换，见 [实际迁入记录](legacy-data-import.md)。本文保留此前独立新库阶段的验证范围。

# 首期集成验收

2026-09-11，在当前工作区本地环境执行。源码已实现并验证首期底座，未导入旧生产库。

## 通过的检查

| 检查 | 实际结果 |
| --- | --- |
| `go test ./...` / `go vet ./...` | 全部通过；此项默认跳过无 DSN 的数据库测试 |
| `python3 scripts/test-integration.py` | 20 组真实 MySQL 测试启用 race 通过；本次整包核心耗时 44.638 秒，另见核心验证记录 |
| 应用升级停用回归 | 停用应用升级后仍停用；显式启用恢复访问 |
| `scripts/check-boundaries.py` | 应用与平台实现依赖边界通过 |
| 清单校验 | system v2：33 资源/35 操作；basic v2：19 资源/27 操作 |
| 三个命令编译 | gateway / saas / saasctl 成功 |
| 前端类型与构建 | 最终竞态修复版本：vue-tsc 17.16 秒、生产构建 39.29 秒，均通过 |
| HTTP 冒烟 | 真实 HTTP→go-micro gRPC→MySQL/Redis：健康、登录、Profile/Auth、菜单、基础查询、草稿创建/删除、退出撤销通过 |
| 对象存储 | 已认证 multipart 上传测试 PNG，经 CreateUpload/MinIO/ConfirmUpload 后按公开 URL 读取，字节及 Content-Type 一致 |
| 应用代理 | 注册节点的正确路由、伪造 metadata 移除、未授权不转发、无可用服务返回503，HTTP httptest/内存 registry 通过 |
| 浏览器 | Chrome/Playwright 真实表单登录与应用切换通过；12 个页面无 pageerror 或失败 API envelope |

## 浏览器页面

平台：租户、应用、资源、租户应用授权、全局账号、租户详情、开通应用。切换到企业管理后：成员、组织、岗位、角色、审计。平台/企业首页均由授权菜单加载；切换确认后实际进入 `/basic/dashboard`。

本轮联调修复了旧应用类型字段缺失、租户 register_type 不匹配、日期字符串被提前转为0、上传应返回字符串 key、清页签与新首页导航竞争等问题。浏览器脚本针对真实导航与接口响应，不等同于每个编辑弹窗都完成了人工操作；核心事务和安全边界由独立数据库测试覆盖。

## 复现

依赖启动及 seed 后执行 `make test integration`；开启核心与网关后执行 `make smoke smoke-upload`。`smoke-api.py` 创建一个未审核草稿并删除；`smoke-upload.py` 在开发对象存储保留一个公开测试像素，不修改已有图片。测试脚本只读取本地随机凭据，不打印密码/token。

`smoke-browser.cjs` 需要 Playwright 模块及 Chrome，默认 require('playwright')，也可用 `KERTHUS_PLAYWRIGHT_PATH` 指向已安装模块。运行时读取 `.local/saas.env`，访问前端15173；失败截图与首页截图保存在被 Git 忽略的 `.local/`。

## 尚未执行的部分

- 旧生产数据迁移、完整行政区划字典导入；没有旧库快照和完整 DDL。
- 具体新应用的真实独立部署。当前已有注册/路由/客户端边界和测试，没有引入任何虚构业务应用。
- 生产压测、集群高可用、备份恢复演练、外部 HTTPS 入口与跨机 mTLS 部署验收。

权限细节与当前管理面语义见 [核心验证记录](implementation-validation.md)，本地配置及部署边界见 [部署说明](../../deploy/README.md)。
