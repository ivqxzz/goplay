# GoPlay

GoPlay is a lightweight AirPlay receiver for Windows, written in Go. It allows you to mirror an iPhone or iPad screen to a PC and stream AirPlay audio using [mpv](https://mpv.io/) for playback.

GoPlay is designed to be simple, portable, and easy to run.

---

## Features

- Screen mirroring from iPhone and iPad to Windows
- AirPlay audio playback
- Experimental support for non-protected AirPlay video and HLS streams
- Simple configuration through `config.json`
- Portable Windows release, no installer required
- Playback through `mpv`

> DRM-protected content, such as Netflix, Apple TV+, and similar services, is not supported.

---

## Quick start for Windows

1. Download the latest archive from the [Releases](https://github.com/ivqxzz/goplay/releases) page.
2. Extract the archive to any folder.

   The release archive contains:

   ```text
   goplay.exe
   config.json
   README.md
   LICENSE
   ```

3. Download a Windows build of `mpv` from [mpv.io](https://mpv.io/installation/).
4. Put `mpv.exe` in the same folder as `goplay.exe`.

   The folder should look like this:

   ```text
   goplay.exe
   mpv.exe
   config.json
   README.md
   LICENSE
   ```

5. Run `goplay.exe`.
6. On your iPhone or iPad, open **Control Center > Screen Mirroring** and select **GoPlay**.

Make sure the PC and the iPhone or iPad are connected to the same local network.

> The release archive does not include `mpv.exe`. It must be downloaded separately.

---

## Linux and macOS status

GoPlay is primarily developed and tested on Windows.

Linux and macOS builds may compile, but they are currently experimental and not fully tested. Some platform-specific behavior may require additional work, especially service discovery, firewall configuration, network interfaces, and child process handling.

On Linux and macOS, `mpv` is normally installed as a system command and must be available in `PATH`.

### Linux

Install `mpv` with your package manager. For example, on Debian or Ubuntu:

```bash
sudo apt install mpv
```

Check that `mpv` is available:

```bash
mpv --version
```

### macOS

Install `mpv` with Homebrew:

```bash
brew install mpv
```

Check that `mpv` is available:

```bash
mpv --version
```

---

## Configuration

GoPlay reads `config.json` from the same folder as the executable on startup. If the file is missing, a default configuration file is created automatically.

```json
{
  "airplayName": "GoPlay",
  "port": 7000
}
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `airplayName` | string | `"GoPlay"` | Name shown in the AirPlay and Screen Mirroring device list. |
| `port` | number | `7000` | TCP port used by the receiver. |

Changes are applied on startup. Restart GoPlay after editing `config.json`.

After changing the AirPlay name, iOS or iPadOS may continue to show the old name for a short time because of mDNS cache. Toggling Wi-Fi on the device can refresh the list faster.

---

## Build from source

### Requirements

- [Go](https://go.dev/dl/)
- `mpv`
  - Windows: `mpv.exe` next to `goplay.exe` or available in `PATH`
  - Linux and macOS: `mpv` available as a system command in `PATH`

### Windows

```bash
git clone https://github.com/ivqxzz/goplay.git
cd goplay
go mod tidy
go build -trimpath -ldflags "-s -w" -o goplay.exe .
```

You can also run the included build script:

```bat
build.bat
```

This builds a console application. To stop GoPlay, close the console window or press `Ctrl+C`.

A windowless build is possible, but it is not recommended unless the application has another built-in way to exit:

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o goplay.exe .
```

### Linux and macOS

```bash
git clone https://github.com/ivqxzz/goplay.git
cd goplay
go mod tidy
go build -trimpath -ldflags "-s -w" -o goplay .
```

Example cross-compilation commands:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o goplay .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o goplay .
```

Non-Windows builds are experimental and may require additional testing or fixes.

---

## Requirements

### Windows

- Windows 10 or Windows 11
- `mpv.exe` next to `goplay.exe` or available in `PATH`
- PC and iPhone or iPad on the same local network
- Firewall access for `goplay.exe`
- Hardware video decoding support is recommended for smoother mirroring

### Linux and macOS

- Experimental support only
- `mpv` installed and available in `PATH`
- Computer and iPhone or iPad on the same local network
- Network and firewall configuration that allows local AirPlay discovery and streaming

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| GoPlay does not appear on the iPhone or iPad | Make sure both devices are on the same local network. Check that the firewall is not blocking GoPlay. |
| The device name does not update | Restart GoPlay and toggle Wi-Fi on the iPhone or iPad to refresh mDNS cache. |
| Nothing plays or the screen is black | Make sure `mpv` is installed and available. On Windows, put `mpv.exe` next to `goplay.exe`. Check `goplay.log`. |
| `mpv` is not found | On Windows, put `mpv.exe` next to `goplay.exe` or add it to `PATH`. On Linux and macOS, install `mpv` and check `mpv --version`. |
| Protected video does not play | This is expected. DRM-protected video is not supported. |

A log file named `goplay.log` is written next to the executable and is the first place to check when something goes wrong.

---

## Credits

GoPlay includes code derived from [RPiPlay](https://github.com/FD-/RPiPlay) and related FairPlay reverse-engineering work, including `playfair` and `omg_hax`.

Video and audio playback is handled by [mpv](https://mpv.io/).

---

## License

Because GoPlay includes code derived from RPiPlay, it is distributed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for details.
