# grammr

> Lightning-fast AI grammar checker in your terminal ⚡

**grammr** (yes, it's misspelled on purpose) is a blazingly fast TUI grammar checker powered by OpenAI or Anthropic Claude. Copy text from anywhere, fix it in seconds, and paste it back. No UI, no bloat, just speed.

## Why grammr?

- **Fast**: Sub-3-second workflow from copy to paste
- **Keyboard-only**: Vim-inspired keybindings
- **AI-powered**: GPT-4o and Claude quality beats rule-based checkers
- **Translation**: Built-in AI translation to any language
- **Offline cache**: Already-checked text loads instantly
- **Beautiful**: Colorful diffs, clean interface
- **Private**: Runs locally, API calls only for corrections

## Install

### Homebrew (macOS)

**Option 1: Direct installation (recommended)**

```bash
brew tap maximbilan/grammr https://github.com/maximbilan/grammr
brew install grammr
```

**Option 2: Using the installation script**

```bash
curl -fsSL https://raw.githubusercontent.com/maximbilan/grammr/main/install.sh | bash
```

Or for a specific version:
```bash
curl -fsSL https://raw.githubusercontent.com/maximbilan/grammr/main/install.sh | bash -s v1.0.0
```

**Option 3: Direct formula URL**

```bash
brew install --build-from-source https://raw.githubusercontent.com/maximbilan/grammr/v1.0.0/Formula/grammr.rb
```

Replace `v1.0.0` with the desired version tag.

### Go Install

```bash
go install github.com/maximbilan/grammr@latest
```

### Build from Source

```bash
git clone https://github.com/maximbilan/grammr
cd grammr
go build -o grammr
```

## Setup

1. Get an API key from [OpenAI](https://platform.openai.com/api-keys) or [Anthropic](https://console.anthropic.com/)
2. Initialize configuration (optional, creates config directory):
```bash
grammr config init
```

3. Configure grammr:

**For OpenAI:**
```bash
grammr config set provider openai
grammr config set api_key YOUR_OPENAI_API_KEY
```

**For Anthropic Claude:**
```bash
grammr config set provider anthropic
grammr config set anthropic_api_key YOUR_ANTHROPIC_API_KEY
```

Optional: Choose a model (default: gpt-4o for OpenAI, claude-3-5-sonnet-20241022 for Anthropic)
```bash
grammr config set model gpt-4o-mini  # OpenAI: Faster and cheaper
grammr config set model claude-3-5-sonnet-20241022  # Anthropic: Latest Claude model
```

Optional: Set language (default: english)
```bash
grammr config set language spanish  # For Spanish text correction
```

Optional: Enable translation to a target language
```bash
grammr config set translation_language french  # Translate corrected text to French
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

**Global Mode:**
| Key | Action |
|-----|--------|
| `V` | Paste from clipboard |
| `C` | Copy corrected text |
| `T` | Copy translation (if translation enabled) |
| `E` | Edit corrected text |
| `O` | Edit original text |
| `R` | Retry correction |
| `D` | Toggle diff view |
| `A` | Review changes word-by-word |
| `Q` | Quit |
| `Ctrl+V` | Paste & auto-correct |
| `Ctrl+C` | Copy & quit |
| `?` or `F1` | Show help |

**Edit Mode:**
| Key | Action |
|-----|--------|
| `Esc` | Exit edit mode |
| `Ctrl+S` | Save and re-correct (original only) |

**Review Mode:**
| Key | Action |
|-----|--------|
| `Tab` | Apply current change |
| `Space` | Skip current change |
| `Esc` | Exit review mode |

### Styles

Switch correction styles:
- `1` - Casual (default)
- `2` - Formal
- `3` - Academic
- `4` - Technical

## Configuration

Edit `~/.grammr/config.yaml`:
```yaml
provider: "openai"  # or "anthropic"
api_key: "sk-..."  # OpenAI API key
anthropic_api_key: "sk-ant-..."  # Anthropic API key (if using Anthropic)
model: "gpt-4o"  # OpenAI: gpt-4o, gpt-4o-mini | Anthropic: claude-3-5-sonnet-20241022, claude-3-opus-20240229, etc.
style: "casual"  # or use "mode" for backward compatibility
language: "english"  # Default: english. Options: english, spanish, french, german, etc.
translation_language: ""  # Optional: Translate corrected text to this language (e.g., "spanish", "french", "german")
cache_enabled: true
cache_ttl_days: 7
show_diff: true
auto_copy: false
```

Or use the CLI:
```bash
grammr config set provider anthropic
grammr config set anthropic_api_key sk-ant-...
grammr config set model claude-3-5-sonnet-20241022
grammr config set language spanish
grammr config set translation_language french
grammr config get provider
grammr config get language
grammr config get translation_language
```

## Model Comparison

### OpenAI Models
| Model | Speed | Cost | Quality |
|-------|-------|------|---------|
| gpt-4o | Fast | Medium | Excellent |
| gpt-4o-mini | Very Fast | Cheap | Very Good |

### Anthropic Models
| Model | Speed | Cost | Quality |
|-------|-------|------|---------|
| claude-3-5-sonnet-20241022 | Fast | Medium | Excellent |
| claude-3-opus-20240229 | Medium | High | Excellent |
| claude-3-haiku-20240307 | Very Fast | Cheap | Very Good |

**Recommendation**: 
- **OpenAI**: Start with `gpt-4o-mini` for speed and cost, upgrade to `gpt-4o` if you need better quality.
- **Anthropic**: Start with `claude-3-5-sonnet-20241022` for the best balance, use `claude-3-haiku-20240307` for speed/cost, or `claude-3-opus-20240229` for maximum quality.

## Examples

**Quick fix:**
```bash
grammr
# Press V, wait, press C
```

**Change style:**
```bash
# Press 2 for formal writing
# Press V to paste
```

**Review changes word-by-word:**
```bash
grammr
# Press V to paste
# Press A to enter review mode
# Press Tab to apply changes, Space to skip
# Press Esc when done
```

**Use translation:**
```bash
# First, configure translation language
grammr config set translation_language spanish

# Then use grammr normally
grammr
# Press V to paste
# After correction completes, translation appears automatically
# Press T to copy translation
```

**Clear cache:**
```bash
rm -rf ~/.grammr/cache/
```

**Initialize config:**
```bash
grammr config init
```

## Features

- ✅ Real-time streaming corrections
- ✅ AI-powered translation to any language
- ✅ Smart caching (hash-based, configurable TTL)
- ✅ Beautiful colored diffs
- ✅ Word-by-word change review mode
- ✅ Multiple writing modes (casual, formal, academic, technical)
- ✅ Inline text editing
- ✅ Vim-inspired keybindings
- ✅ Cross-platform (macOS, Linux, Windows)
- ✅ Single binary, no dependencies
- ✅ Comprehensive test suite

## Why the weird name?

Because a grammar checker with a misspelled name is hilariously ironic. Also, it's shorter to type. 😄

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o grammr
```

### Test Coverage

The project includes comprehensive unit tests covering:
- Cache operations (hash, get, set, expiration)
- Configuration management (load, save, set, get)
- Corrector initialization and prompt building
- UI utility functions (diff parsing, text building, whitespace trimming)

## Roadmap

- [x] Support for Anthropic Claude (in addition to OpenAI)
- [ ] Custom system prompts
- [ ] Plugin system for custom corrections
- [ ] Batch file processing

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See LICENSE file for details.
