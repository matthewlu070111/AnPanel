# AnPanel API 与动作约定

浏览器接口统一位于 `/api/v1`。API 当前服务于内嵌管理界面，不承诺跨大版本保持客户端兼容；自动化调用应锁定 AnPanel 版本。

## 认证与请求保护

- 登录成功后使用 `HttpOnly` 会话 Cookie。
- 除登录和只读请求外，状态变更请求必须携带当前会话的 `X-CSRF-Token`。
- WebSocket 在建立连接时验证会话。
- 登录具有速率限制和连续失败锁定；修改凭据后可撤销已有会话。
- root agent 不监听 TCP，仅通过 `/run/anpanel/agent.sock` 接收携带身份和审计上下文的固定动作。

## 浏览器端点

以下路径均省略 `/api/v1` 前缀。

| 方法与路径 | 作用 |
| --- | --- |
| `POST /auth/login` | 使用密码及可选 TOTP 登录 |
| `POST /auth/logout` | 注销当前会话 |
| `GET /me` | 获取当前管理员和初始化状态 |
| `POST /me/change` | 修改管理员用户名与密码 |
| `POST /me/totp/setup` | 创建待确认的 TOTP 密钥 |
| `POST /me/totp/enable` | 校验验证码并启用 TOTP |
| `POST /me/totp/disable` | 禁用 TOTP |
| `GET /overview` | 最新指标、已检测服务和容器摘要 |
| `GET /metrics/history` | 查询分层监控历史 |
| `GET /ws/metrics` | 实时指标 WebSocket |
| `GET /services` | Nginx、Apache、Docker 和证书工具检测结果 |
| `GET /docker/containers` | 本机全部容器 |
| `GET /docker/inventory/{images,networks,volumes}` | Docker 对象清单 |
| `GET /ws/docker/terminal` | 受限容器终端 WebSocket |
| `GET /websites` | Nginx/Apache 虚拟主机及未识别原始配置块 |
| `POST /actions` | 创建异步系统任务 |
| `GET /tasks` | 查询任务及脱敏结果 |
| `GET /audits` | 查询操作审计记录 |
| `GET /alerts/rules` | 查询告警规则 |
| `POST /alerts/rules/update` | 创建或更新告警规则 |
| `POST /alerts/test` | 发送测试通知 |

## 异步动作

所有系统变更都通过 `POST /api/v1/actions` 创建任务：

```json
{
  "kind": "docker.container.restart",
  "resource": "container-id",
  "options": {}
}
```

任务状态固定为：

| 状态 | 含义 |
| --- | --- |
| `queued` | 已持久化，等待 agent 执行 |
| `running` | agent 正在执行 |
| `succeeded` | 操作成功 |
| `failed` | 操作失败且未产生可恢复变更 |
| `rolled_back` | 操作失败，已恢复上一版本 |

agent 当前允许的动作如下。除此之外的 `kind` 会被拒绝。

| 动作 | `resource` | 说明 |
| --- | --- | --- |
| `docker.container.start/stop/restart/delete` | 容器 ID 或名称 | 删除不强制且默认保留卷 |
| `docker.image.pull/delete` | 镜像名或 ID | 删除不启用 force |
| `docker.volume.create/delete` | 卷名 | 删除不启用 force |
| `docker.network.create/delete` | 网络名 | 创建时检测重名 |
| `docker.compose.up/down` | Compose YAML 绝对路径 | 路径必须位于允许目录 |
| `service.start/stop/restart` | 白名单 systemd 服务名 | 限 Nginx、Apache、Docker 和 AnPanel 服务 |
| `web.apply` | 受管配置文件路径 | 内容放在 `options.content`，应用前验证并支持回滚 |
| `web.reload` | `nginx` 或 `apache` | 验证配置后 reload |
| `package.install` | `nginx`、`apache`、`docker` 或 `certbot` | 受兼容目录和仓库状态限制 |
| `notification.configure` | 配置标识 | JSON 放在 `options.json`，凭据文件权限为 `0600` |
| `panel.bind_domain` | 面板入口 | 使用 `options.domain/server/tool/email` |
| `panel.unbind_domain` | 面板入口 | 撤销域名入口并恢复直接访问 |

## 错误与审计

- API 使用标准 HTTP 状态码，并返回可供界面展示的错误信息。
- 命令输出在保存前会过滤常见密码、Token 和 Authorization 字段，并限制日志长度。
- 每个变更任务记录调用身份、动作摘要、状态和时间；敏感凭据不应放入 `resource` 或自由文本字段。
- 删除、下线等高风险操作必须由界面完成二次确认，但 API 调用方仍需自行实现同等级保护。

最终白名单以 [`internal/agent/actions.go`](../internal/agent/actions.go) 中的实现为准。
