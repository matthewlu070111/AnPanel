# 构建与发布

仓库包含两条 GitHub Actions 工作流：

- `ci.yml`：在 push 和 pull request 上执行前端构建、Go 测试与 Linux 交叉编译检查。
- `release.yml`：成功完成 `ci` 后自动构建并发布，也支持手动运行。

自动发布只接收成功的 `push` CI，不接收 pull request CI。普通提交会生成 `build-{commit_id}` prerelease；如果该提交带有 `v*` 标签，则生成对应的正式 Release。

默认分支每次 prerelease 发布还会更新 `prerelease-latest` 中的安装器，因此最新成功构建始终可以通过固定地址安装：

```bash
curl -qfsSL https://github.com/matthewlu070111/anpanel/releases/download/prerelease-latest/install.sh | sudo bash
```

GitHub 要求使用 `workflow_run` 的工作流文件已经存在于默认分支，因此首次启用时需要先将 `release.yml` 合并到默认分支。

## 手动运行

在 GitHub 仓库中打开 **Actions → build-release → Run workflow**。`version` 可以留空，此时版本号为 `build-<commit>`；也可以填写符合以下规则的版本：

```text
v0.1.0
v0.1.0-rc.1
```

未填写 `version` 时，工作流使用当前提交的前七位 ID 创建 `build-{commit_id}` prerelease，例如 `build-a1b2c3d`。填写版本时会以该名称创建 Release，其中 `build-*` 及带 `alpha`、`beta`、`rc` 的版本仍标记为 prerelease。所有方式都会在该次运行的 **Artifacts** 区域保留构建文件。

每个 Release 中的 `install.sh` 都会内嵌该次构建版本。直接下载 prerelease 的安装脚本时，它默认安装同一个 `build-*` 版本，而不是回退到 latest 稳定版；仍可用 `--version=...` 或 `ANPANEL_VERSION=...` 显式覆盖。

例如安装指定的自动构建版本：

```bash
curl -qfsSL https://github.com/matthewlu070111/anpanel/releases/download/build-a1b2c3d/install.sh | sudo bash
```

## 创建正式 Release

创建并推送标签：

```bash
git tag -a v0.1.0 -m "AnPanel v0.1.0"
git push origin v0.1.0
```

标签 push 首先运行 `ci.yml`。只有 CI 成功，`release.yml` 才会识别指向该提交的 `v*` 标签并创建正式 Release。工作流会：

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
