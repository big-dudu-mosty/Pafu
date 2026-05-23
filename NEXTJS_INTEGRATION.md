# Next.js Frontend Integration

This document shows how a Next.js + React + TypeScript frontend can talk to
the local Pafu backend over HTTP and SSE.

> **Migration note:** the previous version of this guide used `Spank*`
> identifiers (`SpankStatus`, `useSpankEvents`, `lib/spank/...`). The schema
> on the wire has not changed, but the suggested folder and identifier names
> are now `pafu`-prefixed. If your existing frontend code already uses the
> old names, you can either rename or keep them - the HTTP API behaves
> identically either way.

## Backend Startup

Run the backend once on the same Mac:

```bash
cd /Users/dudu/Projects/Internship_Program/spank_mac/spank
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

Default API base URL:

```ts
const PAFU_API_BASE = "http://127.0.0.1:17878";
```

The backend only listens on `127.0.0.1`, so it is intended for local browser
frontends running on the same machine.

## API Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/status` | Get current runtime status |
| `POST` | `/api/pause` | Pause slap responses |
| `POST` | `/api/resume` | Resume slap responses |
| `POST` | `/api/set` | Update runtime settings |
| `GET` | `/api/packs` | List switchable voice packs |
| `POST` | `/api/packs/activate` | Activate a voice pack without restarting |
| `POST` | `/api/packs/import` | Upload MP3 files and create a user voice pack |
| `DELETE` | `/api/packs/delete` | Delete a user-imported voice pack |
| `GET` | `/api/events` | Subscribe to slap events using SSE |

## TypeScript Types

Create `lib/pafu/types.ts`:

```ts
export type PafuStatus = {
  status: "ok";
  active_pack_id: string;
  mode: string;
  preset: string;
  paused: boolean;
  amplitude: number;
  cooldown: number;
  speed: number;
  volume_scaling: boolean;
  version: string;
};

export type PafuReadyEvent = {
  active_pack_id: string;
  mode: string;
  preset: string;
  amplitude: number;
  cooldown: number;
  speed: number;
  volume_scaling: boolean;
  version: string;
};

export type PafuSlapEvent = {
  timestamp: string;
  slapNumber: number;
  amplitude: number;
  severity: string;
  file: string;
  activePackId: string;
  activePackName: string;
};

export type PafuPack = {
  id: string;
  name: string;
  mode: "random" | "escalation";
  source: "builtin" | "custom" | "user";
  built_in: boolean;
  active: boolean;
  file_count: number;
  path?: string;
};

export type PafuPacksResponse = {
  active_pack_id: string;
  packs: PafuPack[];
};

export type PafuPackActivatedResponse = {
  status: "pack_activated";
  active_pack_id: string;
  pack: PafuPack;
};

export type PafuPackImportedResponse = {
  status: "pack_imported";
  pack: PafuPack;
};

export type PafuPackDeletedResponse = {
  status: "pack_deleted";
  deleted_pack_id: string;
  active_pack_id: string;
  pack: PafuPack;
};

export type PafuSettingsUpdate = {
  amplitude?: number;
  cooldown?: number;
  speed?: number;
  volume_scaling?: boolean;
};

export type PafuSettingsResponse = {
  status: "settings_updated";
  amplitude: number;
  cooldown: number;
  speed: number;
  volume_scaling: boolean;
};
```

## API Client

Create `lib/pafu/client.ts`:

```ts
import type {
  PafuPackActivatedResponse,
  PafuPackDeletedResponse,
  PafuPackImportedResponse,
  PafuPacksResponse,
  PafuSettingsResponse,
  PafuSettingsUpdate,
  PafuStatus,
} from "./types";

export const PAFU_API_BASE =
  process.env.NEXT_PUBLIC_PAFU_API_BASE ?? "http://127.0.0.1:17878";

async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${PAFU_API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });

  if (!res.ok) {
    let message = `pafu request failed: ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // Keep the generic message when the response body is not JSON.
    }
    throw new Error(message);
  }

  return res.json() as Promise<T>;
}

export function getPafuStatus() {
  return requestJson<PafuStatus>("/api/status");
}

export function pausePafu() {
  return requestJson<{ status: "paused" }>("/api/pause", {
    method: "POST",
  });
}

export function resumePafu() {
  return requestJson<{ status: "resumed" }>("/api/resume", {
    method: "POST",
  });
}

export function updatePafuSettings(settings: PafuSettingsUpdate) {
  return requestJson<PafuSettingsResponse>("/api/set", {
    method: "POST",
    body: JSON.stringify(settings),
  });
}

export function getPafuPacks() {
  return requestJson<PafuPacksResponse>("/api/packs");
}

export function activatePafuPack(id: string) {
  return requestJson<PafuPackActivatedResponse>("/api/packs/activate", {
    method: "POST",
    body: JSON.stringify({ id }),
  });
}

export async function importPafuPack(name: string, files: File[]) {
  const form = new FormData();
  form.append("name", name);
  files.forEach((file) => form.append("files", file));

  const res = await fetch(`${PAFU_API_BASE}/api/packs/import`, {
    method: "POST",
    body: form,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.error ?? `import failed: ${res.status}`);
  }
  return res.json() as Promise<PafuPackImportedResponse>;
}

export function deletePafuPack(id: string) {
  return requestJson<PafuPackDeletedResponse>("/api/packs/delete", {
    method: "DELETE",
    body: JSON.stringify({ id }),
  });
}
```

## SSE Hook

Create `hooks/use-pafu-events.ts`:

```tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { PAFU_API_BASE } from "@/lib/pafu/client";
import type { PafuReadyEvent, PafuSlapEvent } from "@/lib/pafu/types";

export function usePafuEvents() {
  const [connected, setConnected] = useState(false);
  const [ready, setReady] = useState<PafuReadyEvent | null>(null);
  const [lastSlap, setLastSlap] = useState<PafuSlapEvent | null>(null);
  const [slapCount, setSlapCount] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const source = new EventSource(`${PAFU_API_BASE}/api/events`);
    sourceRef.current = source;

    source.onopen = () => {
      setConnected(true);
      setError(null);
    };

    source.onerror = () => {
      setConnected(false);
      setError("Unable to connect to local Pafu backend");
    };

    source.addEventListener("ready", (event) => {
      setReady(JSON.parse((event as MessageEvent).data));
    });

    source.addEventListener("slap", (event) => {
      const slap = JSON.parse((event as MessageEvent).data) as PafuSlapEvent;
      setLastSlap(slap);
      setSlapCount((count) => count + 1);
    });

    source.addEventListener("pack-changed", (event) => {
      console.log("active pack changed", JSON.parse((event as MessageEvent).data));
    });

    source.addEventListener("pack-imported", (event) => {
      console.log("pack imported", JSON.parse((event as MessageEvent).data));
    });

    source.addEventListener("pack-deleted", (event) => {
      console.log("pack deleted", JSON.parse((event as MessageEvent).data));
    });

    source.addEventListener("bye", () => {
      setConnected(false);
    });

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, []);

  return {
    connected,
    ready,
    lastSlap,
    slapCount,
    error,
  };
}
```

If your project does not use the `@/*` alias, replace imports like
`@/lib/pafu/client` with relative paths.

## Example Client Component

Create `app/pafu/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import {
  getPafuStatus,
  pausePafu,
  resumePafu,
  updatePafuSettings,
} from "@/lib/pafu/client";
import type { PafuStatus } from "@/lib/pafu/types";
import { usePafuEvents } from "@/hooks/use-pafu-events";

export default function PafuPage() {
  const { connected, ready, lastSlap, slapCount, error } = usePafuEvents();
  const [status, setStatus] = useState<PafuStatus | null>(null);
  const [amplitude, setAmplitude] = useState(0.05);
  const [cooldown, setCooldown] = useState(750);
  const [speed, setSpeed] = useState(1);
  const [volumeScaling, setVolumeScaling] = useState(false);

  async function refreshStatus() {
    const next = await getPafuStatus();
    setStatus(next);
    setAmplitude(next.amplitude);
    setCooldown(next.cooldown);
    setSpeed(next.speed);
    setVolumeScaling(next.volume_scaling);
  }

  useEffect(() => {
    refreshStatus().catch(() => {
      // The backend may not be running yet. The SSE hook shows connection state.
    });
  }, []);

  async function applySettings(next?: Partial<{
    amplitude: number;
    cooldown: number;
    speed: number;
    volume_scaling: boolean;
  }>) {
    const updated = await updatePafuSettings({
      amplitude,
      cooldown,
      speed,
      volume_scaling: volumeScaling,
      ...next,
    });
    setAmplitude(updated.amplitude);
    setCooldown(updated.cooldown);
    setSpeed(updated.speed);
    setVolumeScaling(updated.volume_scaling);
    await refreshStatus();
  }

  return (
    <main className="mx-auto flex max-w-3xl flex-col gap-6 p-8">
      <section className="rounded-xl border p-4">
        <h1 className="text-2xl font-semibold">Pafu Control Panel</h1>
        <p className="mt-2 text-sm text-gray-500">
          Backend: {connected ? "connected" : "disconnected"}
        </p>
        {error && <p className="mt-2 text-sm text-red-500">{error}</p>}
      </section>

      <section className="grid gap-3 rounded-xl border p-4">
        <h2 className="text-lg font-medium">Runtime Status</h2>
        <p>Mode: {status?.mode ?? ready?.mode ?? "-"}</p>
        <p>Preset: {status?.preset ?? ready?.preset ?? "-"}</p>
        <p>Paused: {status?.paused ? "yes" : "no"}</p>
        <p>Total slaps in this page session: {slapCount}</p>
        {lastSlap && (
          <div className="rounded-lg bg-gray-100 p-3 text-sm">
            <p>Last slap #{lastSlap.slapNumber}</p>
            <p>Amplitude: {lastSlap.amplitude.toFixed(4)}g</p>
            <p>Severity: {lastSlap.severity}</p>
            <p className="break-all">File: {lastSlap.file}</p>
          </div>
        )}
      </section>

      <section className="grid gap-4 rounded-xl border p-4">
        <h2 className="text-lg font-medium">Controls</h2>

        <div className="flex gap-2">
          <button className="rounded bg-black px-4 py-2 text-white" onClick={refreshStatus}>
            Refresh
          </button>
          <button className="rounded bg-gray-900 px-4 py-2 text-white" onClick={pausePafu}>
            Pause
          </button>
          <button className="rounded bg-gray-900 px-4 py-2 text-white" onClick={resumePafu}>
            Resume
          </button>
        </div>

        <label className="grid gap-1">
          <span>Amplitude: {amplitude.toFixed(2)}</span>
          <input
            type="range"
            min="0.01"
            max="0.5"
            step="0.01"
            value={amplitude}
            onChange={(e) => setAmplitude(Number(e.target.value))}
            onMouseUp={() => applySettings()}
          />
        </label>

        <label className="grid gap-1">
          <span>Cooldown: {cooldown}ms</span>
          <input
            type="range"
            min="100"
            max="2000"
            step="50"
            value={cooldown}
            onChange={(e) => setCooldown(Number(e.target.value))}
            onMouseUp={() => applySettings()}
          />
        </label>

        <label className="grid gap-1">
          <span>Speed: {speed.toFixed(2)}x</span>
          <input
            type="range"
            min="0.5"
            max="2"
            step="0.05"
            value={speed}
            onChange={(e) => setSpeed(Number(e.target.value))}
            onMouseUp={() => applySettings()}
          />
        </label>

        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={volumeScaling}
            onChange={(e) => {
              const checked = e.target.checked;
              setVolumeScaling(checked);
              applySettings({ volume_scaling: checked });
            }}
          />
          <span>Volume scaling by slap amplitude</span>
        </label>
      </section>
    </main>
  );
}
```

## Browser Console Smoke Test

After starting `pafu --serve`, open your Next.js app and run:

```ts
await fetch("http://127.0.0.1:17878/api/status").then((r) => r.json());
```

Expected:

```json
{
  "status": "ok",
  "mode": "custom",
  "paused": false
}
```

SSE test:

```ts
const es = new EventSource("http://127.0.0.1:17878/api/events");
es.addEventListener("slap", (e) => console.log("slap", JSON.parse(e.data)));
```

Then slap the MacBook and check the browser console.

## Voice Pack Switching

The backend supports runtime pack switching. The frontend does **not** need
to restart Pafu when switching between packs that are already known to the
backend.

List packs:

```ts
const packs = await fetch("http://127.0.0.1:17878/api/packs").then((r) =>
  r.json()
);
```

Response:

```json
{
  "active_pack_id": "custom",
  "packs": [
    {
      "id": "custom",
      "name": "Custom",
      "mode": "random",
      "source": "custom",
      "built_in": false,
      "active": true,
      "file_count": 9,
      "path": "/Users/dudu/Music/pafu-custom"
    },
    {
      "id": "halo",
      "name": "Halo",
      "mode": "random",
      "source": "builtin",
      "built_in": true,
      "active": false,
      "file_count": 9,
      "path": "audio/halo"
    }
  ]
}
```

Activate a pack:

```ts
await fetch("http://127.0.0.1:17878/api/packs/activate", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ id: "halo" }),
});
```

After activation, the next slap uses the new pack immediately. The SSE stream
also emits:

```text
event: pack-changed
data: {"active_pack_id":"halo","pack":{...}}
```

Known pack IDs:

- `pain`
- `sexy`
- `halo`
- `lizard`
- `custom` (only present when backend was started with `--custom <dir>`)
- `custom-files` (only present when backend was started with `--custom-files`)

## Upload / Custom Voice Pack Flow

The browser frontend can upload MP3 files directly to the backend.

```ts
const input = document.querySelector("input[type=file]") as HTMLInputElement;
const files = Array.from(input.files ?? []);

await importPafuPack("我的恶搞包", files);
await getPafuPacks();
```

Request:

```http
POST /api/packs/import
Content-Type: multipart/form-data

name=我的恶搞包
files=<mp3 file 1>
files=<mp3 file 2>
```

Response:

```json
{
  "status": "pack_imported",
  "pack": {
    "id": "my-pack-20260523210345",
    "name": "我的恶搞包",
    "mode": "random",
    "source": "user",
    "built_in": false,
    "active": false,
    "file_count": 2,
    "path": "/Users/xxx/Library/Application Support/Pafu/packs/my-pack-..."
  }
}
```

The backend stores packs under:

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

The exact home directory is resolved automatically from the real user, even
when Pafu is started with `sudo`. (If you have packs from an earlier build
under `Application Support/Spank/packs/`, the backend migrates them to the
new directory automatically on first startup.)

Delete a user-imported pack:

```ts
await deletePafuPack("my-pack-20260523210345");
```

Rules:

- only `.mp3` files are accepted
- max 60 files per pack
- max 10 MB per file
- max 200 MB per pack
- built-in packs (`pain`, `sexy`, `halo`, `lizard`) cannot be deleted
- if the active pack is deleted, backend falls back to `pain`

## Common Errors

### Backend is not running

Frontend symptoms:

- `fetch failed`
- `EventSource` connection error
- UI shows disconnected

Fix:

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

### Wrong sound is playing

Check `/api/status`:

```ts
await fetch("http://127.0.0.1:17878/api/status").then((r) => r.json());
```

If `mode` is `pain`, the backend was not started with `--custom`.

Start with:

```bash
sudo ./pafu --serve --custom ~/Music/pafu-custom
```

### CORS

The backend already sends:

```text
Access-Control-Allow-Origin: *
```

No Next.js proxy is required for local development.
