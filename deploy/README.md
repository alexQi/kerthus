# 运行与配置

`compose.yaml` 仅提供本地依赖，不是生产部署模板。应用入口为 `cmd/saas`、`cmd/gateway`；前端生产产物为 `web/admin/dist`。所有配置从 `KERTHUS_*` 环境变量读取，可显式用 `KERTHUS_ENV_FILE` 加载一份 KEY=VALUE 文件，进程环境变量优先。配置文件不执行 shell 展开。

| 配置 | 用途 |
| --- | --- |
| MYSQL_DSN | MySQL DSN，启用 `clientFoundRows=true`，默认库 kerthus |
| SESSION_NAMESPACE | 数据集专用会话命名空间；切换含相同数字账号 ID 的数据集时使用不同值 |
| LEGACY_STATIC_URL | 可选的旧图片静态站点，旧相对 key 经网关跳转，新图片继续使用 MinIO |
| REDIS_ADDRESS / REDIS_PASSWORD | Redis 会话，token 以 SHA-256 后的 key 存储 |
| CONSUL_ADDRESS | 服务发现地址 |
| RPC_ADDRESS | 核心 gRPC 监听地址；非回环地址必须启用 TLS |
| RPC_GATEWAY_KEY | 网关专属服务凭据，至少 32 字符 |
| RPC_APP_KEYS | JSON 对象，app_code 到应用专属随机凭据；不可与网关或其他应用共用 |
| RPC_TLS_CA / RPC_TLS_CERT / RPC_TLS_KEY | 双向 TLS，最低 TLS 1.3；双方证书分别配置，服务证书 SAN 匹配发现地址 |
| HTTP_ADDRESS / CORS_ORIGINS | 网关地址与逗号分隔的精确允许来源 |
| STORAGE_ENDPOINT / STORAGE_ACCESS_KEY / STORAGE_SECRET_KEY / STORAGE_BUCKET | 私有对象桶连接 |
| STORAGE_TLS | 对象存储 HTTPS，`true`/`false` |
| STATIC_URL | 浏览器访问公开图片的 `/files` URL |
| ADMIN_PHONE / ADMIN_PASSWORD | 仅首次 seed 创建平台账号时使用 |

本地无 TLS 的 RPC 仅允许回环监听。跨主机部署启用双向 TLS；服务密钥仍用于区分平台网关与各应用权限，不能靠客户端传入 actor/tenant metadata 获权。Consul 目录属于可信控制面，正式环境需配置网络隔离及访问控制，租户不能登记 upstream 节点。

公开 HTTP 由受控 HTTPS 入口代理；限制请求频率、配置审计日志采集与备份、使用独立运行账号/迁移账号。这些生产设施及恢复演练不在本次本地底座闭环中。当前 Redis/Consul/MinIO 接线默认服务端本地或可信私网；尚未验证集群高可用、跨机部署或压测。

核心写事务首期使用平台级数据库行锁防止并发破坏管理员/授权约束，后续高并发优化需要保留等价隔离与测试。权限不缓存；每次请求从 SQL 校验有效身份、租户、开通与角色交集。应用端在线校验提供请求开始时的权限快照，不承诺跨应用数据库写入与同时发生的撤权处于同一事务。

图片 URL 为公开访问，仅适合头像、LOGO；不要用于合同等私有资料。上传失败记录暂保留为 pending，生产环境需要定期清理未完成对象/记录。应用数据销户清理由各应用独立负责，当前不执行跨应用删除。
