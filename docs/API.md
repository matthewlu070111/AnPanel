# API 概览

所有浏览器 API 位于 `/api/v1`，使用 HttpOnly 会话 Cookie。除登录与只读请求外，变更请求必须携带登录响应中的 `X-CSRF-Token`。

| 路径 | 作用 |
| --- | --- |
| `POST /auth/login` | 密码及可选 TOTP 登录 |
| `POST /me/change` | 修改首个管理员账号与密码 |
| `POST /me/totp/setup`、`/enable`、`/disable` | TOTP 生命周期 |
| `GET /overview` | 最新指标、组件和容器摘要 |
| `GET /metrics/history` | 监控历史 |
| `GET /ws/metrics` | 鉴权实时指标 WebSocket |
| `GET /services` | Nginx、Apache、Docker、证书工具检测 |
| `GET /docker/containers` | 本机全部容器 |
| `GET /docker/inventory/{images,networks,volumes}` | Docker 资源清单 |
| `GET /websites` | 导入的 Nginx/Apache 虚拟主机 |
| `POST /actions` | 创建异步受限系统任务 |
| `GET /tasks`、`/audits` | 任务及审计记录 |
| `GET /alerts/rules`、`POST /alerts/rules/update` | 告警规则管理 |
| `POST /alerts/test` | 发送测试通知 |

`POST /actions` 请求：

```json
{"kind":"docker.container.restart","resource":"container-id","options":{}}
```

状态固定为 `queued`、`running`、`succeeded`、`failed`、`rolled_back`。允许动作定义在 `internal/agent/actions.go`；agent 不提供通用命令执行接口。
