# GoPlay

**GoPlay** is a lightweight AirPlay receiver for Windows, written in Go. It lets you mirror your iPhone/iPad screen to your PC and stream AirPlay audio with low latency, using [mpv](https://mpv.io/) for hardware‑accelerated rendering.

> No bloated runtime, no Bonjour service to install — a single `.exe` plus `mpv.exe`.

---

## ✨ Features

- 🖥️ **Screen Mirroring** — mirror an iPhone/iPad screen to your PC, with audio, at low latency (hardware decode via `--hwdec=auto`).
- 🔊 **AirPlay Audio** — stream audio from your device to the PC (AAC‑ELD).
- 🧪 **AirPlay Video / HLS** — experimental playback of non‑protected AirPlay video streams.
- ⚙️ **Simple config** — name and port live in a single `config.json`.
- 📦 **Portable** — no installer; just unzip and run.

> ⚠️ **DRM‑protected content** (Netflix, Apple TV+, etc.) will **not** play — those streams use FairPlay video DRM, which is intentionally not implemented.

---

## 🚀 Quick start (release build)

1. Download the latest archive from the [**Releases**](../../releases) page.
2. Unzip it anywhere. You should have these files in one folder:
   ```
   goplay.exe
   mpv.exe
   config.json
   ```
3. Run `goplay.exe`.
4. On your iPhone/iPad open **Control Center → Screen Mirroring** (or **AirPlay** from a media app) and pick **GoPlay**.

Make sure your PC and your iPhone are on the **same Wi‑Fi network**.

> If you downloaded a build **without** `mpv.exe`, grab a Windows build of mpv from <https://mpv.io/installation/> and drop `mpv.exe` next to `goplay.exe`.

---

## ⚙️ Configuration

GoPlay reads `config.json` from the **same folder as `goplay.exe`** on startup. If the file is missing, a default one is created automatically.

```json
{
  "airplayName": "GoPlay",
  "port": 7000
}
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `airplayName` | string | `"GoPlay"` | Name shown in the AirPlay / Screen Mirroring list on your device. |
| `port` | number | `7000` | TCP port the receiver listens on. |

> Changes are applied **on startup** — restart `goplay.exe` after editing the file.
> After a name change, your iPhone may keep showing the old name for a minute (mDNS cache); toggle Wi‑Fi to refresh it instantly.

---

## 🛠️ Build from source

**Requirements:** [Go 1.25+](https://go.dev/dl/) and a Windows build of `mpv.exe` for running.

```bash
git clone https://github.com/ivqxzz/goplay.git
cd goplay
go mod tidy
go build -ldflags "-H windowsgui" -o goplay.exe .
```

Or just run the included `build.bat`.

The `-H windowsgui` flag builds a windowless app. For debugging (so you can see logs and stop it with `Ctrl+C`), build a normal console binary instead:

```bash
go build -o goplay.exe .
```

### Cross‑compiling

The project is pure Go (no cgo), so it cross‑compiles cleanly:

```bash
GOOS=linux  GOARCH=amd64 CGO_ENABLED=0 go build -o goplay .   # Linux
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o goplay .   # macOS (Apple Silicon)
```

> Non‑Windows builds compile, but are **not yet fully tested**. The auto‑kill of child `mpv` processes is currently Windows‑only, and macOS may need extra handling for its system Bonjour responder on port 5353.

---

## 📋 Requirements

- Windows 10 / 11 (primary target)
- `mpv.exe` available next to the binary or on `PATH`
- A GPU that supports hardware video decode (recommended for smooth mirroring)
- PC and device on the same local network

---

## 🧯 Troubleshooting

| Problem | Fix |
|---------|-----|
| GoPlay doesn't appear on the iPhone | Check both devices are on the same Wi‑Fi; check your firewall isn't blocking `goplay.exe`. |
| Name change doesn't show up | Restart `goplay.exe`; toggle Wi‑Fi on the iPhone (mDNS cache). |
| Nothing plays / black screen | Make sure `mpv.exe` is present next to `goplay.exe`. See `goplay.log`. |
| Protected video won't play | Expected — DRM (FairPlay video) is not supported. |

A log file `goplay.log` is written next to the executable and is the first place to look when something goes wrong.

---

## 🙏 Credits

GoPlay's AirPlay/FairPlay handshake and key‑derivation code is derived from the excellent
[**RPiPlay**](https://github.com/FD-/RPiPlay) project and its predecessors
(`playfair`, `omg_hax`, FairPlay reverse‑engineering work). Huge thanks to those authors.

Video and audio rendering is handled by [**mpv**](https://mpv.io/).

---

## 📄 License

Because GoPlay includes code derived from RPiPlay, it is distributed under the
**GNU General Public License v3.0**. See [LICENSE](LICENSE) for details.
