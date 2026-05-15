<!-- Version: 0.6 | Last updated: 2026-05-13 -->

# Architecture

## Overview

`pix` is a single-binary CLI tool written in Go. It has no runtime dependencies beyond the binary itself (ImageMagick is optional for format conversion). The architecture is deliberately minimal -- a handful of files in package `main`, no subdirectories, no abstractions beyond what subcommand dispatch requires.

## Components

```
                    ┌────────────────┐
stdin (prompt)─────►│                │─► FAL API ─► image file
                    │   pix binary   │─► FAL pricing API ─► cost to stderr
config.yaml ───────►│                │─► preview command ─► image viewer
                    └────────────────┘
                          │
                    ┌─────┴─────┐
                    │           │
                generate       cost
```

### File layout

All code lives in package `main` at the project root.

| File | Purpose |
|------|---------|
| `main.go` | Entry point, global flag parsing, subcommand dispatch |
| `genimg.go` | `pix generate` handler (alias `gen`) -- generates or edits images from prompts |
| `cost.go` | `pix cost` handler -- queries pricing without generation |
| `config.go` | `config.yaml` loading, API key resolution, config directory resolution |
| `fal.go` | FAL API HTTP helpers (generation, pricing, historical estimate) |
| `prompts.go` | `--load-prompt` flow -- enumerates saved-prompt files, invokes the configured picker, assembles base + optional appended text |
| `models.go` | `--pick-model` flow -- fetches FAL `/v1/models` (image categories), invokes the configured picker, returns the selected `endpoint_id` |
| `helptext.go` | Loads embedded `docs/helptext/*.md` files via `//go:embed` and writes them to stderr on `--help` |
| `docs/helptext/*.md` | Source of truth for `--help` output: one file per subcommand (`pix.md`, `generate.md`, `cost.md`). Embedded at compile time so the binary stays a single artefact, while the text lives alongside the rest of the project documentation. |

### Subcommands

| Subcommand | Purpose |
|------------|---------|
| `generate [refs...] <output>` | Reads prompt from stdin, writes image to disk. Earlier positionals that exist as image files become reference images (max 3) and pix uses the FAL edit endpoint. Alias: `gen`. |
| `cost` | Queries pricing for configured model |

Each subcommand parses its own flags, including a subcommand-specific `--help` / `-h` and `--dry-run` (where applicable).

### Flag system

`main.go` performs a single-pass classification of `os.Args[1:]`:

- Recognized global flags (`-q`/`--quiet`, `--version`) are consumed regardless of position.
- `-h`/`--help` is top-level when seen before any non-flag token, subcommand-level otherwise.
- The first non-flag token is the subcommand; remaining flags and positionals pass through to the subcommand handler.

To the user, flag position is irrelevant -- `pix --no-load-prompt gen out.png`, `pix gen --no-load-prompt out.png`, and `pix gen out.png --no-load-prompt` are equivalent. `--help` remains mutually exclusive with every other flag/positional. See issue #11 for design history.

### Configuration

Two files in the config directory (`~/.config/pix/` or next to the binary):

- **`config.yaml`** -- model, API key sources, preview command, interactive-picker settings
- **`.env`** -- fallback API key storage (optional, legacy)

The config directory is resolved at runtime by checking for `config.yaml` or `.env` next to the binary first, then falling back to the XDG location. This allows development (config next to binary) and installed (XDG) use without any flag or env var.

`config.yaml` has four top-level keys plus an optional `interactive:` block:

- `model:` -- default FAL endpoint_id
- `api-keys.<provider>.{command, file}` -- API key resolution (sh -c command or filesystem path; both tilde-expanded)
- `preview-command:` -- command for `-p`/`--preview` (sh -c)
- `interactive:` -- TTY-only behaviour (picker, prompt-picker, load-prompt, model-picker). Piped/scripted invocations silently bypass everything under this block.

The `interactive:` block separates picker behaviour (`prompt-picker.{always, filter}`, `model-picker.{always, filter, preselect}`) from data sources (`load-prompt.path`). `interactive.picker` is the single source of truth for the picker command shared by both flows. See issue #12.

### API integration

The tool calls three FAL endpoints:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `https://fal.run/{model}` | POST | Image generation (`generate`) |
| `https://api.fal.ai/v1/models/pricing?endpoint_id={model}` | GET | Unit price lookup (`cost`, post-generation cost) |
| `https://api.fal.ai/v1/models/pricing/estimate` | POST | Historical cost estimate (`cost`) |
| `https://api.fal.ai/v1/models?category={cat}&status=active` | GET | Model catalogue for `--pick-model`; `cat` is `text-to-image` (no refs) or `image-to-image` (refs present) |

All use `Authorization: Key {fal_key}` headers. The `FAL_BASE_URL` environment variable redirects all endpoints to a test server via `httptest.NewServer`.

### Testing

The regression test suite runs the compiled binary as a subprocess via `os/exec`. The FAL API is intercepted using Go's `httptest.NewServer` -- no real API calls are made during `make test`. The Makefile sets `HOME` to a temp directory to prevent personal config from leaking into tests.

## Design decisions

| Decision | Rationale |
|----------|-----------|
| Subcommand structure (vs single-purpose binaries) | Discoverable surface, single config, single install. Future operations land cleanly. |
| Multi-file package main (vs monolithic main.go) | Each subcommand and concern in its own file -- navigability without architectural overhead. |
| Position-irrelevant flag parsing (since #11) | The original strict-position rule was friction in practice; even the author got tripped up. Single-pass scan classifies by name, treats first non-flag as subcommand. |
| `interactive:` block (since #12) | TTY-only settings live under one parent so the schema teaches the rule. Piped/scripted invocations silently bypass everything in the block. Picker behaviour separates from data sources (prompt-picker vs load-prompt). |
| Go, not Python | Single static binary. No venv, no pip, no runtime. Trivial cross-compilation. |
| No FAL SDK | The FAL API is a handful of HTTP calls. A dependency is not justified. |
| `sh -c` for user commands | Config commands (key retrieval, preview) are user-specified shell expressions. |
| Extension from Content-Type | The FAL API returns JPEG by default regardless of what the user requests. Detecting and handling this is better than surprising the user with a misnamed file. |

## Roadmap

Future enhancements, in rough priority order:

| Feature | Description | Complexity |
|---------|-------------|------------|
| Reference image / edit mode | Reference image support added to `pix generate` via positional args. Uses FAL's `/edit` endpoint with `image_urls` parameter. Implemented in #4. | Done |
| Editor invocation for prompts | Allow `$VISUAL`/`$EDITOR` to open the selected saved prompt for free-form editing instead of single-line append. Deferred from [#8](https://github.com/tadg-paul/pix/issues/8). | Medium |
| Model picker (`--pick-model`) | Fetches FAL `/v1/models` for the active category and presents the catalogue via the configured picker. `model-picker.preselect` surfaces a habitual default. Implemented in #10 and #12. | Done |
| `--model` flag | Override `config.yaml` model per invocation without the picker. Enables side-by-side comparisons in scripts. | Small |
| Pricing in model picker | Lazy-fetch pricing for the selected model after the picker exits and before the FAL call. Currently pricing is fetched per-invocation only after generation. | Medium |
| Local cache for `/v1/models` | Cache the model catalogue to `~/.cache/pix/models.json` with a TTL so the picker doesn't fetch on every invocation. | Small |
| Image dimensions | Support `--aspect-ratio` or `--size` presets. FAL API accepts `aspect_ratio` ("1:1", "16:9") and `resolution` ("1k", "2k"). | Small |
| Homebrew formula | Cross-compiled binaries for Darwin/Linux/Windows. `make release` with GitHub releases. See [#3](https://github.com/tadg-paul/pix/issues/3). | Medium |
| Batch mode | Accept multiple prompts (one per line), generate in parallel. | Medium |
| Cost tracking | Cumulative cost log for budgeting across sessions. | Medium |
| Prompt templates | Reusable prefix/suffix fragments in config. | Small |
| Provider abstraction | Support APIs beyond FAL (e.g. Replicate, direct vendor APIs). Not planned until a concrete need arises. | Large |
