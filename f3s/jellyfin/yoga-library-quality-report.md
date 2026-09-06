# Yoga library quality report

Survey date: 2026-09-06  
Host path: `f0.lan:/data/nfs/k3svolumes/jellyfin/libraries/yoga`  
Method: `ffprobe` on every file (resolution, codec, size, format duration/bitrate)

No files were deleted; this is investigation + recommendation only.

## Summary

| Metric | Value |
|--------|------:|
| Files | 489 |
| Total size | **434.6 GiB** |
| Layout | Flat directory (no subfolders) |

The library is dominated by **Gaia at 1080p** (~277 GiB) plus a smaller **Charlie Follows / YouTube 4K** set (~132 GiB). True “old low-res” content is a **Gaia 480p/720p** pocket (~21 GiB) with **no HD remux** of those titles. Most space is already decent quality; the easy wins are junk leftovers and optional cull of sub-1080p Gaia.

## Resolution breakdown

| Resolution | Files | Size | Share of library |
|------------|------:|-----:|-----------------:|
| 4K (2160p) | 68 | 132.3 GiB | 30.4% |
| 1080p | 386 | 280.7 GiB | 64.6% |
| 720p | 3 | 1.6 GiB | 0.4% |
| ≤480p | 22 | 19.8 GiB | 4.6% |
| No video (audio/junk) | 10 | 0.1 GiB | ~0% |

## Codec / container

| Codec | Files | Size |
|-------|------:|-----:|
| H.264 | 400 | 298.6 GiB |
| AV1 | 68 | 108.2 GiB |
| VP9 | 11 | 27.7 GiB |
| none | 10 | 0.1 GiB |

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
| Gaia | 398 | 297.7 GiB | 373×1080p, 3×720p, 22×480p — **all low-res are Gaia-only titles** |
| Charlie Follows (named) | ~39 | ~91.5 GiB | Mostly 4K; some very high bitrate encodes |
| YouTube-raw (`N Min … [id]`) | ~40 | ~37 GiB | Mostly Charlie-style AV1/WebM 4K + leftover m4a/fragments |
| Other (Boho, Breathe and Flow, Kassandra, …) | ~12 | ~8 GiB | Mostly 4K |

## Bitrate (encode efficiency)

Median video bitrate from `size / duration`:

| Resolution | Median | P90 | Max |
|------------|-------:|----:|----:|
| 4K | 6.7 Mbps | 14.9 Mbps | 17.9 Mbps |
| 1080p / 720p / 480p | ~3.7 Mbps | ~3.7 Mbps | ~3.7 Mbps |

Gaia encodes are consistently ~3.7 Mbps (fine for 1080p; wasteful for 480p — those files are long, not high-bitrate). Several Charlie Follows 4K files sit at **14–18 Mbps** and 4–6+ GiB each; they are keepable for quality, but are the main “space hog” class if you ever need to reclaim large amounts.

## Age vs quality

mtime is download time, not publish date, but it still shows when material landed:

- **2024–early 2025:** mix of 1080p Gaia and early 4K Charlie Follows
- **2025-05 / 2025-06:** bulk Gaia import (hundreds of 1080p + the 480p/720p set)
- **2025-10 → 2026-09:** continued 4K Charlie / YouTube AV1 downloads

“Older” does **not** mean low-res across the board. Low-res is almost entirely the Gaia ≤720p set from the May/June 2025 Gaia import, not ancient YouTube leftovers.

## Recommendations

### Tier A — delete safely (~3.8 GiB)

Incomplete yt-dlp leftovers, temp merges, audio-only stubs. Safe to remove; they are not useful library entries.

| Size | Why | File |
|-----:|-----|------|
| 1.68 GiB | yt-dlp format fragment (final `.webm` exists) | `…[HbQfzWDZ2M4].f401.mp4` |
| 1.39 GiB | temp merge artifact (final `.webm` exists) | `…[HbQfzWDZ2M4].temp.webm` |
| 0.62 GiB | incomplete `.part` (no finished sibling for this id) | `…[LkqKz9xf35A].f401.mp4.part` |
| 0.02 GiB | audio-only webm | `Charlie Follows — … [audio only] …` |
| 0.02 GiB | audio-only fragment | `…[HbQfzWDZ2M4].f251.webm` |
| ~0.1 GiB | eight `.m4a` audio-only stubs | various `[id].m4a` |

Also delete the worse encode of an exact duplicate:

| Action | File |
|--------|------|
| **Delete** (VP9 6.31 GiB) | `Charlie Follows — 60 Min Yoga Flow Full Body Yoga for All Levels (1h00m22s, 4K).mp4` |
| **Keep** (AV1 3.33 GiB, same 4K + duration) | `… (1h00m22s, 4K).webm` |

**Tier A junk (~3.8 GiB) + worse duplicate encode (6.31 GiB) ≈ 10.1 GiB reclaim.**

### Tier B — optional delete if you refuse sub-HD (~21.4 GiB)

All **Gaia 480p/720p** titles. None have a 1080p counterpart in this folder. Content is unique; quality is clearly worse on a modern TV.

Delete these **only if** you are fine losing that Gaia material or will re-acquire it in HD later. Full list (25 files):

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
- `Gaia — Twists (26m03s, 480p).mp4` / `Power Yoga for Stamina` / `Session 2…` (~0.65–0.67 GiB each)
- `Gaia — Get Into Your Glutes (44m59s, 720p).mp4` (0.67 GiB)
- `Gaia — Connect to Your Devotion (19m19s, 720p).mp4` (0.50 GiB)
- `Gaia — Upper Body On The Go` / `Standing Poses` / `Water's Reflection` / `Creativity` / `Reflection` / `Centering` (0.18–0.47 GiB)

### Tier C — do **not** mass-delete

| Keep | Why |
|------|-----|
| Gaia 1080p (373 files, ~276 GiB) | Majority of the library; solid HD |
| Charlie Follows / YouTube 4K | Best visual quality; already a minority of file count |
| Other 4K creators | Small footprint |

Avoid “delete all 4K to save space” unless storage is critical — that frees ~132 GiB but throws away the best-looking workouts. Prefer re-encoding obese VP9/high-bitrate 4K to AV1 later if you need space without losing resolution.

### Suggested policy going forward

1. Run Tier A cleanup now.
2. Decide consciously on Tier B (Gaia ≤720p): keep for completeness, or drop for a clean HD+ library.
3. When downloading YouTube yoga: prefer merged AV1/WebM; delete `.f###.*`, `.temp.*`, `.part`, and `.m4a` after a successful mux.
4. Prefer one file per title: if both VP9/MP4 and AV1/WebM exist at the same resolution, keep the smaller efficient encode.

## Verdict

You should **not** wipe the library for being “old and low-res.” ~95% of bytes are already 1080p or 4K. The clear cleanup is **junk/incomplete downloads (~4 GiB)** plus one **worse 4K duplicate (~3 GiB)**; optionally drop **Gaia 480p/720p (~21 GiB)** if you only want HD+.
