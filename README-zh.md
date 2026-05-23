# Pafu

[English][readme-en-link] | **简体中文**

拍打你的 MacBook，Pafu 会回应你。

Pafu 是一个 macOS 桌面互动小工具：通过 Apple Silicon 芯片的加速度传感器
检测笔记本上的物理拍打动作，并播放对应的语音反馈。后端是单文件 Go 二进制，
前端可以接 Next.js（或任何浏览器），通过本地 HTTP API 通信。

## 功能特性

- 基于 Apple Silicon 加速度计的实时拍打检测
- 内置语音包：`pain`、`sexy`、`halo`、`lizard`
- 自定义语音包：指定任意 MP3 目录
- 运行时切换语音包，无需重启后端
- 浏览器上传自定义语音包，自动持久化到本机
- 灵敏度、冷却时间、播放速度可调
- 音量自适应：轻拍小声、重拍大声
- 提供 HTTP REST + Server-Sent Events，便于浏览器前端接入
- 提供 stdio 模式，方便嵌入 Electron / Tauri 桌面壳

## 系统要求

- 基于 Apple Silicon 的 macOS（任何 M2 及以上型号的 M 系列芯片，或特定的
  M1 Pro 型号；其他 M1/A 系列芯片没有暴露加速度计）
- `sudo`（用于访问 IOKit HID 加速度传感器）
- Go 1.26+（从源码构建时）

## 从源码构建

```bash
git clone <你的 fork 或本地路径>
cd pafu
go build -o pafu .
```

可选：复制到系统路径

```bash
sudo cp pafu /usr/local/bin/pafu
```

## 使用方法

```bash
# 默认 pain 模式
sudo ./pafu

# 性感模式 — 根据拍打频率升级回应
sudo ./pafu --sexy

# 光环死亡音效
sudo ./pafu --halo

# Lizard 模式 — 拟人爬行类反应
sudo ./pafu --lizard

# 快速模式 — 更快轮询、更短冷却
sudo ./pafu --fast
sudo ./pafu --sexy --fast

# 自定义模式 — 播放你自己的 MP3 文件夹
sudo ./pafu --custom /path/to/mp3s

# 调整检测灵敏度（数值越低越敏感）
sudo ./pafu --min-amplitude 0.10
sudo ./pafu --min-amplitude 0.25

# 设置冷却时间（毫秒，默认 750）
sudo ./pafu --cooldown 600

# 播放速度倍数（默认 1.0）
sudo ./pafu --speed 0.7   # 更慢更深沉
sudo ./pafu --speed 1.5   # 更快
```

### 模式

- **Pain（默认）**：随机播放疼痛/抗议片段
- **Sexy（`--sexy`）**：在 5 分钟滚动窗口内统计拍打次数，60 级强度递增
- **Halo（`--halo`）**：随机播放光环死亡音效
- **Lizard（`--lizard`）**：随机/递增的爬行类回应
- **Custom（`--custom <dir>`）**：播放指定文件夹中的 MP3

### 检测调优

`--fast` 提供更敏捷的预设：更快的轮询（4ms vs 10ms）、更短的冷却
（350ms vs 750ms）、更高的灵敏度（0.18 vs 0.05）、更大的样本批次
（320 vs 200）。

`--min-amplitude` 与 `--cooldown` 永远会覆盖预设值。

### 灵敏度

`--min-amplitude`（默认 `0.05`）：

- 0.05 - 0.10：非常敏感，能识别轻拍
- 0.15 - 0.30：均衡
- 0.30 - 0.50：只有较重的拍打才会触发

数值表示触发声音所需的最小加速度幅度（g 力）。

## 浏览器前端（HTTP + SSE）

如果要接图形界面，让 Pafu 以 serve 模式运行：

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

API 监听在 `http://127.0.0.1:17878`：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/status` | 当前运行状态 |
| `POST` | `/api/pause` | 暂停回应 |
| `POST` | `/api/resume` | 恢复回应 |
| `POST` | `/api/set` | 修改运行参数 |
| `GET` | `/api/packs` | 列出语音包 |
| `POST` | `/api/packs/activate` | 激活某个语音包 |
| `POST` | `/api/packs/import` | 上传 MP3 创建用户语音包 |
| `DELETE` | `/api/packs/delete` | 删除用户语音包 |
| `GET` | `/api/events` | 实时事件流（SSE） |

监听只绑定 `127.0.0.1`，所以只有同一台 Mac 上的前端能访问。完整的接口
文档和 Next.js 集成示例见 `NEXTJS_INTEGRATION.md`，视觉设计规范见
`FRONTEND_DESIGN.md`。

用户上传的语音包会保存到：

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

通过 `sudo` 启动 Pafu 时，会自动从 `$SUDO_USER` 解析出真实用户的家目录，
确保文件落在你的账户下，而不是 `/var/root` 里。

## stdio 模式（Electron / Tauri）

```bash
sudo ./pafu --stdio
```

Pafu 会向 stdout 输出每行一个 JSON 事件，并从 stdin 读取 JSON 命令，
方便桌面外壳作为子进程调用。

## 作为系统服务运行

要让 Pafu 开机自启，可以创建 `LaunchDaemon` 配置。选一个模式：

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

> **注意：** 如果你将 Pafu 安装在其他位置（例如 `~/go/bin/pafu`），
> 请更新二进制路径。

加载并启动服务：

```bash
sudo launchctl load /Library/LaunchDaemons/dev.pafu.app.plist
```

由于 plist 位于 `/Library/LaunchDaemons` 且没有 `UserName` 字段，
`launchd` 会以 root 身份运行 Pafu，之后无需再加 `sudo`。

停止或卸载：

```bash
sudo launchctl unload /Library/LaunchDaemons/dev.pafu.app.plist
```

## 工作原理

1. 通过 IOKit HID 直接读取 Apple SPU 加速度计原始数据
2. 跑振动检测（STA/LTA、CUSUM、峰度、Peak/MAD）
3. 检测到显著撞击后，从当前激活语音包中播放一个 MP3
4. 可选 `--volume-scaling`：轻拍小声、重拍大声
5. 可选 `--speed`：调整播放速度和音调
6. 默认 750ms 响应冷却防止连发，可通过 `--cooldown` 调整

## 致谢

加速度数据读取与振动检测来源于
[olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer)，
拍打检测器实现参考自
[taigrr/spank](https://github.com/taigrr/spank)。

## 许可证

MIT

<!-- Links -->
[readme-en-link]: ./README.md
