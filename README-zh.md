<div align="center">

# Pafu

**为 Apple Silicon Mac 设计的反应式桌面伙伴。**
**拍打你的 MacBook —— Pafu 会回应你。**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Apple%20Silicon-black.svg)](#系统要求)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8.svg?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-experimental-orange.svg)](#路线图)

[English](./README.md) | [简体中文](./README-zh.md)

</div>

---

Pafu 是一个 macOS 桌面互动伙伴：通过 Apple Silicon 上的**内置加速度计**实时
感知物理拍打，并播放对应的语音反馈。后端是一个单文件 Go 二进制，前端可以接
Next.js（或任何浏览器）通过本地 HTTP API 通信。

> 它是个"压力释放阀"，不是生产力工具。拍它一下，看它怎么回应你，然后继续干活。

## 目录

- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [系统要求](#系统要求)
- [安装](#安装)
- [使用](#使用)
  - [模式](#模式)
  - [检测调优](#检测调优)
  - [灵敏度](#灵敏度)
- [浏览器前端（HTTP + SSE）](#浏览器前端http--sse)
- [stdio 模式（Electron / Tauri）](#stdio-模式electron--tauri)
- [作为系统服务运行](#作为系统服务运行)
- [项目结构](#项目结构)
- [工作原理](#工作原理)
- [开发](#开发)
- [路线图](#路线图)
- [致谢](#致谢)
- [许可证](#许可证)

## 核心特性

- **实时拍打检测**：基于 IOKit HID + Apple SPU 加速度计
- **四个内置语音包**：`pain`、`sexy`、`halo`、`lizard`
- **自定义语音包**：指定任意 MP3 目录即可
- **运行时切换**：无需重启后端即可切换语音包
- **浏览器上传**：用户自定义语音包，自动持久化到磁盘
- **可调参数**：灵敏度、冷却时间、播放速度、音量自适应
- **HTTP REST + Server-Sent Events**：方便浏览器前端（Next.js / 纯 HTML）接入
- **stdio JSON 模式**：方便嵌入 Electron / Tauri 桌面壳

## 快速开始

```bash
git clone <你的 fork>
cd pafu
go build -o pafu .
sudo ./pafu
```

拍一下 MacBook，应该听到一声 "ow!"。就这样。

如果要接浏览器前端，启动时加 `--serve`：

```bash
sudo ./pafu --serve
# API 在 http://127.0.0.1:17878
```

## 系统要求

| 依赖 | 说明 |
| --- | --- |
| Apple Silicon 上的 macOS | M2 及以上的 M 系列芯片，**或** M1 Pro 这一个特定型号。其他 M1 / A 系列没有暴露加速度计。 |
| `sudo` | 访问 IOKit HID 加速度计需要 root 权限。 |
| Go 1.26+ | 仅在从源码构建时需要。 |

## 安装

### 从源码构建

```bash
git clone <你的 fork>
cd pafu
go build -o pafu .
```

可选：复制到系统路径

```bash
sudo cp pafu /usr/local/bin/pafu
```

之后就可以省去 `./` 前缀：

```bash
sudo pafu --sexy
```

## 使用

```bash
# 默认 pain 模式
sudo ./pafu

# 性感模式 —— 根据拍打频率升级回应
sudo ./pafu --sexy

# 光环死亡音效
sudo ./pafu --halo

# Lizard 模式 —— 升级强度
sudo ./pafu --lizard

# 快速模式 —— 更快轮询、更短冷却
sudo ./pafu --fast
sudo ./pafu --sexy --fast

# 自定义模式 —— 播放你自己的 MP3 文件夹
sudo ./pafu --custom /path/to/mp3s

# 调整检测灵敏度（数值越低越敏感）
sudo ./pafu --min-amplitude 0.10
sudo ./pafu --min-amplitude 0.25

# 设置冷却时间（毫秒，默认 750）
sudo ./pafu --cooldown 600

# 播放速度倍数（默认 1.0）
sudo ./pafu --speed 0.7
sudo ./pafu --speed 1.5
```

### 模式

| 模式 | 命令行 | 行为 |
| --- | --- | --- |
| Pain *（默认）* | — | 随机播放疼痛/抗议片段 |
| Sexy | `--sexy` | 在 5 分钟滚动窗口内统计拍打次数，60 级强度递增 |
| Halo | `--halo` | 随机播放光环死亡音效 |
| Lizard | `--lizard` | 升级的爬行类回应 |
| Custom | `--custom <dir>` | 播放指定文件夹中的 MP3 |

### 检测调优

`--fast` 启用更敏捷的预设：

| 参数 | 默认 | `--fast` |
| --- | --- | --- |
| 轮询周期 | 10 ms | 4 ms |
| 冷却时间 | 750 ms | 350 ms |
| 阈值 | 0.05 | 0.18 |
| 样本批次 | 200 | 320 |

`--min-amplitude` 与 `--cooldown` 永远会覆盖预设值。

### 灵敏度

`--min-amplitude`（默认 `0.05`）：

| 数值范围 | 行为 |
| --- | --- |
| 0.05 – 0.10 | 非常敏感，能识别轻拍 |
| 0.15 – 0.30 | 均衡 |
| 0.30 – 0.50 | 只有较重的拍打才会触发 |

数值表示触发声音所需的最小加速度幅度（g 力）。

## 浏览器前端（HTTP + SSE）

启动时加 `--serve` 即可暴露本地 HTTP API：

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

API 基地址：`http://127.0.0.1:17878`

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/status` | 当前运行状态 |
| `POST` | `/api/pause` | 暂停回应 |
| `POST` | `/api/resume` | 恢复回应 |
| `POST` | `/api/set` | 修改运行参数（amplitude / cooldown / speed / volume scaling） |
| `GET` | `/api/packs` | 列出语音包 |
| `POST` | `/api/packs/activate` | 激活某个语音包 |
| `POST` | `/api/packs/import` | 上传 MP3 创建用户语音包 |
| `DELETE` | `/api/packs/delete` | 删除用户语音包 |
| `GET` | `/api/events` | 实时事件流（SSE） |

监听只绑定 `127.0.0.1`，所以只有同一台 Mac 上的前端能访问。

用户上传的语音包持久化路径：

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

通过 `sudo` 启动时，会自动从 `$SUDO_USER` 解析真实用户的家目录，文件不会落在
`/var/root` 下。

完整的接口文档、TypeScript 类型和 Next.js 集成示例见
[`NEXTJS_INTEGRATION.md`](./NEXTJS_INTEGRATION.md)。视觉设计规范见
[`FRONTEND_DESIGN.md`](./FRONTEND_DESIGN.md)。

## stdio 模式（Electron / Tauri）

```bash
sudo ./pafu --stdio
```

Pafu 会向 stdout 输出每行一个 JSON 事件，并从 stdin 读取 JSON 命令，方便桌面
外壳作为子进程调用。

## 作为系统服务运行

要让 Pafu 开机自启，可以创建 `LaunchDaemon`。选一个模式：

<details>
<summary>默认（Pain）模式</summary>

```bash
sudo tee /Library/LaunchDaemons/dev.pafu.app.plist > /dev/null << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.pafu.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/pafu</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/pafu.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/pafu.err</string>
</dict>
</plist>
EOF
```

</details>

<details>
<summary>性感模式</summary>

```bash
sudo tee /Library/LaunchDaemons/dev.pafu.app.plist > /dev/null << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.pafu.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/pafu</string>
        <string>--sexy</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/pafu.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/pafu.err</string>
</dict>
</plist>
EOF
```

</details>

<details>
<summary>光环模式</summary>

```bash
sudo tee /Library/LaunchDaemons/dev.pafu.app.plist > /dev/null << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.pafu.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/pafu</string>
        <string>--halo</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/pafu.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/pafu.err</string>
</dict>
</plist>
EOF
```

</details>

> **注意：** 如果你将 Pafu 安装在其他位置（例如 `~/go/bin/pafu`），请更新二进制
> 路径。

加载并启动服务：

```bash
sudo launchctl load /Library/LaunchDaemons/dev.pafu.app.plist
```

由于 plist 位于 `/Library/LaunchDaemons` 且没有 `UserName` 字段，`launchd`
会以 root 身份运行 Pafu，之后无需再加 `sudo`。

停止或卸载：

```bash
sudo launchctl unload /Library/LaunchDaemons/dev.pafu.app.plist
```

## 项目结构

```text
.
├── main.go               # CLI 入口、传感器循环、拍打检测
├── stdin_handler.go      # --stdio JSON 协议
├── http_server.go        # --serve REST + SSE 接口
├── pack_manager.go       # 语音包注册表 / 运行时激活包
├── pack_storage.go       # 用户上传语音包的持久化
├── audio/
│   ├── pain/             # 默认 "ow!" 反馈
│   ├── sexy/             # 升级语音包（60 个 MP3）
│   ├── halo/             # 光环死亡音效
│   └── lizard/           # Lizard 升级语音包
├── nix/                  # Nix flake & Home Manager 模块
├── .goreleaser.yaml      # 发布配置
├── README.md
├── README-zh.md
├── AGENTS.md             # 开发者 / AI agent 协作约定
├── NEXTJS_INTEGRATION.md # 浏览器前端集成指南
└── FRONTEND_DESIGN.md    # 视觉设计规范
```

## 工作原理

1. 通过 IOKit HID 直接读取 Apple SPU 加速度计原始数据
2. 跑振动检测算法：**STA/LTA**、**CUSUM**、**峰度**、**Peak / MAD**
3. 检测到显著撞击后，从当前激活语音包中播放一个 MP3
4. 可选 `--volume-scaling`：轻拍小声、重拍大声
5. 可选 `--speed`：调整播放速度和音调
6. 默认 750 ms 响应冷却防止连发，可通过 `--cooldown` 调整

## 开发

```bash
# 格式化
gofmt -w .

# 静态检查
go vet ./...

# 运行测试
go test ./...

# 构建
go build -o pafu .

# 通过 GoReleaser 发布（CI 中执行）
git tag v0.1.0
git push origin v0.1.0
```

仓库约定、代码模式以及给开发者 / AI agent 的协作建议，见
[`AGENTS.md`](./AGENTS.md)。

## 路线图

- [x] 后端：HTTP / SSE API
- [x] 后端：运行时切换语音包
- [x] 后端：用户上传语音包（浏览器上传）
- [ ] 前端：Next.js Pafu 控制面板（进行中）
- [ ] 反应式 Creature 动画（Framer Motion）
- [ ] WAV / M4A 解码器支持
- [ ] 原生菜单栏 / 浮窗（Tauri 外壳）
- [ ] 启动间持久化设置

## 致谢

- 加速度计读取与振动检测算法参考自
  [olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer)
- 原始的拍打检测器实现 fork 自
  [taigrr/spank](https://github.com/taigrr/spank)

## 许可证

[MIT](./LICENSE) © Pafu contributors
