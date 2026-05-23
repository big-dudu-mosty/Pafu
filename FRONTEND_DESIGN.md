# Frontend Design Specification

This document describes the visual design language, functional modules and
backend integration mapping for the Pafu Next.js frontend.

It complements `NEXTJS_INTEGRATION.md` (pure API integration) with the
product/design requirements derived from the reference mockup.

---

## 1. Brand & Mood

### Keywords

- Cyber
- Emotional
- Fetish Aesthetic
- Reactive
- Minimal
- Glitch
- Dark
- Cute but Dangerous

### Tone

A reactive desktop creature that mirrors the user's pressure. The product is a
release valve, not a productivity tool. Visuals lean into **dark**,
**emotional**, and **playful but unsettling**. Avoid generic SaaS aesthetics.

### Hero Slogan (default copy)

- EN: `Your pressure. Its reaction.`
- ZH: `拍打 Mac，释放压力，看它如何回应你。`
- Tagline: `MINIMAL · REACTIVE · PERSONAL`

---

## 2. Color System

| Token | Hex | Usage |
| --- | --- | --- |
| `bg-base` | `#0A0A0B` | Page background, deepest layer |
| `bg-surface` | `#161618` | Section background |
| `bg-elevated` | `#232228` | Card / panel surface |
| `accent-primary` | `#A855F7` | Primary actions, links, active sliders |
| `accent-danger` | `#FF3B6C` | Heavy slaps, alerts, rage state |
| `accent-info` | `#4D7CFE` | Secondary, links, calm signals |
| `text-primary` | `#E6E6E6` | Primary text |
| `text-muted` | `#7A7A85` | Secondary text, labels |
| `border-subtle` | `#2A2A30` | Card borders, dividers |

### Tailwind Theme Extension

```ts
// tailwind.config.ts
import type { Config } from "tailwindcss";

const config: Config = {
  theme: {
    extend: {
      colors: {
        bg: {
          base: "#0A0A0B",
          surface: "#161618",
          elevated: "#232228",
        },
        accent: {
          primary: "#A855F7",
          danger: "#FF3B6C",
          info: "#4D7CFE",
        },
        text: {
          primary: "#E6E6E6",
          muted: "#7A7A85",
        },
        borderx: {
          subtle: "#2A2A30",
        },
      },
      boxShadow: {
        "glow-primary": "0 0 24px rgba(168, 85, 247, 0.45)",
        "glow-danger": "0 0 28px rgba(255, 59, 108, 0.55)",
      },
    },
  },
};

export default config;
```

---

## 3. Typography

| Role | Font (EN) | Font (ZH) | Weight |
| --- | --- | --- | --- |
| Display | `Sora` or `Space Grotesk` | `PingFang SC` | 700 |
| Body | `Inter` | `PingFang SC` / `Noto Sans SC` | 400/500 |
| Numeric / Mono | `JetBrains Mono` or `Geist Mono` | same | 500 |

Recommended scale (Tailwind):

- Hero: `text-6xl md:text-7xl tracking-tight`
- Section title: `text-3xl font-semibold`
- Body: `text-base`
- Caption: `text-xs uppercase tracking-widest text-text-muted`

---

## 4. Creature & Mood System

The Creature is the centerpiece. It reflects the user's slap behavior in real
time. The frontend derives the creature's state from raw slap events.

### Mood States (5)

| Mood | Trigger heuristic | Visual treatment |
| --- | --- | --- |
| `calm` | No slaps in last 60s | Eyes closed, slow breathing animation, blue tint |
| `annoyed` | 3-7 slaps / 60s | Eyes half-open, slight twitch, warm purple tint |
| `angry` | 8-14 slaps / 60s, avg amp > 0.1 | One eye `X`, shake on each slap, red tint |
| `broken` | 15-24 slaps / 60s | Both eyes `X`, mouth distortion, glitch overlay |
| `rage` | 25+ slaps / 60s OR avg amp > 0.3 sustained | Full glitch, red strobe, screen shake on slap |

### Single-Slap Intensity Levels (4)

| Level | Amplitude range (g) | Animation |
| --- | --- | --- |
| `light` | `amp < 0.10` | Soft pulse halo, micro tilt |
| `medium` | `0.10 ≤ amp < 0.25` | Pulse halo + 4px screen shake |
| `heavy` | `0.25 ≤ amp < 0.50` | Big halo + 12px shake + glitch frame |
| `combo` | Detected when 3+ slaps in last 1.5s | Stacked halo + chromatic aberration |

### Mood Derivation (reference implementation)

Create `lib/pafu/mood.ts`:

```ts
import type { PafuSlapEvent } from "./types";

export type Mood = "calm" | "annoyed" | "angry" | "broken" | "rage";
export type Intensity = "light" | "medium" | "heavy" | "combo";

export function intensityFromAmp(amp: number): Intensity {
  if (amp >= 0.5) return "heavy";
  if (amp >= 0.25) return "heavy";
  if (amp >= 0.1) return "medium";
  return "light";
}

export function detectCombo(events: PafuSlapEvent[], now: number) {
  const recent = events.filter(
    (e) => now - new Date(e.timestamp).getTime() < 1500
  );
  return recent.length >= 3;
}

export function deriveMood(events: PafuSlapEvent[], now: number): Mood {
  const recent = events.filter(
    (e) => now - new Date(e.timestamp).getTime() < 60_000
  );
  if (recent.length === 0) return "calm";

  const avgAmp = recent.reduce((s, e) => s + e.amplitude, 0) / recent.length;

  if (recent.length >= 25 || avgAmp >= 0.3) return "rage";
  if (recent.length >= 15) return "broken";
  if (recent.length >= 8) return "angry";
  if (recent.length >= 3) return "annoyed";
  return "calm";
}

export function deriveBpm(events: PafuSlapEvent[], now: number): number {
  const recent = events.filter(
    (e) => now - new Date(e.timestamp).getTime() < 60_000
  );
  return recent.length;
}
```

> Tune the thresholds with the design team. The numbers above are starting
> points; we should iterate during the first internal demo.

---

## 5. App Appearance Modes

The reference mockup proposes three runtime appearances. With a Next.js web
frontend, only the panel mode is fully native. Floating widgets and menu bar
items require an Electron/Tauri shell which is out of MVP scope.

| Mode | MVP feasibility | Notes |
| --- | --- | --- |
| Settings panel | Full | Default Next.js page |
| Floating desktop widget | Out of MVP | Requires Tauri/Electron shell |
| Menu bar | Out of MVP | Same as above |

For MVP, ship the settings panel as the single experience, but design the UI
so it can be reused inside an Electron `BrowserWindow` later (no
hard-coded `position: fixed` on body, no global keyboard shortcuts).

---

## 6. Module Map (UI ↔ Backend)

| UI module | Backend touchpoint | Notes |
| --- | --- | --- |
| Connection status | `GET /api/status` + SSE open/error | Reflects whether backend is running |
| BPM / mood badge | SSE `slap` events (client-side derivation) | No backend call |
| Sensitivity slider | `POST /api/set { amplitude }` | Debounce 200ms |
| Cooldown slider | `POST /api/set { cooldown }` | Debounce 200ms |
| Speed slider | `POST /api/set { speed }` | Debounce 200ms |
| Volume scaling toggle | `POST /api/set { volume_scaling }` | Immediate |
| Pause / resume button | `POST /api/pause` / `POST /api/resume` | Immediate |
| Voice pack selection | `GET /api/packs` + `POST /api/packs/activate` | Runtime switch, no backend restart |
| Voice pack import | `POST /api/packs/import` with `FormData` | Browser uploads MP3 directly to backend |
| Voice pack delete | `DELETE /api/packs/delete` | Only user-imported packs |
| Mode (pain/sexy/halo/lizard) | `POST /api/packs/activate` | Built-in packs are exposed as switchable packs |

---

## 7. Page Architecture

```
app/
├── layout.tsx                # Theme provider, fonts, global glitch overlay
├── page.tsx                  # Hero with Creature + quick controls
├── customize/
│   └── page.tsx              # Creature picker, voice pack manager, sliders
├── settings/
│   └── page.tsx              # Connection, advanced parameters, about
└── components/
    ├── creature/
    │   ├── creature-canvas.tsx     # Animated creature renderer
    │   ├── pulse-halo.tsx          # Reactive halo around the creature
    │   └── glitch-overlay.tsx      # Full-screen glitch on rage/broken
    ├── controls/
    │   ├── sensitivity-slider.tsx
    │   ├── cooldown-slider.tsx
    │   ├── speed-slider.tsx
    │   └── volume-scaling-toggle.tsx
    ├── feedback/
    │   ├── intensity-grid.tsx      # 4-tile intensity preview
    │   └── waveform.tsx            # ECG-style heartbeat
    ├── voice-pack/
    │   ├── pack-list.tsx
    │   ├── pack-card.tsx
    │   └── pack-import-button.tsx
    └── ui/                     # shadcn/ui style primitives
        ├── button.tsx
        ├── slider.tsx
        ├── toggle.tsx
        └── indicator.tsx
```

---

## 8. Component Specifications

### 8.1 Buttons

| Variant | Background | Text | Hover | Use |
| --- | --- | --- | --- | --- |
| Primary | `accent-primary` | white | scale 1.02 + glow-primary | Main action |
| Secondary | `bg-elevated` | `text-primary` | border accent | Side action |
| Ghost | transparent | `text-muted` | `text-primary` | Tertiary |

Use shadcn/ui `Button` as base; override with the tokens above.

### 8.2 Slider (70% style)

- Track: 4px high, `bg-elevated`
- Filled portion: `accent-primary`, with subtle inner shadow
- Thumb: 12px circle, `accent-primary`, `glow-primary` on focus
- Show numeric value to the right (e.g. `70%`)

### 8.3 Toggle

- Off state: `bg-elevated` track + `text-muted` thumb
- On state: `accent-primary` track + white thumb
- 200ms cubic-bezier transition

### 8.4 Indicator (status dot)

- 8px circle
- `calm` → `accent-info`
- `annoyed` → orange `#FFB04D`
- `angry`/`broken`/`rage` → `accent-danger`
- Pulse animation tied to BPM:
  - duration = `60 / Math.max(bpm, 30)` seconds
  - opacity 0.4 → 1.0 → 0.4

### 8.5 Heartbeat / ECG line

Used in mood badge and intensity preview. Implement with SVG `path` + animated
`stroke-dashoffset`. Color follows mood.

---

## 9. Visual Effects

| Effect | Trigger | Implementation |
| --- | --- | --- |
| Pulse halo | Every slap | Framer Motion scale + opacity, color from intensity |
| Shockwave ring | `medium` and above | Expanding circle with `border-radius: 50%` |
| Glitch overlay | `broken` / `rage` mood | CSS keyframes `clip-path` shifts + RGB split |
| Screen shake | `heavy` slap | `transform: translate(±Xpx, ±Ypx)` over 200ms |
| Chromatic aberration | `combo` | Filter on Creature SVG: red layer offset +2px, blue -2px |
| Color strobe | `rage` | Background flash with `accent-danger` at 8% opacity |

Animation library recommendation:

- **Framer Motion** for declarative animations
- **CSS keyframes** for glitch and strobe (better perf for full-screen)
- **react-three-fiber** optional for 3D Creature (only if design provides 3D
  asset)

---

## 10. Voice Pack Management

### Data model

```ts
type VoicePack = {
  id: string;
  name: string;
  mode: "random" | "escalation";
  source: "builtin" | "custom";
  builtIn: boolean;
  active: boolean;
  path?: string;
  fileCount: number;
};
```

Backend pack IDs:

| Pack id | Display name | Notes |
| --- | --- | --- |
| `pain` | Pain | Default random pack |
| `sexy` | Sexy | Escalation pack |
| `halo` | Halo | Random Halo-style pack |
| `lizard` | Lizard | Escalation pack |
| `custom` | Custom | Present when backend starts with `--custom <dir>` |
| `custom-files` | Custom Files | Present when backend starts with `--custom-files` |

The visual UI may still display branded names such as `傲娇喵喵包` or
`电子崩溃包`, but activation must use the backend `id` returned by
`GET /api/packs`.

### Runtime switching flow

1. Fetch `GET /api/packs` on page load.
2. Render the returned `packs` array.
3. When the user selects a card, call:

   ```ts
   await fetch("http://127.0.0.1:17878/api/packs/activate", {
     method: "POST",
     headers: { "Content-Type": "application/json" },
     body: JSON.stringify({ id: pack.id }),
   });
   ```

4. Update UI from the response and listen for SSE `pack-changed` events.
5. The next slap uses the activated pack immediately.

User-defined packs are stored under:

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

### Import flow

1. User clicks `+ 导入自定义声音包`.
2. Frontend opens a file picker: `<input type="file" accept="audio/mpeg,.mp3" multiple>`.
3. Frontend builds `FormData`:
   ```ts
   const form = new FormData();
   form.append("name", packName);
   files.forEach((file) => form.append("files", file));
   await fetch("http://127.0.0.1:17878/api/packs/import", { method: "POST", body: form });
   ```
4. Refresh `GET /api/packs`.
5. Optionally auto-activate the imported pack with `POST /api/packs/activate`.
6. Listen for SSE `pack-imported` to update the list in real time.

The backend stores files under:

```text
~/Library/Application Support/Pafu/packs/<pack-id>/
```

The exact user home directory is resolved automatically, even when Pafu is
started with `sudo`.

---

## 11. Localization

The mockup mixes English headers and Chinese microcopy. Provide both. Use
`next-intl` or simple JSON dictionaries.

| Section | EN | ZH |
| --- | --- | --- |
| Hero pressure line | `Your pressure.` | `你的压力。` |
| Hero reaction line | `Its reaction.` | `它的反应。` |
| Section: feedback | `FEEDBACK BY INTENSITY` | `拍打程度不同，反应不同` |
| Section: appearance | `APP APPEARANCE` | `应用形态` |
| Section: customization | `CUSTOMIZATION` | `自定义你的专属体验` |
| Section: components | `UI COMPONENTS` | `组件` |
| Section: effects | `ICON & EFFECTS` | `图标 & 效果` |
| Status: calm | `Calm` | `平静` |
| Status: annoyed | `Annoyed` | `烦躁` |
| Status: angry | `Angry` | `愤怒` |
| Status: broken | `Broken` | `崩溃` |
| Status: rage | `Rage` | `暴走` |
| Footer | `DESIGN FOR RELEASE. REACT FOR RELIEF.` | `为释放而设计，为减压而响应。` |

---

## 12. State Management

Use **Zustand** for global UI state, **React Query (TanStack Query)** for
backend status fetching.

```ts
// stores/pafu-store.ts
import { create } from "zustand";
import type { PafuSlapEvent } from "@/lib/pafu/types";
import { type Mood, deriveMood, deriveBpm } from "@/lib/pafu/mood";

type PafuState = {
  events: PafuSlapEvent[];
  bpm: number;
  mood: Mood;
  pushSlap: (e: PafuSlapEvent) => void;
  reset: () => void;
};

export const usePafuStore = create<PafuState>((set) => ({
  events: [],
  bpm: 0,
  mood: "calm",
  pushSlap: (e) =>
    set((state) => {
      const trimmed = [...state.events, e].slice(-200);
      const now = Date.now();
      return {
        events: trimmed,
        bpm: deriveBpm(trimmed, now),
        mood: deriveMood(trimmed, now),
      };
    }),
  reset: () => set({ events: [], bpm: 0, mood: "calm" }),
}));
```

Recompute `bpm` and `mood` every second from a `setInterval` so the mood
decays back to `calm` even when no new events arrive:

```ts
useEffect(() => {
  const id = setInterval(() => {
    const { events, pushSlap } = usePafuStore.getState();
    if (events.length === 0) return;
    pushSlap(events[events.length - 1]); // re-trigger derivation
  }, 1000);
  return () => clearInterval(id);
}, []);
```

---

## 13. Asset Checklist

The design team must deliver:

- [ ] Creature character set: 3 skins (cute / demon / glitch) × 5 moods
      (calm / annoyed / angry / broken / rage), each as static SVG + idle
      animation (Lottie or sprite sheet)
- [ ] Heartbeat waveform SVGs: 4 intensities
- [ ] Icon set: lightning, heart, collar, soundwave, glitch bar, shock,
      pulse, status dots
- [ ] Voice pack covers: 3 default packs + neutral fallback for user packs
- [ ] App appearance preview screenshots: desktop / menubar / panel
- [ ] Brand fonts (license confirmed)

---

## 14. Implementation Milestones

### Sprint 1 (1 week) — Skeleton

- [ ] Next.js + Tailwind + shadcn/ui setup with theme tokens above
- [ ] `lib/pafu/{client,types,mood}.ts`
- [ ] `usePafuEvents` hook
- [ ] Settings page wired to all backend endpoints
- [ ] Connection state badge
- [ ] Static placeholder Creature (single image)

### Sprint 2 (1 week) — Reactive Creature

- [ ] Mood derivation + state store
- [ ] Pulse halo + shockwave + screen shake
- [ ] BPM badge + ECG waveform
- [ ] Voice pack manager UI (read-only built-ins)

### Sprint 3 (1 week) — Customization

- [ ] Creature skin switcher
- [ ] Voice pack import flow with toast/restart guide
- [ ] Glitch overlay for `broken` / `rage`
- [ ] Settings persistence (localStorage)

### Sprint 4 (optional) — Polish

- [ ] Animations review with design
- [ ] i18n EN/ZH switcher
- [ ] Performance audit (Framer Motion is heavy; lazy load Creature scene)

---

## 15. Open Decisions

These are not blockers but should be clarified before Sprint 2:

1. Will we ship 3 Creature skins or only 1 in MVP?
2. Where do default voice packs live? (we need to package the MP3s and a
   downloader, or include them in the Next.js public folder?)
3. Do we need an opt-in for the rage screen strobe? (accessibility concern —
   photosensitivity)
4. Who provides Chinese copy review?
5. Will we run the frontend with `next dev` over HTTP or do we need
   `https://localhost`? (matters for CORS and `EventSource` behavior)

When answers are available, update this document and tag affected components.

---

For pure HTTP/SSE wire format and code samples, see `NEXTJS_INTEGRATION.md`.
