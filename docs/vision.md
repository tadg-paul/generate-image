<!-- Version: 0.7 | Last updated: 2026-05-13 -->

# Vision

## The Holy Grail

A useful CLI (and maybe GUI) to let you create and edit images using generative AI models, running locally on your device. Dream on.

## And back down to earth ...

`pix` is a minimal CLI tool that interacts with the [FAL API](https://fal.ai) for image-related operations. It is built around subcommands so that distinct operations remain discoverable and individually testable as the tool grows.

```bash
echo "a sunset over Dublin Bay" | pix gen sunset
cat description.txt | pix gen --preview poster
echo "make it dramatic" | pix gen photo.jpg dramatic.jpg
pix cost
```

## Goals

- **Subcommand-based:** every distinct operation is its own subcommand. New features extend the surface, they don't bloat existing commands.
- **Zero friction:** `make install` places the binary on `PATH`; a YAML config file is the only setup.
- **Pipeline-friendly:** reads stdin, writes files, reports status to stderr. No interactive prompts by default.
- **Reusable prompts:** an opt-in `--load-prompt` flow lets users keep a directory of saved prompts and pick one interactively via fzf (or any configured picker), optionally appending text on the fly.
- **Model discovery:** an opt-in `--pick-model` flow fetches FAL's `/v1/models` catalogue and lets the user pick the model for an invocation interactively. A `model-picker.preselect` config knob keeps a habitual default at the top of the picker.
- **Interactive when interactive, scriptable when scripted:** every picker and prompt only fires when stdin is a TTY. Piped or redirected invocations silently bypass them, so scripts stay scripts.
- **Cost-aware:** reports generation cost when the FAL pricing API has data; standalone cost lookup via `pix cost`.

## Non-goals (for now)

- GUI or web interface.
- Batch generation or multi-image output.
- Video generation.
- Provider abstraction (FAL is the only backend).

## Subcommands

### `pix generate [refs...] <output>`

Reads a text prompt from stdin and generates an image via the FAL API. Writes the result to `<output>` (extension optional -- if omitted, the API response format is used). Alias: `gen`.

`generate` also accepts reference images: if earlier positional arguments exist as image files (max 3), they are sent to the FAL edit endpoint. So `pix gen out.png` is text-to-image; `pix gen photo.jpg out.png` is edit. One subcommand, two modes, decided by what's on the command line.

### `pix cost`

Queries pricing for the configured model without generating an image. Reports both the unit price and a historical estimate based on past usage.

## Flag system

Recognized flags may appear in any position (before, after, or interleaved with positional arguments). The subcommand is the first non-flag token.

- **Global flags:** `--quiet` / `-q`, `--version`, `--help` / `-h`.
- **Subcommand flags** (on `generate`): `--dry-run`, `--preview` / `-p`, `--load-prompt` / `--no-load-prompt`, `--pick-model` / `--no-pick-model`, `--help` / `-h`.
- **Subcommand flags** (on `cost`): `--dry-run`, `--help` / `-h`.

`--help` is mutually exclusive with all other flags and arguments. The position of `--help` selects which help text renders: `pix --help` shows the top level, `pix <subcommand> --help` shows that subcommand's specific help.

## Configuration

### `config.yaml`

```yaml
model: xai/grok-imagine-image     # default model when --pick-model isn't in play

api-keys:
  fal:
    command: op read op://vault/fal/credential
    # file: /path/to/fal.key

preview-command: chafa

# Interactive-only -- only applies when stdin is a TTY. Piped/scripted runs bypass.
interactive:
  picker: fzf                       # shared picker for both flows (default: fzf)
  prompt-picker:
    always: false                   # if true, --load-prompt is implicit on every gen
    filter: ""                      # if set, fzf opens with this query pre-filled
  load-prompt:
    path: ~/snips/prompts-genai     # directory of saved prompt files
  model-picker:
    always: false                   # if true, --pick-model is implicit on every gen
    filter: ""                      # if set, fzf opens with this query pre-filled
    preselect: ""                   # endpoint_id to surface as the first candidate
```

The config file lives at `~/.config/pix/config.yaml`, with fallback to the binary directory for development.

### API key resolution priority

1. `FAL_KEY` environment variable
2. `api-keys.fal.command` in config (runs a command, uses stdout)
3. `api-keys.fal.file` in config (reads a file)
4. `.env` file in the config directory (legacy fallback)

## Technology

- **Language:** Go 1.22+
- **Dependencies:** `gopkg.in/yaml.v3` (config parsing). Standard library for everything else.
- **Optional:** ImageMagick (`magick`) for format conversion
- **Installation:** `make install` copies the binary to `~/.local/bin/pix`

## Licence

MIT -- Copyright Tadg Paul
