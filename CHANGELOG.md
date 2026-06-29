# Changelog

All notable changes to `offload-harness` are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [SemVer](https://semver.org/).

## [0.4.1] — 2026-06-28

### Fixed
- **The shadow-labeling flywheel now actually manufactures counterfactual labels.** Two compounding bugs had left it producing ~0 labels:
  - **Config silently ignored by the MCP server.** A bare `local-offload mcp` (host passes neither `--config` nor `$LOCAL_OFFLOAD_CONFIG`) ran on built-in defaults with shadow capture **off**. `loadCfg` now also auto-discovers `~/.local-offload/config.json` when both are unset (new `resolveCfgPath`; precedence: flag → env → conventional path → defaults).
  - **Healthy entry tiers were route-skipped.** `internal/health` flagged tiers DEGRADED on margin/throughput **drift** (CUSUM/Page-Hinkley) or throughput collapse, and the cascade routed *around* any DEGRADED tier — so an accurate small entry tier that was merely non-stationary (single-GPU throughput variance) got skipped to a larger, slower one, starving the flywheel of entry-tier data. Health now separates a `route_skip` signal (true only on a genuine **quality collapse** — confidence margin far below the tier's own baseline) from the observability `Status` (any drift/throughput anomaly); only `route_skip` populates the routing skip-list. Drift/throughput remain visible for timeout tuning.
- The CLI `version` string now matches the `VERSION` file (was a stale `0.1.0`).

## [0.4.0] — 2026-06-28

### Added
- First public release. 0.4.0 reflects the already-mature harness (core text offload + full self-learning cascade + flywheel + kNN + vision/STT/video understanding + image & SVG generation); media generation, DaVinci editing, and the capstone remain on the roadmap.
- Text offload tools — **summarize, classify, extract, triage** — on a free local Gemma-4 cascade via llama.cpp; never calls a cloud model (returns a structured **defer** on low confidence).
- **MCP server** (stdio) exposing 12 tools, plus a Go CLI.
- **Vision**: VQA, OCR, image field-extraction, and render QA (`assess-image`).
- **Speech-to-text** via whisper.cpp (`transcribe`) and **video understanding** (`video-describe`).
- **Image generation** (SDXL via ComfyUI) and a dependency-free **SVG component kit** (gauge / comparison-bar / chromatogram / icons).
- **Self-learning cascade**: confidence-gated escalation, conformal thresholds, a logistic entry-tier router, health/circuit-breakers, few-shot exemplars, and an offline shadow-labeling flywheel — all inference-free over the token ledger.
- Append-only JSONL **token-savings ledger**.
