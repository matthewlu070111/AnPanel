# 构建与发布

仓库包含两条 GitHub Actions 工作流：

- `ci.yml`：在 push 和 pull request 上执行前端构建、Go 测试与 Linux 交叉编译检查。
- `release.yml`：手动运行时上传构建产物；推送 `v*` 标签时同时创建 GitHub Release。

## 手动构建产物

在 GitHub 仓库中打开 **Actions → build-release → Run workflow**。`version` 可以留空，此时版本号为 `dev-<commit>`；也可以填写符合以下规则的版本：

```text
v0.1.0
v0.1.0-rc.1
```

完成后可在该次运行的 **Artifacts** 区域下载 `amd64`、`arm64` 二进制、自包含安装器和 SHA-256 文件。手动运行不会创建 Release。

## 创建正式 Release

确认默认分支 CI 通过后创建并推送标签：

```bash
git tag -a v0.1.0 -m "AnPanel v0.1.0"
git push origin v0.1.0
```

工作流会：

1. 安装锁定的前端依赖并运行 lint/build。
2. 运行 `go vet`、`go test` 和安装脚本语法检查。
3. 构建 Linux `amd64`、`arm64` 静态二进制。
4. 生成逐文件校验和、`SHA256SUMS` 和自包含 `install.sh`。
5. 上传 Actions artifact，并创建带自动发行说明的 GitHub Release。

重复运行同一标签时会覆盖该 Release 中的同名资产，便于恢复失败的上传；已公开使用的版本不应重新构建或替换。

## 可选 Ed25519 签名

生成并妥善保存 Ed25519 密钥：

```bash
openssl genpkey -algorithm ED25519 -out anpanel-release-private.pem
openssl pkey -in anpanel-release-private.pem -pubout -out anpanel-release-public.pem
```

将私钥完整内容保存为 GitHub Actions repository secret：

```text
ANPANEL_RELEASE_PRIVATE_KEY
```

配置后，工作流会为两个二进制额外生成 `.sig`。私钥只写入 runner 临时目录，不进入 artifact 或 Release。安装时通过本地公钥启用验证：

```bash
sudo ANPANEL_RELEASE_PUBLIC_KEY=/etc/anpanel/release-public.pem bash install.sh
```

SHA-256 始终验证下载完整性；签名用于验证发布者身份。生产发布应固定分发公钥并记录指纹，避免从与二进制相同的未验证渠道临时下载公钥。

## 本地复现

环境要求为 Go 1.24+、Node.js 22+、GNU Make、Bash 和 `sha256sum`：

```bash
npm ci --prefix web
make release VERSION=v0.1.0
cd dist && sha256sum -c anpanel-linux-amd64.sha256
cd dist && sha256sum -c anpanel-linux-arm64.sha256
```

自包含安装器由 `scripts/build-installer.sh` 将换源库嵌入主安装脚本生成。不要直接把开发目录中的 `scripts/install.sh` 作为 Release 资产上传。
