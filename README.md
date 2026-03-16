# nomarchy

Personal Arch Linux installer and configuration system. Uses a vanilla Arch ISO with a data partition on USB — no custom ISO build, no archinstall, no configuration management framework. Just bash scripts, a `config.env` file, and direct `pacstrap`.

## What it installs

- **Bootloader:** systemd-boot
- **Encryption:** LUKS2 (argon2id) on the root partition
- **Filesystem:** btrfs with subvolumes (`@`, `@home`, `@log`, `@pkg`, `@snapshots`)
- **Display:** Hyprland + UWSM, greetd autologin, Waybar, Walker launcher
- **Terminals:** Ghostty, Alacritty, Kitty
- **Shell:** bash with a modular rc system, starship prompt
- **Audio:** PipeWire + WirePlumber
- **Snapshots:** Snapper on `/` and `/home`
- **Security:** UFW, fail2ban, LUKS at rest

## Structure

```
nomarchy/
├── usb-make          # USB creation script (run once on host)
├── config.env        # Non-secret install config (edit before usb-make)
├── install.sh        # Phase 1: partition, LUKS, pacstrap, bootloader, users
├── setup.sh          # Phase 2: packages, configs, GPU drivers, services
├── packages/
│   ├── base.txt      # Minimal packages added during pacstrap (Hyprland, greetd, PipeWire)
│   ├── desktop.txt   # Full desktop package set installed in Phase 2
│   └── aur.txt       # AUR packages installed in Phase 2 if internet is available
├── config/           # Deployed to ~/.config/
├── default/          # bashrc, bash partials, Plymouth theme, and other system defaults
├── bin/              # 165 nomarchy-* scripts deployed to ~/.local/bin/
└── applications/     # .desktop files deployed to ~/.local/share/applications/
```

## Usage

### 1. Configure

Edit `config.env` with your target disk, username, hostname, timezone, and locale. Passwords are **not** stored here — they are prompted interactively during install.

```bash
INSTALL_DISK=          # leave blank to be prompted, or set e.g. /dev/sda
INSTALL_USER="samurai"
INSTALL_HOSTNAME="archlinux"
INSTALL_TIMEZONE="America/Boise"
INSTALL_LOCALE="en_US.UTF-8"
INSTALL_KB_LAYOUT="us"
INSTALL_BOOT_SIZE="1G"
```

### 2. Create USB

Requires a USB drive of at least ~1.5 GB larger than the Arch ISO (~1.2 GB).

```bash
sudo ./usb-make
```

The script will:
- Locate or download the latest Arch ISO (with SHA256 verification)
- Show all block devices and ask you to select one
- Require **two confirmation steps** before erasing anything
- `dd` the ISO to the device
- Append a Linux data partition in the remaining space (using `sfdisk`, not `parted`)
- Format it `ext4` labeled `NOMARCHY`
- `rsync` this entire repo onto it

> **Why `sfdisk` and not `parted`?** `parted -s` rewrites the GPT backup sector when it detects it isn't at the end of the disk (always true after `dd`'ing an isohybrid ISO), overwriting MBR bytes 0–445 — the boot code — and breaking BIOS/CSM boot. `sfdisk --append` preserves those bytes.

### 3. Boot and install

Boot the USB. At the live Arch prompt:

```bash
# Connect WiFi if needed
iwctl station wlan0 connect <SSID>

# Mount the data partition and run the installer
mkdir /mnt/setup
mount LABEL=NOMARCHY /mnt/setup
bash /mnt/setup/nomarchy/install.sh
```

You will be prompted for:
- LUKS passphrase (entered twice)
- User password (entered twice)

`install.sh` will partition the disk, set up LUKS2, create btrfs subvolumes, run `pacstrap`, configure systemd-boot, set up greetd autologin into Hyprland, and copy this repo to `~/.local/share/nomarchy/`.

Reboot when done.

### 4. First-boot setup

On first login, a systemd user service automatically opens a terminal and runs `setup.sh`. This installs the full desktop package set, deploys all configs and bin scripts, detects GPU and installs drivers, configures Snapper, enables services, and sets up Docker and UFW.

If you skip or need to re-run it manually:

```bash
bash ~/.local/share/nomarchy/setup.sh
```

### 5. AUR packages (requires internet)

AUR packages are installed during `setup.sh` if internet is available. If not, a `nomarchy-post-install` script is placed in `~/.local/bin/` to run later:

```bash
nomarchy-post-install
```

## Disk layout

| Partition | Type | Size | Purpose |
|-----------|------|------|---------|
| p1 | EFI | 1G (configurable) | systemd-boot |
| p2 | LUKS2 | remainder | btrfs root |

btrfs subvolumes inside LUKS:

| Subvolume | Mount |
|-----------|-------|
| `@` | `/` |
| `@home` | `/home` |
| `@log` | `/var/log` |
| `@pkg` | `/var/cache/pacman/pkg` |
| `@snapshots` | `/.snapshots` |

## bin scripts

165 `nomarchy-*` scripts cover system management, theming, hardware, and desktop operations. A few highlights:

| Category | Scripts |
|----------|---------|
| **Themes** | `nomarchy-theme-set`, `nomarchy-theme-install`, `nomarchy-theme-list`, ... |
| **Updates** | `nomarchy-update`, `nomarchy-update-aur-pkgs`, `nomarchy-update-firmware` |
| **Hardware** | `nomarchy-hw-framework16`, `nomarchy-hw-asus-rog`, `nomarchy-hw-surface` |
| **Hyprland** | `nomarchy-hyprland-monitor-scaling-cycle`, `nomarchy-hyprland-workspace-layout-toggle`, ... |
| **Installs** | `nomarchy-install-steam`, `nomarchy-install-docker-dbs`, `nomarchy-install-vscode`, ... |
| **Toggles** | `nomarchy-toggle-nightlight`, `nomarchy-toggle-idle`, `nomarchy-toggle-waybar`, ... |
| **Drives** | `nomarchy-drive-select`, `nomarchy-drive-info`, `nomarchy-drive-set-password` |

## Customization

### User config hooks

Drop scripts into `~/.config/nomarchy/hooks/` to run on system events (theme changes, font changes, updates, low battery). Samples are in `config/nomarchy/hooks/`.

### Menu extensions

Override or extend `nomarchy-menu` by placing scripts in `~/.config/nomarchy/extensions/`. See `config/nomarchy/extensions/menu.sh` for the pattern.

### bashrc

`~/.bashrc` sources `~/.local/share/nomarchy/default/bash/rc`, which loads modular partials (envs, aliases, functions, shell options). Add your own overrides directly in `~/.bashrc` below that line — the comment in the deployed file explains this.

## GPU support

`setup.sh` detects the GPU via `lspci` and installs the appropriate drivers automatically:

- **NVIDIA (Turing+):** `nvidia-open-dkms` + kernel headers + Hyprland env vars
- **NVIDIA (older):** `nvidia-dkms`
- **AMD:** `mesa`, `vulkan-radeon`, `libva-mesa-driver`
- **Intel:** `intel-media-driver`, `vulkan-intel`

## Requirements

- A Linux machine to run `usb-make` on (any distro)
- A USB drive (≥ 4 GB recommended)
- Internet access during install (for `pacstrap`) — AUR packages can be deferred
