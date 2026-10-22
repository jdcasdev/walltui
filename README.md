# walltui

Keyboard-driven TUI for managing wallpapers on Linux (sway / Wayland).

Browse the wallpaper library, preview images in the terminal, apply one with
`swaybg`, and move unwanted ones to the system trash — all from the keyboard.

## Install

```sh
go install github.com/jdcasdev/walltui/cmd/walltui@latest   # -> $GOBIN/walltui
```

Or from a clone:

```sh
make build                                  # -> bin/walltui
sudo install -m755 bin/walltui /usr/local/bin/walltui
```

## Dependencies

| command | needed for |
|---|---|
| `swaybg` | drawing the wallpaper (required) |
| `gio` | moving files to the system trash (`d`) |

Wallpapers are set through a backend layer; `swaybg` is the only backend
today, more will follow. swaybg always draws PNG; the other formats need it
built with `gdk-pixbuf2`, which the distribution packages ship.

## Usage

```sh
walltui              # open the picker
walltui apply        # apply without the TUI: the stored wallpaper, or the first one
walltui apply PATH  # apply a specific wallpaper
walltui --version
```

### Keys

| key | action |
|---|---|
| `↑`/`k`, `↓`/`j` | move |
| `pgup`/`pgdn`, `home`/`end` | jump |
| `space` | mark a wallpaper |
| `enter` | apply the wallpaper under the cursor |
| `s` | search: type to filter, `enter` applies, `esc` cancels |
| `d` | move to trash: the marked wallpapers, or the one under the cursor |
| `esc`, `ctrl+c` | quit |

## Configuration

| variable | default | meaning |
|---|---|---|
| `WALLPAPER_DIR` | `~/Pictures/Wallpapers` | the wallpaper library (scanned recursively) |
| `WALLPAPER_MODE` | `fill` | fit mode: `fill`, `fit`, `stretch`, `center`, `tile` |
| `XDG_STATE_HOME` | `~/.local/state` | where walltui keeps its state |

State lives in `~/.local/state/walltui/`: `current` holds the applied
wallpaper, `apply.lock` serializes concurrent applies, `swaybg.pid` tracks the
background process walltui started.

## Notes

- walltui only retires the `swaybg` it started itself (tracked in
  `swaybg.pid`): it never kills another program's wallpaper.
- To get the wallpaper you applied back on login, let walltui own the boot:
  `swaybg_command -` plus `exec /path/to/walltui apply` in sway's config.
  The first disables sway's own background client, which also makes its
  `output … background` directives inert; the second runs once, at startup
  (a config reload does not re-run it). It applies the stored wallpaper, or
  the first one in the library when there is none yet.
- The wallpaper on screen is only retired once the replacement is confirmed
  running, so a failed apply leaves the current wallpaper alone.
- No desktop notifications: the card shows the operation in flight, and
  the ● marker in the list names the applied wallpaper.
- Trashed wallpapers go to the FreeDesktop trash: they stay recoverable.

## License

MIT. Fork it, rename it, ship it in a distro, change it — keep the copyright
line and that's it.
