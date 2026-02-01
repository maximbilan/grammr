# grammr

> Lightning-fast AI grammar checker in your terminal ⚡

**grammr** (yes, it's misspelled on purpose) is a blazingly fast TUI grammar checker powered by OpenAI. Copy text from anywhere, fix it in seconds, and paste it back. No UI, no bloat, just speed.

## Why grammr?

- **Fast**: Sub-3-second workflow from copy to paste
- **Keyboard-only**: Vim-inspired keybindings
- **AI-powered**: GPT-4o quality beats rule-based checkers
- **Offline cache**: Already-checked text loads instantly
- **Beautiful**: Colorful diffs, clean interface
- **Private**: Runs locally, API calls only for corrections

## Install

```bash
go install github.com/maximbilan/grammr@latest
```

Or build from source:
```bash
git clone https://github.com/maximbilan/grammr
cd grammr
go build -o grammr
```

## Setup

1. Get an API key from [OpenAI](https://platform.openai.com/api-keys)
2. Configure grammr:
```bash
grammr config set api_key YOUR_API_KEY
```

Optional: Choose a model (default: gpt-4o)
```bash
grammr config set model gpt-4o-mini  # Faster and cheaper
```

## Usage

1. Copy text from anywhere (Cmd+C / Ctrl+C)
2. Run `grammr`
3. Press `V` to paste
4. Wait ~1s for AI correction
5. Press `C` to copy result
6. Paste back to your app (Cmd+V / Ctrl+V)

That's it! 🎉

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `V` | Paste from clipboard |
| `C` | Copy corrected text |
| `E` | Edit corrected text |
| `O` | Edit original text |
| `R` | Retry correction |
| `D` | Toggle diff view |
| `Q` | Quit |
| `Ctrl+V` | Paste & auto-correct |
| `Ctrl+C` | Copy & quit |
| `?` or `F1` | Show help |

### Modes

Switch correction styles:
- `1` - Casual (default)
- `2` - Formal
- `3` - Academic
- `4` - Technical

## Configuration

Edit `~/.grammr/config.yaml`:
```yaml
api_key: "sk-..."
model: "gpt-4o"  # or gpt-4o-mini
mode: "casual"
cache_enabled: true
cache_ttl_days: 7
show_diff: true
auto_copy: false
```

Or use the CLI:
```bash
grammr config set model gpt-4o-mini
grammr config get model
```

## Model Comparison

| Model | Speed | Cost | Quality |
|-------|-------|------|---------|
| gpt-4o | Fast | Medium | Excellent |
| gpt-4o-mini | Very Fast | Cheap | Very Good |

**Recommendation**: Start with `gpt-4o-mini` for speed and cost, upgrade to `gpt-4o` if you need better quality.

## Examples

**Quick fix:**
```bash
grammr
# Press V, wait, press C
```

**Change mode:**
```bash
# Press 2 for formal writing
# Press V to paste
```

**Clear cache:**
```bash
rm -rf ~/.grammr/cache/
```

## Features

- ✅ Real-time streaming corrections
- ✅ Smart caching (hash-based)
- ✅ Beautiful colored diffs
- ✅ Multiple writing modes
- ✅ Vim-inspired keybindings
- ✅ Cross-platform (macOS, Linux, Windows)
- ✅ Single binary, no dependencies

## Why the weird name?

Because a grammar checker with a misspelled name is hilariously ironic. Also, it's shorter to type. 😄

## Roadmap

- [ ] Support for Anthropic Claude (in addition to OpenAI)
- [ ] Custom system prompts
- [ ] Plugin system for custom corrections
- [ ] Multi-language support
- [ ] Batch file processing

## License

See LICENSE file for details.
