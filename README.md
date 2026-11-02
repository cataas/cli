# cataas CLI

Cats in your terminal, powered by [Cat as a Service](https://cataas.com) 🐱

```bash
cataas cute --says "Hello"
```

The picture is drawn directly in the terminal:

- **Kitty graphics protocol** (Kitty, Ghostty) and **iTerm2 inline images** (iTerm2, WezTerm, VS Code) for full-resolution pictures;
- **24-bit colour half-blocks** (`▀`) everywhere else, including animated GIFs.

## Install

On Linux or macOS (amd64 / arm64):

```bash
curl -fsSL https://raw.githubusercontent.com/cataas/cli/main/install.sh | bash
```

The script downloads the matching prebuilt binary from [`bin/`](bin), checks its SHA-256 and
installs it in `/usr/local/bin` (or `~/.local/bin` when not writable).
Set `CATAAS_INSTALL_DIR` to choose another directory, or `CATAAS_REF` to install from a branch or tag.

You can also download a binary from [`bin/`](bin) yourself, or build it with Go 1.22+:

```bash
go install github.com/cataas/cli/cmd/cataas@latest
```

Or from a clone:

```bash
make build
```

## Usage

```
cataas [options] [tag...]      show a random cat (optionally having all the tags)
cataas tags [filter]           list available tags
cataas list [options]          list cats (id and tags)
cataas version                 print the version
```

### Examples

| Command | Result |
| --- | --- |
| `cataas` | a random cat |
| `cataas cute orange` | a random cat tagged both `cute` and `orange` |
| `cataas --gif` | a random animated cat |
| `cataas --says "Hello" --font-color orange --font-size 40` | a cat saying hello |
| `cataas --filter mono --type square` | a black and white square cat |
| `cataas --filter custom --r 7 --g 12 --b 50` | a cat with tinted colours |
| `cataas --id 595f280c557291a9750ebf80` | a specific cat |
| `cataas --output cat.jpg` or `cataas > cat.jpg` | save the picture |
| `cataas --url --says hi` | print the picture URL without downloading it |
| `cataas tags cat` | tags containing "cat" |
| `cataas list --tag cute --limit 5 --json` | cats metadata as JSON |

### Options

| Option | Description |
| --- | --- |
| `-t, --tag` | only cats having this tag (repeatable, or comma separated) |
| `--id` | a specific cat |
| `--gif` | an animated cat |
| `-s, --says` | text written on the picture |
| `--font`, `--font-size`, `--font-color`, `--font-background` | text style |
| `--type` | `xsmall`, `small`, `medium` or `square` |
| `--width`, `--height`, `--fit`, `--position` | server-side resize |
| `--blur` | blur radius |
| `--filter` | `mono`, `negate` or `custom` |
| `--r`, `--g`, `--b`, `--brightness`, `--saturation`, `--hue`, `--lightness` | adjustments for `--filter custom` |
| `-o, --output` | save to a file (`-` for stdout) |
| `--url` | only print the URL |
| `--cols` | width in terminal columns (default: fits the terminal, max 100) |
| `--mode` | `auto`, `kitty`, `iterm` or `blocks` |
| `--loops` | how many times animated cats play in `blocks` mode (default 3) |
| `--base-url` | API base URL |

When the output is not a terminal (pipe or redirection), the raw picture is written instead of being drawn.

### Environment

| Variable | Description |
| --- | --- |
| `CATAAS_URL` | API base URL (default `https://cataas.com`) |
| `CATAAS_MODE` | default render mode |

## Development

```bash
make test
```

After changing the code, rebuild the committed binaries (and their `SHA256SUMS`) before committing:

```bash
make dist VERSION=x.y.z
```
