<div align="center">

# Pafu

**A reactive desktop creature for Apple Silicon Macs.**
**Slap your MacBook — Pafu yells back.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Apple%20Silicon-black.svg)](#requirements)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8.svg?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-experimental-orange.svg)](#roadmap)

[English](./README.md) | [简体中文](./README-zh.md)

</div>

---

Pafu is a macOS desktop companion that listens to the **built-in accelerometer**
on Apple Silicon laptops, detects physical hits in real time, and plays back
voice responses. The backend is a single Go binary; an optional Next.js
frontend can render the creature in a browser.

> Designed as a *release valve*, not a productivity tool. Hit your laptop,
> watch it react, get on with your day.

## Table of Contents

- [Highlights](#highlights)
- [Quickstart](#quickstart)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
  - [Modes](#modes)
  - [Detection tuning](#detection-tuning)
  - [Sensitivity](#sensitivity)
- [Browser frontend (HTTP + SSE)](#browser-frontend-http--sse)
- [Stdio mode (Electron / Tauri)](#stdio-mode-electron--tauri)
- [Running as a service](#running-as-a-service)
- [Project structure](#project-structure)
- [How it works](#how-it-works)
- [Development](#development)
- [Roadmap](#roadmap)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Highlights

- **Real-time impact detection** powered by IOKit HID + Apple SPU accelerometer
- **Four built-in voice packs** — `pain`, `sexy`, `halo`, `lizard`
- **Custom voice packs** from any folder of MP3 files
- **Hot-swappable packs** — switch at runtime without restarting the backend
- **Browser-side upload** for user-imported voice packs (persisted on disk)
- **Tunable sensitivity, cooldown, playback speed, volume scaling**
- **HTTP REST + Server-Sent Events** for browser frontends (Next.js, plain HTML)
- **Stdio JSON mode** for embedding in Electron / Tauri shells

## Quickstart

```bash
git clone <your fork>
cd pafu
go build -o pafu .
sudo ./pafu
```

Slap the MacBook. You should hear an "ow!". That's it.

For the browser frontend, run the backend with `--serve`:

```bash
sudo ./pafu --serve
# API at http://127.0.0.1:17878
```

## Requirements

| Dependency | Notes |
| --- | --- |
| macOS on Apple Silicon | M-series M2 or newer, **or** the M1 Pro SKU specifically. Other M1 / A-series chips do not expose the accelerometer. |
| `sudo` | Required for IOKit HID accelerometer access. |
| Go 1.26+ | Only when building from source. |

## Installation

### Build from source

```bash
git clone <your fork>
cd pafu
go build -o pafu .
```

Optionally install system-wide:

```bash
sudo cp pafu /usr/local/bin/pafu
```

After that you can drop the `./` prefix:

```bash
sudo pafu --sexy
```

## Usage

```bash
# Default pain pack
sudo ./pafu

# Sexy mode — escalating responses
sudo ./pafu --sexy

# Halo death sounds
sudo ./pafu --halo

# Lizard mode — escalating reptilian responses
sudo ./pafu --lizard

# Faster polling and shorter cooldown
sudo ./pafu --fast
sudo ./pafu --sexy --fast

# Play your own MP3 folder
sudo ./pafu --custom /path/to/mp3s

# Adjust detection sensitivity (lower = more sensitive)
sudo ./pafu --min-amplitude 0.10
sudo ./pafu --min-amplitude 0.25

# Cooldown between responses, in milliseconds (default 750)
sudo ./pafu --cooldown 600

# Playback speed multiplier (default 1.0)
sudo ./pafu --speed 0.7
sudo ./pafu --speed 1.5
```

### Modes

| Mode | Flag | Behavior |
| --- | --- | --- |
| Pain *(default)* | — | Random pain / protest clips |
| Sexy | `--sexy` | Escalates with slap frequency, 60 levels in a 5-minute rolling window |
| Halo | `--halo` | Random Halo death sounds |
| Lizard | `--lizard` | Escalating reptilian responses |
| Custom | `--custom <dir>` | Plays MP3 files from any folder you point at |

### Detection tuning

`--fast` enables a more responsive profile:

| Param | Default | `--fast` |
| --- | --- | --- |
| Polling interval | 10 ms | 4 ms |
| Cooldown | 750 ms | 350 ms |
| Threshold | 0.05 | 0.18 |
| Sample batch | 200 | 320 |

`--min-amplitude` and `--cooldown` always override the preset.

### Sensitivity

`--min-amplitude` (default `0.05`):

| Range | Behavior |
| --- | --- |
| 0.05 – 0.10 | Very sensitive, picks up light taps |
| 0.15 – 0.30 | Balanced |
| 0.30 – 0.50 | Only firm slaps register |

The value is the minimum acceleration amplitude in g-force.

## Browser frontend (HTTP + SSE)

Run Pafu with `--serve` to expose the local HTTP API:

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

API base URL: `http://127.0.0.1:17878`

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/status` | Current runtime status |
| `POST` | `/api/pause` | Pause responses |
| `POST` | `/api/resume` | Resume responses |
| `POST` | `/api/set` | Update settings (amplitude / cooldown / speed / volume scaling) |
| `GET` | `/api/packs` | List voice packs |
| `POST` | `/api/packs/activate` | Activate a voice pack |
| `POST` | `/api/packs/import` | Upload MP3s as a new user pack |
| `DELETE` | `/api/packs/delete` | Delete a user-imported pack |
| `GET` | `/api/events` | Live event stream (SSE) |

The listener binds to `127.0.0.1` only — frontends must run on the same Mac.

User-imported voice packs are persisted to:

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

When `pafu` is started with `sudo`, the home directory is resolved from
`$SUDO_USER`, so files always land under your account, not `/var/root`.

For full request / response schemas, TypeScript types and a Next.js
walkthrough, see [`NEXTJS_INTEGRATION.md`](./NEXTJS_INTEGRATION.md). For the
visual design system see [`FRONTEND_DESIGN.md`](./FRONTEND_DESIGN.md).

## Stdio mode (Electron / Tauri)

```bash
sudo ./pafu --stdio
```

Pafu writes one JSON event per line to stdout and accepts JSON commands on
stdin. Intended for desktop shells that spawn Pafu as a subprocess.

## Running as a service

To start Pafu at boot, install a `LaunchDaemon`. Pick a mode:

<details>
<summary>Default (pain) mode</summary>

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
<summary>Sexy mode</summary>

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
<summary>Halo mode</summary>

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

> **Note:** update the binary path if you installed Pafu elsewhere
> (for example `~/go/bin/pafu`).

Load and start:

```bash
sudo launchctl load /Library/LaunchDaemons/dev.pafu.app.plist
```

Because the plist sits under `/Library/LaunchDaemons` and no `UserName` key
is set, `launchd` runs Pafu as root, so no extra `sudo` is needed afterwards.

Stop or unload:

```bash
sudo launchctl unload /Library/LaunchDaemons/dev.pafu.app.plist
```

## Project structure

```text
.
├── main.go               # CLI entry, sensor loop, slap detection
├── stdin_handler.go      # --stdio JSON protocol
├── http_server.go        # --serve REST + SSE endpoints
├── pack_manager.go       # voice pack registry, runtime active pack
├── pack_storage.go       # user-uploaded pack persistence
├── audio/
│   ├── pain/             # default "ow!" responses
│   ├── sexy/             # escalating responses (60 MP3s)
│   ├── halo/             # Halo death sounds
│   └── lizard/           # lizard escalation pack
├── nix/                  # Nix flake & Home Manager module
├── .goreleaser.yaml      # release configuration
├── README.md
├── README-zh.md
├── AGENTS.md             # contributor / AI agent guidelines
├── NEXTJS_INTEGRATION.md # browser frontend integration guide
└── FRONTEND_DESIGN.md    # visual design specification
```

## How it works

1. Reads raw accelerometer data via IOKit HID (Apple SPU sensor)
2. Runs vibration detection algorithms — **STA/LTA**, **CUSUM**, **kurtosis**,
   **peak / MAD**
3. When a significant impact passes the threshold, plays an MP3 from the
   currently active voice pack
4. Optional volume scaling (`--volume-scaling`) makes light taps quieter and
   hard slaps louder
5. Optional speed control (`--speed`) adjusts playback speed and pitch
6. A 750 ms cooldown prevents rapid-fire responses, adjustable via
   `--cooldown`

## Development

```bash
# Format
gofmt -w .

# Lint
go vet ./...

# Run tests
go test ./...

# Build
go build -o pafu .

# Tagged release (via GoReleaser, run inside CI)
git tag v0.1.0
git push origin v0.1.0
```

See [`AGENTS.md`](./AGENTS.md) for repository conventions, code patterns and
guidance for contributors (and AI agents) working on this codebase.

## Roadmap

- [x] Backend: HTTP / SSE API
- [x] Backend: runtime voice pack switching
- [x] Backend: user-uploaded voice packs (browser upload)
- [ ] Frontend: Next.js Pafu Control Panel (in progress)
- [ ] Reactive creature animations (Framer Motion)
- [ ] WAV / M4A decoder support
- [ ] Native menu bar / floating widget (Tauri shell)
- [ ] Settings persistence between launches

## Acknowledgements

- Sensor reading and vibration detection based on
  [olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer)
- Original slap detector implementation forked from
  [taigrr/spank](https://github.com/taigrr/spank)

## License

[MIT](./LICENSE) © Pafu contributors
