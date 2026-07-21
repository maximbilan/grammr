#!/usr/bin/env python3
import os, re, html, sys

ANSI_DIR = sys.argv[1]
OUT_DIR  = sys.argv[2]
os.makedirs(OUT_DIR, exist_ok=True)

# 16-color ANSI -> hex (Tokyo Night, to match the article art)
FG = {
    30:"#15161e",31:"#f7768e",32:"#9ece6a",33:"#e0af68",34:"#7aa2f7",
    35:"#bb9af7",36:"#7dcfff",37:"#a9b1d6",
    90:"#565f89",91:"#f7768e",92:"#9ece6a",93:"#e0af68",94:"#7aa2f7",
    95:"#bb9af7",96:"#7dcfff",97:"#c0caf5",
}
BG = {
    40:"#15161e",41:"#f7768e",42:"#9ece6a",43:"#e0af68",44:"#7aa2f7",
    45:"#bb9af7",46:"#7dcfff",47:"#a9b1d6",
    100:"#565f89",101:"#f7768e",102:"#9ece6a",103:"#e0af68",104:"#7aa2f7",
    105:"#bb9af7",106:"#7dcfff",107:"#c0caf5",
}
DEFAULT_FG = "#c0caf5"

SGR = re.compile(r"\x1b\[([0-9;]*)m")

def new_state():
    return {"fg":None,"bg":None,"bold":False,"italic":False,"strike":False,"under":False}

def style_css(s):
    parts = []
    fg = s["fg"] or DEFAULT_FG
    parts.append(f"color:{fg}")
    if s["bg"]:
        parts.append(f"background:{s['bg']};border-radius:3px")
    if s["bold"]:   parts.append("font-weight:700")
    if s["italic"]: parts.append("font-style:italic")
    decos = []
    if s["strike"]: decos.append("line-through")
    if s["under"]:  decos.append("underline")
    if decos: parts.append("text-decoration:"+" ".join(decos))
    return ";".join(parts)

def apply(state, params):
    if params == "" :
        params = "0"
    for p in params.split(";"):
        if p == "": p = "0"
        n = int(p)
        if n == 0: state.update(new_state())
        elif n == 1: state["bold"] = True
        elif n == 3: state["italic"] = True
        elif n == 4: state["under"] = True
        elif n == 9: state["strike"] = True
        elif n == 22: state["bold"] = False
        elif n == 23: state["italic"] = False
        elif n == 24: state["under"] = False
        elif n == 29: state["strike"] = False
        elif n == 39: state["fg"] = None
        elif n == 49: state["bg"] = None
        elif n in FG: state["fg"] = FG[n]
        elif n in BG: state["bg"] = BG[n]

def to_html(text):
    state = new_state()
    out = []
    open_span = False
    def close():
        nonlocal open_span
        if open_span:
            out.append("</span>")
            open_span = False
    def openspan():
        nonlocal open_span
        css = style_css(state)
        out.append(f'<span style="{css}">')
        open_span = True
    pos = 0
    # start a span for the initial (default) state so everything is wrapped
    openspan()
    for m in SGR.finditer(text):
        chunk = text[pos:m.start()]
        if chunk:
            out.append(html.escape(chunk))
        close()
        apply(state, m.group(1))
        openspan()
        pos = m.end()
    tail = text[pos:]
    if tail:
        out.append(html.escape(tail))
    close()
    return "".join(out)

def measure(text):
    plain = SGR.sub("", text)
    lines = plain.split("\n")
    rows = len(lines)
    cols = max((len(l) for l in lines), default=0)
    return rows, cols

TITLES = {
    "main":"grammr — zsh",
    "review":"grammr — zsh",
    "help":"grammr — zsh",
    "translation":"grammr — zsh",
}

PAGE = """<!doctype html><html><head><meta charset="utf-8"><style>
*{{margin:0;padding:0;box-sizing:border-box}}
html,body{{width:100%;height:100%}}
body{{font-family:'Menlo','Monaco','SF Mono',ui-monospace,monospace;
 display:flex;align-items:center;justify-content:center;
 background:
   radial-gradient(1200px 600px at 20% -10%, rgba(122,162,247,.16), transparent 60%),
   radial-gradient(1000px 600px at 100% 120%, rgba(187,154,247,.14), transparent 55%),
   #0b0c16;}}
.win{{background:#1a1b26;border-radius:12px;overflow:hidden;
 box-shadow:0 40px 90px rgba(0,0,0,.55),0 0 0 1px rgba(122,162,247,.10);
 border:1px solid #2a2e42;}}
.bar{{height:42px;background:#20222f;display:flex;align-items:center;padding:0 16px;gap:9px;border-bottom:1px solid #2a2e42}}
.dot{{width:12px;height:12px;border-radius:50%}}
.red{{background:#f7768e}}.yel{{background:#e0af68}}.grn{{background:#9ece6a}}
.bar .t{{flex:1;text-align:center;color:#565f89;font-size:13px;margin-right:54px}}
.scr{{padding:20px 24px}}
.scr pre{{font-size:15px;line-height:1.18;white-space:pre;color:#c0caf5}}
</style></head><body style="width:{cw}px;height:{ch}px">
<div class="win"><div class="bar"><span class="dot red"></span><span class="dot yel"></span><span class="dot grn"></span><span class="t">{title}</span></div>
<div class="scr"><pre>{body}</pre></div></div>
</body></html>"""

CW, LH = 9.04, 17.7   # per-char width, line height in px at 15px Menlo
manifest = []
for fn in sorted(os.listdir(ANSI_DIR)):
    if not fn.endswith(".ansi"): continue
    name = fn[:-5]
    raw = open(os.path.join(ANSI_DIR, fn), encoding="utf-8").read()
    raw = raw.rstrip("\n")
    rows, cols = measure(raw)
    frag = to_html(raw)
    inner_w = cols * CW + 2*24          # pre + .scr padding
    inner_h = rows * LH + 2*20 + 42     # pre + padding + title bar
    win_w = inner_w + 2                 # borders
    win_h = inner_h + 2
    cw = int(win_w + 200)               # canvas margins
    ch = int(win_h + 160)
    htmlout = PAGE.format(cw=cw, ch=ch, title=TITLES.get(name,"grammr — zsh"), body=frag)
    open(os.path.join(OUT_DIR, name+".html"), "w", encoding="utf-8").write(htmlout)
    manifest.append(f"{name} {cw} {ch}")
    print(f"{name}: {rows} rows x {cols} cols -> canvas {cw}x{ch}")

open(os.path.join(OUT_DIR, "manifest.txt"), "w").write("\n".join(manifest))
