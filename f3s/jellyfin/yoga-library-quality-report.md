# Yoga library quality report

Survey date: 2026-09-06  
Host path: `f0.lan:/data/nfs/k3svolumes/jellyfin/libraries/yoga`  
No files were deleted; this is investigation + recommendation only.

## Method

1. Listed every file under the yoga library (`find`, `du`, extension counts).
2. Ran `ffprobe` stream probe on each file for codec / width / height / pix_fmt (written to `/tmp/yoga-probe.csv` on `f0`; note: stream CSV field order from ffprobe is `codec|width|height|…`, not the header’s `width|height|codec` — analysis accounted for that).
3. Ran a second `ffprobe` format probe for `duration` + `bit_rate` (`/tmp/yoga-duration.csv`; 480 of 489 paths — the 9 audio-only / odd files were handled separately).
4. Bitrate medians use `8 * size / duration` when duration is available.

## Summary

| Metric | Value |
|--------|------:|
| Files | 489 |
| Total size | **434.6 GiB** |
| Layout | Flat directory (no subfolders) |

The library is dominated by **Gaia at 1080p** (~277 GiB) plus a smaller **Charlie Follows / YouTube 4K** set. True “old low-res” content is a **Gaia 480p/720p** pocket (~21 GiB) with **no HD remux** of those titles. Most space is already decent quality; the easy wins are confirmed junk leftovers, one worse 4K duplicate encode, and an optional cull of sub-1080p Gaia.

## Resolution breakdown

| Resolution | Files | Size | Share of library |
|------------|------:|-----:|-----------------:|
| 4K (2160p) | 68 | 132.3 GiB | 30.4% |
| 1080p | 386 | 280.7 GiB | 64.6% |
| 720p | 3 | 1.6 GiB | 0.4% |
| ≤480p | 22 | 19.8 GiB | 4.6% |
| No video (audio/junk) | 10 | 0.1 GiB | ~0% |

Of the 68 “4K” rows, **3 are junk/incomplete** that still probe as 2160p (~3.7 GiB total): `[HbQfzWDZ2M4].f401.mp4`, `[HbQfzWDZ2M4].temp.webm`, and `[LkqKz9xf35A].f401.mp4.part`. Finished unique 4K is closer to **65 files / ~128.6 GiB**.

## Codec / container

| Codec | Files | Size |
|-------|------:|-----:|
| H.264 | 400 | 298.6 GiB |
| AV1 | 68 | 108.2 GiB |
| VP9 | 11 | 27.7 GiB |
| none / no video stream | 10 | 0.1 GiB |

| Ext | Files | Size |
|-----|------:|-----:|
| `.mp4` | 406 | 323.7 GiB |
| `.webm` | 73 | 109.7 GiB |
| `.mkv` | 1 | 0.5 GiB |
| `.part` | 1 | 0.6 GiB |
| `.m4a` | 8 | 0.1 GiB |

## By collection

| Collection | Files | Size | Notes |
|------------|------:|-----:|-------|
| Gaia | 398 | 297.7 GiB | 373×1080p, 3×720p, 22×≤480p — **all sub-HD video is Gaia** |
| Charlie Follows (named) | 39 | 91.5 GiB | Mostly 4K; some very high bitrate encodes |
| YouTube-raw (`N Min … [id]`) | 40 | 37.0 GiB | Mostly Charlie-style AV1/WebM 4K + leftover m4a/fragments |
| Other (Boho, Breathe and Flow, Kassandra, …) | 12 | ~8 GiB | Mostly 4K |

## Bitrate (encode efficiency)

Median video bitrate from `size / duration`:

| Resolution | Median | P90 | Max |
|------------|-------:|----:|----:|
| 4K | 6.7 Mbps | 14.9 Mbps | 17.9 Mbps |
| 1080p / 720p / 480p | ~3.7 Mbps | ~3.7 Mbps | ~3.7 Mbps |

Gaia encodes are consistently ~3.7 Mbps (fine for 1080p; the 480p files are long, not high-bitrate). Several Charlie Follows 4K files sit at **14–18 Mbps** and 4–6+ GiB each — keepable for quality, but the main “space hog” class if storage gets tight.

## Age vs quality

mtime is download time, not publish date:

- **2024–early 2025:** mix of 1080p Gaia and early 4K Charlie Follows
- **2025-05 / 2025-06:** bulk Gaia import (hundreds of 1080p + the 480p/720p set)
- **2025-10 → 2026-09:** continued 4K Charlie / YouTube AV1 downloads

“Older” does **not** mean low-res across the board. Low-res is almost entirely the Gaia ≤720p set from the May/June 2025 Gaia import.

## Recommendations

### Tier A — safe deletes (finished sibling exists) ≈ 9.4 GiB

Only files where a finished playable copy of the **same YouTube id / same title+duration** remains:

| Size | Why | File |
|-----:|-----|------|
| 1.68 GiB | yt-dlp fragment; final `[HbQfzWDZ2M4].webm` exists | `…[HbQfzWDZ2M4].f401.mp4` |
| 1.39 GiB | temp merge; final `.webm` exists | `…[HbQfzWDZ2M4].temp.webm` |
| 0.02 GiB | audio-only fragment of same id | `…[HbQfzWDZ2M4].f251.webm` |
| 0.01 GiB | audio stub; video sibling `.webm` exists | `…[6Oz0tyMQNbM].m4a` |
| 6.31 GiB | worse encode of exact duplicate (same title, ~3622s, 4K) | `Charlie Follows — 60 Min Yoga Flow Full Body Yoga for All Levels (1h00m22s, 4K).mp4` (VP9) — **keep** the AV1 `.webm` at 3.33 GiB |

**Reclaim if Tier A applied: ≈ 9.4 GiB.**

### Tier A2 — incomplete / audio-only with **no** video sibling (optional, content loss)

These are not useful as Jellyfin video entries, but deleting them **drops the only copy** of that id (or an unfinished download). Re-download as video first if you care about the title.

| Size | Note | File |
|-----:|------|------|
| 0.62 GiB | incomplete `.part`; only file for id `LkqKz9xf35A` | `…[LkqKz9xf35A].f401.mp4.part` |
| 0.02 GiB | labeled `[audio only]` | `Charlie Follows — 30 Min Daily Yoga Flow … [audio only] …` |
| 0.01 GiB each | orphan `.m4a`, no video for that id | `[I8tlg3xd1lo]`, `[a8vyQDUjc6w]`, `[ZFbEt_TzeNI]`, `[NTqVG9Kt070]`, `[F2zJ6y3ZDzY]`, `[H8RPcn9rW7E].m4a` |
| 0.01 GiB | orphan `.m4a` id `QzVhDYWWx4s`; a **different** id (`WmpYhGn8o2k`) has a 4K `.webm` with the same title text — not a proven substitute | `…[QzVhDYWWx4s].m4a` |

**A2 total ≈ 0.7 GiB.** Treat as cleanup after deciding whether to re-fetch those workouts.

### Tier B — optional delete if you refuse sub-HD (~21.4 GiB)

All **25 Gaia files below 1080p**. None have a 1080p counterpart in this folder. Content is unique; quality is clearly worse on a modern TV. Delete **only if** you accept losing that material (or will re-acquire it in HD).

**720p (3):**

- `Gaia — Get Into Your Glutes (44m59s, 720p).mp4` (0.67 GiB)
- `Gaia — Connect to Your Devotion (19m19s, 720p).mp4` (0.50 GiB)
- `Gaia — Water's Reflection (16m33s, 720p).mp4` (0.42 GiB)

**480p / 486p (22):**

- `Gaia — Yoga Conditioning for Life (1h15m21s, 480p).mp4` (1.95 GiB)
- `Gaia — Power Yoga for Total Body (1h04m59s, 480p).mp4` (1.68 GiB)
- `Gaia — Yoga Conditioning for Athletes (1h01m06s, 480p).mp4` (1.58 GiB)
- `Gaia — Yoga Burn (57m19s, 480p).mp4` (1.49 GiB)
- `Gaia — 50 Min Accelerated Workout (50m52s, 480p).mp4` (1.31 GiB)
- `Gaia — Restorative Yoga For Vitality (46m49s, 480p).mp4` (1.21 GiB)
- `Gaia — Abs Workout (44m39s, 480p).mp4` (1.16 GiB)
- `Gaia — Session 1 - Vinyasa and Inversions (36m34s, 480p).mp4` (0.94 GiB)
- `Gaia — Core Workout (32m31s, 480p).mp4` (0.84 GiB)
- `Gaia — Core Restoration (31m52s, 480p).mp4` (0.82 GiB)
- `Gaia — Alternate Angle (31m29s, 480p).mp4` (0.81 GiB)
- `Gaia — Power Yoga for Flexibility (30m19s, 480p).mp4` (0.78 GiB)
- `Gaia — Core Strengthening (30m08s, 480p).mp4` (0.78 GiB)
- `Gaia — Power Yoga for Strength (29m04s, 480p).mp4` (0.75 GiB)
- `Gaia — Twists (26m03s, 480p).mp4` (0.67 GiB)
- `Gaia — Power Yoga for Stamina (25m52s, 480p).mp4` (0.67 GiB)
- `Gaia — Session 2 - Standing Poses and Arm Balances (26m27s, 480p).mp4` (0.65 GiB)
- `Gaia — Upper Body On The Go (18m07s, 480p).mp4` (0.47 GiB)
- `Gaia — Standing Poses (16m24s, 480p).mp4` (0.42 GiB)
- `Gaia — Creativity (15m03s, 480p).mp4` (0.39 GiB)
- `Gaia — Reflection (10m41s, 480p).mp4` (0.28 GiB)
- `Gaia — Centering (6m52s, 480p).mp4` (0.18 GiB)

### Tier C — do **not** mass-delete

| Keep | Why |
|------|-----|
| Gaia 1080p (373 files, ~276 GiB) | Majority of the library; solid HD |
| Charlie Follows / YouTube 4K (finished files) | Best visual quality |
| Other 4K creators | Small footprint |

Avoid “delete all 4K to save space” unless storage is critical — that frees ~128+ GiB of finished 4K but throws away the best-looking workouts. Prefer re-encoding obese VP9/high-bitrate 4K to AV1 later if you need space without losing resolution.

### Suggested policy going forward

1. Apply Tier A now.
2. For Tier A2: either re-download as proper video, or delete consciously.
3. Decide on Tier B (Gaia ≤720p): keep for completeness, or drop for a clean HD+ library.
4. After YouTube downloads: keep only the final merged file; remove `.f###.*`, `.temp.*`, `.part`, and orphan `.m4a`.
5. Prefer one file per title: if both VP9/MP4 and AV1/WebM exist at the same resolution and duration, keep the smaller efficient encode.

## Verdict

You should **not** wipe the library for being “old and low-res.” ~95% of bytes are already 1080p or 4K. Clear safe cleanup is **Tier A (~9.4 GiB)** — confirmed fragments/temp plus one worse 4K duplicate. Separately consider **orphan audio / incomplete downloads (~0.7 GiB)** and optionally **Gaia ≤720p (~21.4 GiB)** if you only want HD+.
