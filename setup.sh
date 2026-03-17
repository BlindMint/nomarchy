#!/bin/bash
set -euo pipefail

NOMARCHY_DIR="$(cd "$(dirname "$0")" && pwd)"
USERNAME="${USER}"
USER_HOME="$HOME"
SENTINEL="$HOME/.local/state/nomarchy/setup-done"

# Ensure user binaries are findable — the systemd launcher uses a minimal PATH
# that does not include ~/.local/bin, so nomarchy-* commands would not be found.
export PATH="$USER_HOME/.local/bin:$PATH"

log() { echo "[*] $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

has_internet() { ping -c1 -W3 archlinux.org &>/dev/null; }

disable_mkinitcpio_hooks() {
    log "Temporarily disabling mkinitcpio hooks..."
    [[ -f /usr/share/libalpm/hooks/90-mkinitcpio-install.hook ]] \
        && sudo mv /usr/share/libalpm/hooks/90-mkinitcpio-install.hook \
                   /usr/share/libalpm/hooks/90-mkinitcpio-install.hook.disabled || true
    [[ -f /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook ]] \
        && sudo mv /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook \
                   /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook.disabled || true
}

enable_mkinitcpio_hooks() {
    [[ -f /usr/share/libalpm/hooks/90-mkinitcpio-install.hook.disabled ]] \
        && sudo mv /usr/share/libalpm/hooks/90-mkinitcpio-install.hook.disabled \
                   /usr/share/libalpm/hooks/90-mkinitcpio-install.hook || true
    [[ -f /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook.disabled ]] \
        && sudo mv /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook.disabled \
                   /usr/share/libalpm/hooks/60-mkinitcpio-remove.hook || true
}

install_packages() {
    local pkgs
    pkgs=$(grep -v '^\s*#' "$NOMARCHY_DIR/packages/desktop.txt" | grep -v '^\s*$' | tr '\n' ' ')
    sudo pacman -S --noconfirm --needed $pkgs
}

install_paru() {
    if paru --version &>/dev/null; then return; fi
    log "Installing paru..."
    rm -rf /tmp/paru-build
    git clone https://aur.archlinux.org/paru.git /tmp/paru-build
    cd /tmp/paru-build
    makepkg -si --noconfirm
    cd /
    rm -rf /tmp/paru-build
}

install_aur_packages() {
    local pkgs
    pkgs=$(grep -v '^\s*#' "$NOMARCHY_DIR/packages/aur.txt" | grep -v '^\s*$' | tr '\n' ' ')
    paru -S --noconfirm --needed $pkgs </dev/null || log "Some AUR packages failed"
}

install_aur() {
    if ! has_internet; then
        log "No internet — skipping AUR packages. Run 'nomarchy-post-install' when connected."
        mkdir -p "$USER_HOME/.local/bin"
        cat > "$USER_HOME/.local/bin/nomarchy-post-install" <<SCRIPT
#!/bin/bash
exec bash "$NOMARCHY_DIR/setup.sh" --aur-only
SCRIPT
        chmod +x "$USER_HOME/.local/bin/nomarchy-post-install"
        return
    fi

    install_paru
    install_aur_packages
}

deploy_configs() {
    # ~/.config
    rsync -a "$NOMARCHY_DIR/config/" "$USER_HOME/.config/"

    # ~/.local/bin
    mkdir -p "$USER_HOME/.local/bin"
    rsync -a "$NOMARCHY_DIR/bin/" "$USER_HOME/.local/bin/"
    chmod +x "$USER_HOME/.local/bin"/*

    # .desktop files
    if [[ -d "$NOMARCHY_DIR/applications" ]]; then
        mkdir -p "$USER_HOME/.local/share/applications"
        cp "$NOMARCHY_DIR/applications/"*.desktop "$USER_HOME/.local/share/applications/" 2>/dev/null || true
    fi

    # bashrc
    cp "$NOMARCHY_DIR/default/bashrc" "$USER_HOME/.bashrc"

    # Screensaver character
    mkdir -p "$USER_HOME/.config/nomarchy/branding"
    echo "ｦ" > "$USER_HOME/.config/nomarchy/branding/screensaver_matrix.txt"
}

setup_theme() {
    # Brave browser policy directory — required for nomarchy-theme-set-browser to write
    # the managed color policy. Must exist and be world-writable before theme runs.
    sudo mkdir -p /etc/brave/policies/managed
    sudo chmod a+rw /etc/brave/policies/managed

    local theme="${INSTALL_THEME:-catppuccin}"
    if [[ -d "$NOMARCHY_DIR/themes/$theme" ]]; then
        log "Setting default theme: $theme"
        NOMARCHY_PATH="$NOMARCHY_DIR" "$USER_HOME/.local/bin/nomarchy-theme-set" "$theme" \
            || log "Theme setup failed — run nomarchy-theme-set $theme manually"
    else
        log "Theme '$theme' not found in themes/ — skipping. Run nomarchy-theme-set <theme> manually."
    fi
}

setup_gpu_drivers() {
    local gpu_info
    gpu_info=$(lspci | grep -iE 'vga|3d|display' || true)

    if echo "$gpu_info" | grep -qi 'nvidia'; then
        # Install headers for every kernel that is installed
        local headers=()
        pacman -Q linux     &>/dev/null && headers+=(linux-headers)
        pacman -Q linux-zen &>/dev/null && headers+=(linux-zen-headers)

        local nvidia_driver="nvidia-dkms"
        echo "$gpu_info" | grep -qE "GTX 16[0-9]{2}|RTX [2-9][0-9]{3}|RTX PRO|Quadro RTX|RTX A[0-9]" \
            && nvidia_driver="nvidia-open-dkms"

        sudo pacman -S --noconfirm --needed \
            "${headers[@]}" "$nvidia_driver" nvidia-utils nvidia-settings \
            egl-wayland libva-nvidia-driver

        sudo mkdir -p /etc/modprobe.d
        echo "options nvidia_drm modeset=1 fbdev=1" | sudo tee /etc/modprobe.d/nvidia.conf >/dev/null

        sudo mkdir -p /etc/mkinitcpio.conf.d
        echo "MODULES+=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)" \
            | sudo tee /etc/mkinitcpio.conf.d/nvidia.conf >/dev/null

        # Write Hyprland env vars (guard against re-run duplicates)
        if ! grep -qF 'LIBVA_DRIVER_NAME,nvidia' "$USER_HOME/.config/hypr/env.conf" 2>/dev/null; then
            cat >> "$USER_HOME/.config/hypr/env.conf" <<'EOF'
env = LIBVA_DRIVER_NAME,nvidia
env = __GLX_VENDOR_LIBRARY_NAME,nvidia
env = NVD_BACKEND,direct
EOF
        fi

    elif echo "$gpu_info" | grep -qi 'amd\|radeon'; then
        sudo pacman -S --noconfirm --needed \
            mesa lib32-mesa vulkan-radeon lib32-vulkan-radeon libva-mesa-driver
    elif echo "$gpu_info" | grep -qi 'intel'; then
        sudo pacman -S --noconfirm --needed \
            intel-media-driver vulkan-intel lib32-vulkan-intel
    fi
}

setup_snapper() {
    if ! command -v snapper &>/dev/null; then return; fi

    sudo snapper -c root create-config / 2>/dev/null || true
    sudo snapper -c home create-config /home 2>/dev/null || true
    sudo btrfs quota enable / 2>/dev/null || true

    for cfg in /etc/snapper/configs/root /etc/snapper/configs/home; do
        [[ -f "$cfg" ]] || continue
        sudo sed -i \
            -e 's/^TIMELINE_CREATE="yes"/TIMELINE_CREATE="no"/' \
            -e 's/^NUMBER_LIMIT="50"/NUMBER_LIMIT="5"/' \
            -e 's/^NUMBER_LIMIT_IMPORTANT="10"/NUMBER_LIMIT_IMPORTANT="5"/' \
            -e 's/^SPACE_LIMIT="0.5"/SPACE_LIMIT="0.3"/' \
            -e 's/^FREE_LIMIT="0.2"/FREE_LIMIT="0.3"/' \
            "$cfg"
    done
}

setup_hardware() {
    # Bluetooth (optional — skip if not installed)
    systemctl list-unit-files bluetooth.service &>/dev/null \
        && sudo systemctl enable bluetooth || true

    # NetworkManager
    sudo systemctl enable NetworkManager

    # Inotify
    echo "fs.inotify.max_user_watches=524288" | sudo tee /etc/sysctl.d/40-max-user-watches.conf >/dev/null

    # Sysctl
    sudo sysctl --system
}

setup_mimetypes() {
    # xdg-mime defaults
    xdg-mime default org.gnome.Nautilus.desktop inode/directory
    xdg-mime default imv.desktop image/png image/jpeg image/gif image/webp image/svg+xml
    xdg-mime default mpv.desktop video/mp4 video/webm audio/mp3 audio/flac
    xdg-mime default typora.desktop text/markdown application/pdf
}

setup_gnome_keyring() {
    # Create a passwordless Default_keyring so gnome-keyring auto-unlocks without
    # a login password. This is compatible with greetd auto-login: no password is
    # entered at login, so PAM can't decrypt an encrypted keyring — a passwordless
    # one sidesteps that entirely. Security at rest is provided by LUKS.
    local keyring_dir="$USER_HOME/.local/share/keyrings"
    local keyring_file="$keyring_dir/Default_keyring.keyring"
    local default_file="$keyring_dir/default"

    if [[ ! -f "$keyring_file" ]]; then
        mkdir -p "$keyring_dir"
        cat > "$keyring_file" <<EOF
[keyring]
display-name=Default keyring
ctime=$(date +%s)
mtime=0
lock-on-idle=false
lock-after=false
EOF
        cat > "$default_file" <<EOF
Default_keyring
EOF
        chmod 700 "$keyring_dir"
        chmod 600 "$keyring_file"
        chmod 644 "$default_file"
    fi

    # Start gnome-keyring daemon on session open via greetd's PAM config.
    # Only the session line is added — omitting auth/password phases prevents
    # PAM from creating a conflicting encrypted login.keyring.
    if ! grep -q 'pam_gnome_keyring\.so' /etc/pam.d/greetd; then
        echo '-session   optional    pam_gnome_keyring.so auto_start' \
            | sudo tee -a /etc/pam.d/greetd > /dev/null
    fi
}

setup_firewall() {
    command -v ufw &>/dev/null && sudo ufw enable || true
}

setup_docker() {
    command -v docker &>/dev/null || return 0
    sudo systemctl enable docker
    sudo usermod -aG docker "$USERNAME"

    # DNS
    sudo mkdir -p /etc/docker
    sudo tee /etc/docker/daemon.json >/dev/null <<EOF
{
    "dns": ["1.1.1.1", "8.8.8.8"]
}
EOF
}

finalize_bootloader() {
    sudo mkinitcpio -P
}

setup_limine_snapper() {
    if ! command -v limine &>/dev/null; then
        log "limine not found — skipping limine-snapper-sync setup"
        return
    fi

    # limine-mkinitcpio-hook: installs pacman hooks for auto-entry-generation on kernel
    # updates, and provides the btrfs-overlayfs mkinitcpio hook for snapshot booting.
    # limine-snapper-sync: auto-generates Limine entries from snapper snapshots.
    paru -S --noconfirm --needed aur/limine-mkinitcpio-hook aur/limine-snapper-sync

    # Add btrfs-overlayfs to HOOKS (provided by limine-mkinitcpio-hook).
    # This must happen before limine-update, which rebuilds the initramfs.
    # Without it, booting into a snapshot would fail — the hook sets up an overlayfs
    # so the read-only snapshot subvolume can accept writes via a tmpfs upper layer.
    sudo tee /etc/mkinitcpio.conf.d/nomarchy-limine.conf >/dev/null <<'EOF'
HOOKS=(base udev plymouth keyboard autodetect microcode modconf kms keymap consolefont block encrypt filesystems fsck btrfs-overlayfs)
EOF

    # Write entry generator config (cmdline extracted from current limine.conf)
    local limine_conf="/boot/limine.conf"
    local cmdline
    cmdline=$(grep -m1 "^[[:space:]]*cmdline:" "$limine_conf" | sed 's/^[[:space:]]*cmdline:[[:space:]]*//')

    sudo cp "$NOMARCHY_DIR/default/limine/default.conf" /etc/default/limine
    sudo sed -i "s|@@CMDLINE@@|$cmdline|g" /etc/default/limine

    # Overwrite boot entries with the visual config; limine-update will regenerate entries
    sudo cp "$NOMARCHY_DIR/default/limine/limine.conf" "$limine_conf"

    sudo systemctl enable limine-snapper-sync.service

    # Re-enable standard mkinitcpio hooks before the final rebuild.
    # After limine-mkinitcpio-hook is installed, its hook in /etc/pacman.d/hooks/
    # takes precedence for future kernel updates.
    enable_mkinitcpio_hooks

    # Rebuilds initramfs (with btrfs-overlayfs now present) and regenerates Limine entries
    sudo limine-update
}

mark_complete() {
    mkdir -p "$(dirname "$SENTINEL")"
    touch "$SENTINEL"
    # Remove the temporary NOPASSWD line appended during install, leaving only
    # the standard wheel rule.
    echo "%wheel ALL=(ALL:ALL) ALL" | sudo tee /etc/sudoers.d/wheel > /dev/null
    sudo chmod 440 /etc/sudoers.d/wheel
}

main() {
    if [[ "${1:-}" == "--aur-only" ]]; then
        install_paru
        install_aur_packages
        return
    fi

    log "Starting nomarchy setup..."

    # Warm up sudo credentials early. The install appended a NOPASSWD rule to
    # /etc/sudoers.d/wheel, so this should be passwordless. If it prompts, that
    # rule is missing — the user can enter their password once and setup continues.
    if ! sudo -n true 2>/dev/null; then
        log "Passwordless sudo not active (expected NOPASSWD line in /etc/sudoers.d/wheel)."
        log "Files in sudoers.d: $(ls /etc/sudoers.d/ 2>/dev/null | tr '\n' ' ')"
        sudo -v
    fi

    disable_mkinitcpio_hooks
    deploy_configs
    install_packages
    install_aur
    setup_theme
    setup_gpu_drivers
    setup_snapper
    setup_limine_snapper
    setup_hardware
    setup_gnome_keyring
    setup_mimetypes
    setup_firewall
    setup_docker
    finalize_bootloader
    mark_complete

    log "Setup complete!"
}

main "$@"