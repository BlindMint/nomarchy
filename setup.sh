#!/bin/bash
set -euo pipefail

PAVE_DIR="$(cd "$(dirname "$0")" && pwd)"
USERNAME="${USER}"
USER_HOME="$HOME"
SENTINEL="$HOME/.local/state/nomarchy/setup-done"

log() { echo "[*] $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

has_internet() { ping -c1 -W3 archlinux.org &>/dev/null; }

install_packages() {
    local pkgs
    pkgs=$(grep -v '^\s*#' "$PAVE_DIR/packages/desktop.txt" | grep -v '^\s*$' | tr '\n' ' ')
    sudo pacman -S --noconfirm --needed $pkgs
}

install_paru() {
    if command -v paru &>/dev/null; then return; fi
    log "Installing paru..."
    mkdir -p /tmp/paru-build
    cd /tmp/paru-build
    git clone https://aur.archlinux.org/paru.git
    cd paru
    makepkg -si --noconfirm
    cd /
    rm -rf /tmp/paru-build
}

install_aur_packages() {
    local pkgs
    pkgs=$(grep -v '^\s*#' "$PAVE_DIR/packages/aur.txt" | grep -v '^\s*$' | sed 's|^|aur/|' | tr '\n' ' ')
    paru -S --noconfirm --needed $pkgs </dev/null || log "Some AUR packages failed"
}

install_aur() {
    if ! has_internet; then
        log "No internet — skipping AUR packages. Run 'nomarchy-post-install' when connected."
        cat > "$USER_HOME/.local/bin/nomarchy-post-install" <<SCRIPT
#!/bin/bash
exec bash "$PAVE_DIR/setup.sh" --aur-only
SCRIPT
        chmod +x "$USER_HOME/.local/bin/nomarchy-post-install"
        return
    fi

    install_paru
    install_aur_packages
}

deploy_configs() {
    # ~/.config
    rsync -a "$PAVE_DIR/config/" "$USER_HOME/.config/"

    # ~/.local/bin
    mkdir -p "$USER_HOME/.local/bin"
    rsync -a "$PAVE_DIR/bin/" "$USER_HOME/.local/bin/"
    chmod +x "$USER_HOME/.local/bin"/*

    # .desktop files
    if [[ -d "$PAVE_DIR/applications" ]]; then
        mkdir -p "$USER_HOME/.local/share/applications"
        cp "$PAVE_DIR/applications/"*.desktop "$USER_HOME/.local/share/applications/" 2>/dev/null || true
    fi

    # bashrc
    cp "$PAVE_DIR/default/bashrc" "$USER_HOME/.bashrc"
    mkdir -p "$USER_HOME/.local/share/nomarchy/default/bash"
    cp -r "$PAVE_DIR/default/bash/"* "$USER_HOME/.local/share/nomarchy/default/bash/"

    # Screensaver character
    mkdir -p "$USER_HOME/.config/nomarchy/branding"
    echo "ｦ" > "$USER_HOME/.config/nomarchy/branding/screensaver_matrix.txt"
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
            "${headers[@]}" "$nvidia_driver" nvidia-utils lib32-nvidia-utils \
            egl-wayland libva-nvidia-driver

        sudo mkdir -p /etc/modprobe.d
        echo "options nvidia_drm modeset=1 fbdev=1" | sudo tee /etc/modprobe.d/nvidia.conf >/dev/null

        sudo mkdir -p /etc/mkinitcpio.conf.d
        echo "MODULES+=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)" \
            | sudo tee /etc/mkinitcpio.conf.d/nvidia.conf >/dev/null

        # Write Hyprland env vars
        cat >> "$USER_HOME/.config/hypr/env.conf" <<'EOF'
env = LIBVA_DRIVER_NAME,nvidia
env = __GLX_VENDOR_LIBRARY_NAME,nvidia
env = NVD_BACKEND,direct
EOF

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
    # Bluetooth
    sudo systemctl enable bluetooth

    # NetworkManager
    sudo systemctl enable NetworkManager

    # Inotify
    echo "fs.inotify.max_user_watches=524288" | sudo tee /etc/sysctl.d/40-max-user-watches.conf >/dev/null

    # Sysctl
    sudo sysctl --system
}

setup_mimetypes() {
    # xdg-mime defaults
    xdg-mime default imv.desktop image/png image/jpeg image/gif image/webp image/svg+xml
    xdg-mime default mpv.desktop video/mp4 video/webm audio/mp3 audio/flac
    xdg-mime default typora.desktop text/markdown application/pdf
}

setup_firewall() {
    sudo ufw enable
}

setup_docker() {
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
    sudo bootctl update
}

mark_complete() {
    mkdir -p "$(dirname "$SENTINEL")"
    touch "$SENTINEL"
    sudo rm -f /etc/sudoers.d/nomarchy-setup
}

main() {
    if [[ "${1:-}" == "--aur-only" ]]; then
        install_paru
        install_aur_packages
        return
    fi

    log "Starting nomarchy setup..."

    install_packages
    install_aur
    setup_gpu_drivers
    deploy_configs
    setup_snapper
    setup_hardware
    setup_mimetypes
    setup_firewall
    setup_docker
    finalize_bootloader
    mark_complete

    log "Setup complete!"
}

main "$@"