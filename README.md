# palmterm

**English** | [日本語](README.ja.md)

A web terminal for driving **tmux from your phone**. It is built for the things that are painful in
xterm.js on a touch screen — selecting text, sending Ctrl/Alt, moving the cursor, scrolling — and
for working with TUIs such as vim, lazygit and Claude Code.

<p align="center"><img src="docs/screenshot.png" width="360" alt="palmterm on a phone: tmux window tabs, a key bar and a text box"></p>

- Rendering uses [wterm](https://github.com/vercel-labs/wterm), which draws the terminal as DOM, so the
  OS's native text selection works.
- The server is a single Go binary with the web UI embedded. Each browser gets its own tmux client
  attached to one tmux session, so you can pick up on your phone where you left off on your PC.

## Features

### Typing

- **Text box** (always shown at the bottom): edit with the OS keyboard — cursor movement, IME,
  predictions and voice input all work.
  - Enter inserts a newline. Send with the send button, Ctrl+Enter (⌘+Enter), or the key bar's Ctrl
    followed by Enter. Multi-line text is sent as a bracketed paste, so Claude Code receives it as-is.
  - Backspace on an empty text box deletes in the terminal.
- **Direct input**: tap the terminal and type; every key goes straight to the terminal (for vim and TUIs).
- Whichever you typed in last receives Tab, paste and images.

### Key bar

- Esc, Tab, Ctrl/Alt/Shift, arrows, Enter, ^C, ^D, Home/End, PgUp/PgDn and a few symbols by default.
  The keys and their order are configurable (see [Configuration](#configuration)).
- **Modifiers**: one tap applies the modifier to the next key (outlined); a quick double tap locks it
  (filled); tap again to release. They also apply to keys typed on the soft keyboard.
- Arrows and PgUp/PgDn repeat while held. Arrow keys follow the application cursor mode (DECCKM).
- Pressing the key bar never pops up the soft keyboard, and never closes it while it is open.

### tmux windows and panes

- **Window tabs** are always shown above the key bar: tap to switch, **+** opens a new window in the
  current directory, **×** closes one (tap twice to confirm), and tapping the current tab renames it
  (an empty name restores tmux's automatic name).
- **The green tmux button**, pinned to the right of the key bar, opens a panel to split panes, zoom,
  close a pane (tap twice), move between panes, enter tmux copy mode, send the prefix (Ctrl+B), and
  change the font size.
- **Tap a pane** to select it.
- These actions run tmux commands on the server, so they work regardless of your prefix and key
  bindings, and with tmux's `mouse` option off.

### Scrolling and gestures

- **Swipe** vertically to scroll. palmterm checks the pane when the swipe starts:
  - a shell or Claude Code (no mouse, no alternate screen), or tmux copy mode: scrolls through tmux's
    history; scrolling back to the bottom leaves copy mode
  - apps that use the mouse (vim, lazygit, …): sends wheel events
  - full-screen apps without the mouse (less, …): sends ↑/↓
  - scrolling continues with momentum after you lift your finger
- **Pinch** to change the font size (8–32px).

### Copy and paste

- **Copy** (the two-sheets button) shows tmux's history (3000 lines) as plain text. Long-press to
  select, then “Copy selection” or “To input”.
- **Paste** (the clipboard button) pastes into the text box or the terminal. Browsers only allow this
  over HTTPS; over plain HTTP, long-press the text box and paste instead.

### Images (for Claude Code)

- Pick photos with the image button, or paste images into the text box or the terminal. They are saved
  on the server (`~/.cache/palmterm/uploads` by default).
- Formats Claude Code cannot read (HEIC, AVIF, …) are converted to JPEG with ffmpeg.
- While you type in the text box, images appear as thumbnails above it (remove one with ×). Sending
  waits for the uploads, pastes each image path, then the text and Enter — Claude Code attaches them as
  `[Image #1]`, …. If an upload failed, nothing is sent and you are told why.
- While you type directly in the terminal, each path is pasted as soon as its upload finishes.

### Fonts

- palmterm serves JetBrains Mono and Symbols Nerd Font Mono, so Nerd Font icons work on phones.
- As in WezTerm, kitty and Ghostty, an icon followed by a space is drawn two cells wide; otherwise it
  fits in one cell. Powerline separators are stretched to fill their cell.

### Other

- If the connection drops, the tmux session stays; reconnecting shows where you left off.
- The UI is available in English and Japanese.

## Requirements

- Linux (tested on Fedora) or another Unix with PTYs
- tmux 3.x
- Go 1.26+ and Node.js 22+ to build
- ffmpeg (optional; converts HEIC and other images for Claude Code)
- Chromium (only for the end-to-end tests)

## Getting started

```sh
git clone https://github.com/MoomA-0750/palmterm.git
cd palmterm
make          # builds the web UI and ./palmterm
./palmterm    # listens on 127.0.0.1:7681 and prints a login URL
```

Open the printed `http://127.0.0.1:7681/auth?token=…` URL once; the browser then stays logged in.
palmterm attaches to the tmux session `main` (creating it if needed).

### From your phone

**Tailscale (recommended).** Keep palmterm on 127.0.0.1 and let Tailscale serve it over HTTPS inside
your tailnet:

```sh
tailscale serve --bg 7681
```

**Local network.** Also listen on the LAN over HTTPS:

```sh
./palmterm -lan :7682
```

palmterm listens on each of the PC's private LAN IPv4 addresses (and its Tailscale address) and prints
`https://<address>:7682/auth?token=…` for each. Give an address instead (`-lan 192.168.1.5:7682`) to
listen on that one only. It uses a self-signed certificate that it creates in `~/.config/palmterm/`
(recreated when a new IP address appears, keeping the old ones), so the browser warns you the first
time — choose “Advanced” and proceed. The LAN side uses HTTPS so the
token is not sent in clear text and the paste button works.

### Options

| Flag | Default | |
|---|---|---|
| `-listen` | `127.0.0.1:7681` | address to listen on (HTTP) |
| `-lan` | off | also listen on this address over HTTPS for the local network, e.g. `:7682` |
| `-session` | `main` | tmux session to attach to |
| `-token` | `~/.config/palmterm/token` | login token (also `PALMTERM_TOKEN`); a random one is created on first run |
| `-config` | `~/.config/palmterm/config.toml` | configuration file |
| `-upload-dir` | `~/.cache/palmterm/uploads` | where uploaded images are saved |
| `-allow-origin` | | extra WebSocket origins (for the Vite dev server) |

## Configuration

`~/.config/palmterm/config.toml` sets the UI language and the key bar. Reload the page to apply
changes; no restart is needed. [config.example.toml](config.example.toml) contains the default key bar,
so it is a good starting point.

```toml
language = "en"        # "en" (default) or "ja"

[[keys]]
key = "esc"

[[keys]]
mod = "ctrl"           # sticky modifier

[[keys]]
key = "shift+tab"      # a key combination
label = "S-Tab"

[[keys]]
text = ":wq\r"         # sent as is
label = ":wq"
```

Each `[[keys]]` entry has exactly one of:

| | |
|---|---|
| `key` | a special key or a single character, optionally with modifiers joined by `+`. Special keys: `esc` `tab` `enter` `backspace` `delete` `insert` `up` `down` `left` `right` `home` `end` `pageup` `pagedown` `f1`–`f12` `space`. Modifiers: `ctrl`, `alt` (or `meta`), `shift`. Examples: `ctrl+c`, `alt+left`, `ctrl+space`, `ctrl++` |
| `mod` | `ctrl`, `alt` or `shift`: a sticky modifier button |
| `text` | text sent as is (TOML escapes such as `\r` and `\u001b` work) |

Optional fields: `label`, `icon` (`left` `right` `up` `down` `backspace` `enter` `close` `plus`
`splitH` `splitV` `maximize` `cycle` `panes` `textSmaller` `textLarger`) and `repeat`.
Keys appear left to right in the order written, and any `[[keys]]` entry replaces the whole default set.
Mistakes (unknown fields, keys or icons) are reported when the page opens; everything else still works.

## Security

Whoever logs in to palmterm gets **a shell on your machine**. Keep it private.

- There is a single token, stored in `~/.config/palmterm/token` (mode 0600, fixed on startup if looser).
  It survives restarts.
  The login URL containing it is printed at startup, so it also ends up in logs such as the journal.
- `/auth?token=…` sets an HttpOnly, SameSite=Lax cookie valid for a year (Secure over HTTPS). The cookie
  holds a value derived from the token (HMAC-SHA256), not the token itself. Every page, API call and the
  WebSocket require it, and the WebSocket also checks the Origin.
- Browsers send cookies to every port of the same host name, so other services you open on the same
  host (for example a dev server on another port of `127.0.0.1`) receive the cookie too. A leaked cookie
  can be reused — from any device — until the token changes; it only does not reveal the token itself.
- Comparisons are constant-time. There is no logout or per-device revocation: to log every device out,
  delete the token file (or pass a new `-token`) and restart.
- By default palmterm only listens on 127.0.0.1. `-lan :7682` listens only on the PC's private
  (RFC 1918), link-local and Tailscale (100.64.0.0/10) IPv4 addresses, not on every interface; give an
  address to listen elsewhere. Do not expose palmterm to the internet.
- Uploaded files are served with `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox`.

## Development

```sh
make dev    # Go server on 7681 and Vite on 5173; open http://localhost:5173/auth?token=…
make test   # Go tests, web unit tests (vitest), end-to-end tests in Chromium (playwright-core)
```

- Tests that use tmux run their own tmux server (separate `TMUX_TMPDIR`, `/bin/sh`), so your own
  sessions are not touched. The end-to-end tests use `/usr/bin/chromium` (override with `CHROMIUM=…`).
- `cd web && npm run screenshot` regenerates `docs/screenshot.png` in the same isolated way.
- `tools/fit-nerd-symbols.py` rebuilds the icon fonts in `web/src/fonts/` from Symbols Nerd Font Mono.

## License

[MIT](LICENSE)

Third-party components: [wterm](https://github.com/vercel-labs/wterm) (Apache-2.0),
[JetBrains Mono](https://github.com/JetBrains/JetBrainsMono) (OFL-1.1),
[Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) symbols (MIT; see
`web/src/fonts/SymbolsNerdFont-LICENSE`), [coder/websocket](https://github.com/coder/websocket) (ISC),
[creack/pty](https://github.com/creack/pty) (MIT) and [BurntSushi/toml](https://github.com/BurntSushi/toml) (MIT).
