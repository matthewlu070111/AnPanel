# AnPanel

AnPanel 是一个面向单台 Linux 服务器的轻量监控与服务管理面板。后端使用 Go，React 管理端被嵌入同一个静态二进制；运行时不需要 Node.js、外部数据库或 PHP。

> 当前仓库是可运行的首版实现。生产部署前仍应在目标发行版 VM 中完成安装器和 Web 服务配置回滚测试，尤其是 CentOS 7、Ubuntu 18.04 与已有复杂 Nginx/Apache 配置的服务器。

## 已实现

- `anpanel web` 与 `anpanel agent` 双服务权限边界；root agent 只接受固定动作，不接受任意 Shell。
- SQLite WAL、Argon2id、强制首次改密、TOTP、CSRF、登录限速、会话撤销和审计日志。
- `/proc` 主机指标、历史保留、实时 WebSocket 仪表盘。
- 1 分钟七天、5 分钟九十天的分层指标，以及 SMTP/Webhook 阈值和恢复告警。
- Docker 容器生命周期，以及镜像、网络、卷、Compose 的受限 API。
- Nginx/Apache 虚拟主机发现、原始指令保留、原子更新、语法检查和 reload 失败回滚。
- certbot/acme.sh 域名绑定、证书失败回滚、IP 入口恢复和受限 Docker 容器终端。
- 组件检测与受兼容目录限制的一键安装任务。
- 中英文响应式管理界面、异步任务中心和高风险删除确认。
- amd64/arm64 静态交叉构建、systemd 单元、随机高位端口和安装源备份/验证/回滚。

## 本地构建

要求 Go 1.24+ 和 Node.js 22+：

```bash
make web
make test
make release VERSION=v0.1.0
```

输出位于 `dist/`。发布前必须给二进制生成 SHA-256；若使用 Ed25519 发布签名，将公钥文件路径通过 `ANPANEL_RELEASE_PUBLIC_KEY` 传给安装器。

## 安装

正式 Release 应上传以下同名资产：

- `anpanel-linux-amd64`、`anpanel-linux-arm64`
- 对应的 `.sha256`
- `install.sh`（由 `make installer` 生成的自包含版本）

```bash
curl -fsSL https://github.com/anpanel/anpanel/releases/latest/download/install.sh | sudo bash
```

可选参数：

```bash
sudo bash install.sh --region=cn
sudo bash install.sh --region=global --open-firewall
sudo bash install.sh --version=v0.1.0
```

安装完成后终端会显示随机端口和一次性的初始 `admin` 密码。首次登录必须修改用户名与密码。IP 模式按产品约定使用明文 HTTP，面板会持续显示风险提示。

## 恢复命令

```bash
anpanelctl show-port
anpanelctl recover-access
anpanelctl reset-admin
anpanelctl disable-totp
```

`anpanelctl` 是指向同一 AnPanel 二进制的符号链接。

## 安全边界

- Web 服务以 `anpanel` 普通用户运行，特权请求通过 `0660` Unix Socket 和随机 agent token 发往 root agent。
- Web 配置只能写入 `/etc/nginx`、`/etc/apache2`、`/etc/httpd`；Compose 文件只允许位于 `/etc/anpanel/compose`、`/opt` 或 `/srv`。
- 第三方 apt/yum 仓库不会被换源逻辑改写。旧系统的归档源只解决软件包可获取性，不代表仍有安全维护。
- 删除 Docker 卷、容器等破坏性操作需要界面二次确认；agent 同时禁用默认强制删除。

接口约定和当前限制见 [docs/API.md](docs/API.md) 与 [docs/ROADMAP.md](docs/ROADMAP.md)。
