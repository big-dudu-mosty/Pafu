# AGENTS.md

> Guidelines for AI agents working in this repository.

## Project Overview

**Pafu** is a macOS desktop creature that detects physical hits/slaps on
Apple Silicon MacBooks via the accelerometer and plays audio responses. The
backend is a Go binary that ships in three runtime modes (CLI, stdio, HTTP
server) and can be paired with a Next.js / React frontend.

- **Platform**: macOS on Apple Silicon (M2+) only
- **Runtime requirement**: `sudo` (for IOKit HID accelerometer access)
- **Backend layout**: Multiple Go files under the project root with embedded
  MP3 assets

> The Go module path is still `github.com/taigrr/spank` so that the original
> upstream history can be tracked. Do **not** rename the module path or any
> internal symbols (`spank`, `slapTracker`, etc.) - only product-facing names
> (binary, plist label, README copy) should read "Pafu".

## Commands

### Build & Run

```bash
# Build the backend binary
go build -o pafu .

# Run (requires sudo)
sudo ./pafu
sudo ./pafu --sexy      # escalating responses mode
sudo ./pafu --halo      # Halo death sounds mode
sudo ./pafu --lizard    # lizard escalation pack
sudo ./pafu --custom /path/to/mp3s  # custom audio directory

# HTTP API for browser frontends
sudo ./pafu --serve --port 17878

# Stdio JSON mode for desktop shells (Electron / Tauri)
sudo ./pafu --stdio
```

### Tests

```bash
go test ./...
```

### Release

Releases are automated via GitHub Actions + GoReleaser Pro when a `v*` tag is
pushed:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Code Organization

```text
spank/                    # repo root (kept for history compatibility)
├── main.go               # CLI entry, sensor loop, slap detection
├── stdin_handler.go      # --stdio JSON protocol
├── http_server.go        # --serve REST + SSE endpoints
├── pack_manager.go       # voice pack registry / active pack runtime
├── pack_storage.go       # user-uploaded pack persistence
├── audio/
│   ├── pain/             # default "ow!" responses
│   ├── sexy/             # escalating responses (60 MP3s)
│   ├── halo/             # Halo death sounds
│   └── lizard/           # lizard escalation pack
├── go.mod
├── .goreleaser.yaml      # release configuration
└── .github/workflows/    # CI/CD
```

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/taigrr/apple-silicon-accelerometer` | Reads accelerometer via IOKit HID |
| `github.com/gopxl/beep/v2` | Audio playback (MP3 decoding, speaker output) |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/charmbracelet/fang` | CLI config/execution wrapper |

## Code Patterns

### Embedded Assets

Audio files are embedded at compile time using `//go:embed`:

```go
//go:embed audio/pain/*.mp3
var painAudio embed.FS
```

### Play Modes

Two playback strategies in `playMode`:

- `modeRandom`: random file selection (pain, halo, custom modes)
- `modeEscalation`: intensity increases with slap frequency (sexy, lizard)

### Slap Detection Flow

1. `sensor.Run()` reads accelerometer in a background goroutine
2. Data shared via `shm.RingBuffer` (POSIX shared memory)
3. `detector.New()` processes samples with vibration detection algorithms
4. Events trigger audio playback with a 750ms cooldown

### Voice Pack Runtime

- `packRegistry` stores every available pack (built-in, command-line custom,
  user-uploaded)
- `activeRuntime` holds the currently active pack and its `slapTracker`
- HTTP / stdio handlers can swap the active pack by calling `setActivePack`
- User uploads land under `~/Library/Application Support/Pafu/packs/`
  (resolved through `$SUDO_USER` so that `sudo` does not poison the path)

### Concurrency

- `speakerMu sync.Mutex` protects speaker initialization
- `slapTracker.mu sync.Mutex` protects slap scoring state
- `packRegistryMu sync.RWMutex` protects pack list mutations
- Audio playback runs in goroutines (`go playAudio(...)`)

## Constants

Key tuning parameters in `main.go`:

| Constant | Value | Purpose |
|----------|-------|---------|
| `decayHalfLife` | 30s | How fast escalation fades |
| `slapCooldown` | 750ms | Minimum time between audio plays |
| `sensorPollInterval` | 10ms | Accelerometer polling rate |
| `maxSampleBatch` | 200 | Max samples processed per tick |

## Gotchas

1. **Root required**: the app must run with `sudo` for IOKit HID access. The
   `run()` function checks `os.Geteuid() != 0`.
2. **Apple Silicon only**: only builds for `darwin/arm64`. Intel Macs are not
   supported.
3. **Private dependency**: `github.com/taigrr/apple-silicon-accelerometer`
   requires `GOPRIVATE` and a GitHub PAT for CI.
4. **Mutually exclusive modes**: `--sexy`, `--halo`, `--lizard` and
   `--custom` cannot be combined at startup; runtime pack switching goes
   through the HTTP / stdio APIs instead.
5. **CGO disabled**: builds use `CGO_ENABLED=0` even though they target
   macOS.
6. **Stdio purity**: when `stdioMode` is on, the only thing written to stdout
   must be valid JSON, one event per line. Plain `fmt.Printf` calls must be
   guarded with `if !stdioMode { ... }`.

## Adding Audio

To add a new built-in sound pack:

1. Create a directory under `audio/`
2. Add MP3 files (numbered for escalation mode, any names for random)
3. Add a `//go:embed audio/newpack/*.mp3` variable
4. Register it inside `buildPackRegistry()` (in `pack_manager.go`)
5. Add a launch flag in `main.go` if it should be selectable at startup

User-uploaded packs do not need code changes; they are discovered at startup
by `loadUserPacks()` and persisted under `~/Library/Application Support/Pafu/packs/`.

## Version

Version is injected via ldflags at build time:

```go
var version = "dev"
```

GoReleaser sets `-X main.version={{.Version}}` during release builds.
