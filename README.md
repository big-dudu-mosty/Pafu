# Pafu

**English** | [简体中文][readme-zh-link]

Slap your MacBook, Pafu yells back.

Pafu is a macOS desktop creature that detects physical hits on Apple Silicon
laptops via the built-in accelerometer and plays back voice responses. It
ships as a single binary for the backend, with an optional Next.js frontend
that talks to it over a local HTTP API.

## Features

- Real-time slap detection via the Apple Silicon accelerometer
- Built-in voice packs: `pain`, `sexy`, `halo`, `lizard`
- Custom voice packs from any folder of MP3 files
- Runtime voice pack switching with no backend restart
- Browser upload for user voice packs (saved to your home directory)
- Adjustable sensitivity, cooldown and playback speed
- Volume scaling: lighter slaps play softer, harder slaps play louder
- HTTP REST + Server-Sent Events API for browser frontends
- Stdio mode for embedding in Electron / Tauri shells

## Requirements

- macOS on Apple Silicon (any M-series chip M2 or greater, or the M1 Pro SKU
  specifically; other M1/A-series chips do not expose the accelerometer)
- `sudo` for IOKit HID accelerometer access
- Go 1.26+ to build from source

## Build from source

```bash
git clone <your fork or local path>
cd pafu
go build -o pafu .
```

Optionally install it system-wide:

```bash
sudo cp pafu /usr/local/bin/pafu
```

## Usage

```bash
# Default pain pack
sudo ./pafu

# Sexy mode (escalating responses)
sudo ./pafu --sexy

# Halo death sounds
sudo ./pafu --halo

# Lizard mode (escalating intensity)
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

- **Pain** (default): random pain/protest clips
- **Sexy** (`--sexy`): escalates with slap frequency, 60 levels in a 5-minute
  rolling window
- **Halo** (`--halo`): random Halo death sounds
- **Lizard** (`--lizard`): escalating reptilian responses
- **Custom** (`--custom <dir>`): plays MP3 files from any folder you point at

### Detection tuning

Use `--fast` for a more responsive profile: faster polling (4ms vs 10ms),
shorter cooldown (350ms vs 750ms), higher sensitivity (0.18 vs 0.05
threshold), and larger sample batch (320 vs 200).

`--min-amplitude` and `--cooldown` always override the preset.

### Sensitivity

`--min-amplitude` (default `0.05`):

- 0.05 - 0.10: very sensitive, picks up light taps
- 0.15 - 0.30: balanced
- 0.30 - 0.50: only firm slaps register

The value is the minimum acceleration amplitude in g-force.

## Browser frontend (HTTP + SSE)

For a graphical UI, run Pafu in serve mode:

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

This exposes the API at `http://127.0.0.1:17878`:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/status` | Current runtime status |
| `POST` | `/api/pause` | Pause responses |
| `POST` | `/api/resume` | Resume responses |
| `POST` | `/api/set` | Update settings |
| `GET` | `/api/packs` | List voice packs |
| `POST` | `/api/packs/activate` | Activate a voice pack |
| `POST` | `/api/packs/import` | Upload MP3s as a new user pack |
| `DELETE` | `/api/packs/delete` | Delete a user pack |
| `GET` | `/api/events` | Live event stream (SSE) |

The local-only listener (`127.0.0.1`) means only frontends running on the
same Mac can call it. See `NEXTJS_INTEGRATION.md` for full schemas and a
Next.js integration walkthrough; see `FRONTEND_DESIGN.md` for the visual
design system.

User-imported voice packs are saved under:

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

When Pafu is started with `sudo`, the home directory is resolved from the
real user (`$SUDO_USER`), so files always end up under your account, not
`/var/root`.

## Stdio mode (Electron / Tauri)

```bash
sudo ./pafu --stdio
```

Pafu writes one JSON event per line to stdout and accepts JSON commands on
stdin. This is intended for desktop shells that spawn Pafu as a subprocess.

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

> **Note:** Update the binary path if you installed Pafu elsewhere
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

## How it works

1. Reads raw accelerometer data via IOKit HID (Apple SPU sensor)
2. Runs vibration detection (STA/LTA, CUSUM, kurtosis, peak/MAD)
3. When a significant impact passes the threshold, plays an MP3 from the
   active voice pack
4. Optional volume scaling (`--volume-scaling`) makes light taps quieter and
   hard slaps louder
5. Optional speed control (`--speed`) adjusts playback speed and pitch
6. A 750ms cooldown prevents rapid-fire responses, adjustable via
   `--cooldown`

## Credits

Sensor reading and vibration detection are based on
[olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer),
and the original slap detector implementation from
[taigrr/spank](https://github.com/taigrr/spank).

## License

MIT

<!-- Links -->
[readme-zh-link]: ./README-zh.md
