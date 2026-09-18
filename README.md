# SingBox WebUI（第一版）

由 GUI.for.SingBox 改造，Vue 前端 + Go HTTP 服务，运行不依赖 Wails、桌面或 WebView。
原项目许可证见 LICENSE。原版目录和正在使用的 GUI 安装目录未改动。

## 已实现

- 管理员密码登录、HttpOnly 会话、退出登录；所有 API、SSE、核心 HTTP/WebSocket 代理统一鉴权。
- 保留配置/订阅/规则集手动管理、插件管理、配置生成和插件交互界面。
- Go 管理 sing-box：先 check 配置，再应用；启动失败尝试恢复旧配置；退出浏览器不停止核心。
- 保存最后成功应用的最终配置和运行意图；服务重启恢复此前运行中的核心。显式停止后不会自动恢复。
- 原版 `Plugins` / `Vue` 接口继续使用，文件、网络、进程接口改用 HTTP；文件路径与权限行为保持原版，包括绝对路径。
- 原版皮肤、链式代理、IP 查询接口保留；当前 sing-box 仪表板插件有专用同源地址适配。

移除桌面托盘/窗口控制、GUI 更新、系统代理/DNS自动设置；第一版不加载定时任务，也不支持插件动态 HTTP 服务器。
无浏览器时可以继续运行或恢复最终配置，但不执行订阅更新、配置重新生成、插件任务和启动/关闭 JS 钩子。

## 构建

需要 Go 1.27+、Node.js 22.12+（推荐 24）、pnpm 10。沿用原项目当前依赖版本。

```sh
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
go test ./...
CGO_ENABLED=0 go build -trimpath -o singbox-webui .
```

也可执行 `bash scripts/build-linux.sh amd64`（或 arm64），生成 `dist/` 下的完整部署包。
先构建前端，再编译 Go；`frontend/dist` 通过 Go embed 嵌入可执行文件。部署仅需 WebUI 可执行文件，sing-box 可在核心设置页下载，无需外置前端目录或 Node.js。前端更新后需重新编译 Go。

## 启动

启动 WebUI 后，在「设置 → 核心设置」下载 Stable 或 Alpha 核心，再创建配置并启动：

```sh
./singbox-webui \
  -listen 0.0.0.0:9090
```

访问 `http://服务器地址:9090`。密码从 `data/user.yaml` 顶层字段 `webuiPassword` 读取。首次缺少该字段或值为空时，自动生成随机密码并保存；查看该文件即可获取初始密码。已有其他设置会保留。

```yaml
webuiPassword: "你的登录密码"
```

修改密码后重启 WebUI 生效；不再读取 `WEBUI_PASSWORD` 环境变量。
`-data-dir` 沿用原版基础目录语义，默认值为 `.`。传 `-data-dir ./` 或省略参数时，实际数据位于当前工作目录的 `./data/`；传 `/var/lib/singbox-webui` 时，实际数据位于 `/var/lib/singbox-webui/data/`。
登录有效期 30 天，服务重启使所有会话失效。密码保存在上述 YAML 文件中。

核心沿用原版目录：基础目录下 `data/sing-box/sing-box`（Stable）或 `sing-box-latest`（Alpha）；Windows 自动加 `.exe`。无需设置核心路径。核心设置支持检查版本、下载更新、备份回退与分支切换，下载按服务器系统和架构选择资源。更新后点击重启生效。
核心下载需要服务器能够访问 GitHub；也可手动将可执行文件放入上述位置。
启动参数固定为 `run --disable-color -c .../config.json -D .../data/sing-box`，保留配置中的核心环境变量；原版自定义启动参数不应用。

使用 HTTPS 反向代理时加 `-secure-cookie`，示例见 `deploy/nginx.conf.example`。
仅 HTTP 访问时不要加该参数，否则浏览器不会发送会话 Cookie。剪贴板读取要求 HTTPS，复制支持浏览器兼容回退。

## systemd

1. 解压到 `/opt/singbox-webui`，首次登录后从核心设置下载 sing-box。
2. 创建 `singbox-webui` 系统用户及组，保证 WebUI 可执行文件可执行，数据目录可写。
3. 首次启动后查看 `/var/lib/singbox-webui/data/user.yaml` 中的 `webuiPassword`；该文件应仅允许服务用户和管理员读取。
4. 将 `deploy/singbox-webui.service` 放入 `/etc/systemd/system/`，按实际路径和 HTTPS 部署调整 ExecStart。
5. 执行 `systemctl daemon-reload` 和 `systemctl enable --now singbox-webui`。

示例服务通过 StateDirectory 自动建立持久化目录，默认配合 HTTPS 反向代理。
普通代理功能可用非 root 用户运行。TUN 所需设备和网络权限应另行配置，本版未验证透明代理部署。WebUI 不提供桌面提权弹窗；更新核心后如使用文件 capabilities，需重新配置相关权限。
同一数据目录只启动一个 WebUI 服务，不再让其他服务同时管理同一个核心进程。

## 插件与数据迁移

复制前先备份。可迁移 `data/plugins.yaml`、`data/plugins/`、插件对应的 `data/third/`、配置/订阅/规则集业务数据。
不要直接复制 Windows 核心、PID、日志或运行缓存；检查绝对路径、接口名和平台相关配置。
旧 user.yaml 可以读取，但桌面自启动、系统代理/DNS、自动重启和更新开关会在 WebUI 初始化时关闭。

- 插件 ReadFile/WriteFile 操作服务器文件，不是浏览器电脑文件；服务器进程的操作系统权限仍然有效。
- Plugins.HttpGet 使用服务器网络；自动 IP 查询得到服务器请求出口。
- 链式代理在点击启动/重新应用时处理配置；服务自身重启使用已处理好的最终配置。
- 皮肤插件在每个浏览器执行，资源来自服务器；UI 插件无需后端 JS 引擎。
- sing-box 仪表板仍需支持 API service 的核心。首次打开后，使用工具栏“复制 URL”和“复制密钥”填写连接参数；URL 指向 `/api/dashboard`。
- 仪表板资源由核心提供；无法联网下载时，可将已有 dashboard 资源放入 `data/sing-box/dashboard/`。
- 桌面 OpenDir/OpenURI 改为目录/文本预览；浏览器打开链接在访问者电脑执行。
- 插件任意命令执行兼容接口保留在管理员鉴权后；StartServer 等未支持接口返回明确错误。

仍采用原版 YAML 整文件写回，不支持多管理员同时编辑合并。核心配置应用带 revision 检查，避免旧页面覆盖新的已应用核心版本。
插件为可信管理员代码，不提供不可信代码沙箱。

## 开发与验证

后端默认监听 127.0.0.1:9090。`pnpm --dir frontend dev` 通过 Vite 将 /api 与 WebSocket 代理到后端。
Linux amd64/arm64 可交叉编译；Linux 实际服务、TUN和外部 IP 查询需在部署环境验证。
