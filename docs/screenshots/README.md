# Screenshots

These images are captured straight from grammr's real `View()` output — same
box-drawing borders, same diff engine, same layout the app renders in a
terminal. No mockups, and no network calls: the model is populated with sample
text directly.

| File | View |
|------|------|
| `main.png` | Correction view with the colored diff |
| `review.png` | Word-by-word review mode |
| `help.png` | Keyboard shortcuts overlay |
| `translation.png` | Correction plus automatic translation |

## Regenerating

Two steps. The first is pure Go; the second turns the captured ANSI into PNGs
and needs Python 3 and Google Chrome.

```bash
# 1. Render the real TUI views to .ansi files (no API calls, guarded test)
GRAMMR_SHOT_DIR=/tmp/grammr-shots go test ./internal/ui -run TestGenerateScreenshots -count=1

# 2. Convert the ANSI captures into HTML, then screenshot them with headless Chrome
python3 docs/screenshots/convert.py /tmp/grammr-shots /tmp/grammr-html
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
while read -r name w h; do
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=2 \
    --screenshot="docs/screenshots/$name.png" --window-size=$w,$h "/tmp/grammr-html/$name.html"
done < /tmp/grammr-html/manifest.txt
```

The generator lives in `internal/ui/screenshots_test.go` and is skipped during
normal test runs (it only does anything when `GRAMMR_SHOT_DIR` is set).
