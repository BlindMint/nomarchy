#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# If running from under /mnt, copy to /tmp now — setup_btrfs will mount over /mnt
# and shadow everything beneath it, making $SCRIPT_DIR inaccessible later.
if [[ "$SCRIPT_DIR" == /mnt/* ]]; then
    cp -a "$SCRIPT_DIR" /tmp/nomarchy-src
    SCRIPT_DIR=/tmp/nomarchy-src
fi

source "$SCRIPT_DIR/config.env"

# Global variables
LUKS_PASSPHRASE=""
USER_PASSWORD=""
BOOT_PART=""
LUKS_PART=""

log() { echo "[*] $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

prompt_passwords() {
    log "Prompting for passwords..."
    echo "Enter LUKS passphrase (twice for confirmation):"
    read -s -p "LUKS passphrase: " LUKS_PASSPHRASE
    echo
    read -s -p "Confirm LUKS passphrase: " confirm
    echo
    [[ "$LUKS_PASSPHRASE" == "$confirm" ]] || error "LUKS passphrases do not match"

    echo "Enter user password (twice for confirmation):"
    read -s -p "User password: " USER_PASSWORD
    echo
    read -s -p "Confirm user password: " confirm
    echo
    [[ "$USER_PASSWORD" == "$confirm" ]] || error "User passwords do not match"
}

load_tui_config() {
    local env_file="/tmp/nomarchy-install.env"
    if [[ -f "$env_file" ]]; then
        log "Loading configuration from TUI ($env_file)..."
        # shellcheck source=/dev/null
        source "$env_file"
        return 0
    fi
    return 1
}

setup_wifi() {
    if [[ -z "${WIFI_SSID:-}" ]]; then
        return
    fi
    log "Connecting to WiFi: $WIFI_SSID"
    nmcli device wifi connect "$WIFI_SSID" password "${WIFI_PASSWORD:-}" 2>/dev/null \
        || log "WiFi connect via nmcli failed — trying iwctl fallback"
    # Verify connectivity
    if ! ping -c1 -W3 archlinux.org &>/dev/null; then
        log "WiFi connection could not be verified — continuing anyway"
    fi
}

persist_wifi() {
    [[ -n "${WIFI_SSID:-}" ]] || return 0

    local conn_dir="/mnt/etc/NetworkManager/system-connections"
    local conn_file="$conn_dir/${WIFI_SSID}.nmconnection"
    local uuid
    uuid=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)

    log "Persisting WiFi connection '$WIFI_SSID' into installed system..."
    mkdir -p "$conn_dir"

    if [[ -n "${WIFI_PASSWORD:-}" ]]; then
        cat > "$conn_file" <<EOF
[connection]
id=${WIFI_SSID}
uuid=${uuid}
type=wifi
autoconnect=yes

[wifi]
mode=infrastructure
ssid=${WIFI_SSID}

[wifi-security]
auth-alg=open
key-mgmt=wpa-psk
psk=${WIFI_PASSWORD}

[ipv4]
method=auto

[ipv6]
addr-gen-mode=default
method=auto
EOF
    else
        cat > "$conn_file" <<EOF
[connection]
id=${WIFI_SSID}
uuid=${uuid}
type=wifi
autoconnect=yes

[wifi]
mode=infrastructure
ssid=${WIFI_SSID}

[ipv4]
method=auto

[ipv6]
addr-gen-mode=default
method=auto
EOF
    fi

    chmod 600 "$conn_file"
    log "WiFi profile written: $conn_file"
}

partition_disk() {
    log "Partitioning $INSTALL_DISK..."
    sgdisk --zap-all "$INSTALL_DISK"
    sgdisk -n 1:0:+${INSTALL_BOOT_SIZE} -t 1:ef00 -c 1:"EFI"  "$INSTALL_DISK"
    sgdisk -n 2:0:0                     -t 2:8309 -c 2:"LUKS" "$INSTALL_DISK"
    partprobe "$INSTALL_DISK"
    sleep 2

    # Detect partition suffixes
    if [[ "$INSTALL_DISK" =~ nvme|mmcblk ]]; then
        BOOT_PART="${INSTALL_DISK}p1"
        LUKS_PART="${INSTALL_DISK}p2"
    else
        BOOT_PART="${INSTALL_DISK}1"
        LUKS_PART="${INSTALL_DISK}2"
    fi
}

setup_luks() {
    log "Setting up LUKS2 on $LUKS_PART..."
    echo -n "$LUKS_PASSPHRASE" | cryptsetup luksFormat \
        --type luks2 \
        --pbkdf argon2id \
        --iter-time 2000 \
        --key-file - \
        "$LUKS_PART"

    echo -n "$LUKS_PASSPHRASE" | cryptsetup open \
        --key-file - \
        "$LUKS_PART" cryptroot
}

setup_btrfs() {
    mkfs.btrfs -L ROOT /dev/mapper/cryptroot
    mount /dev/mapper/cryptroot /mnt

    for sv in @ @home @log @pkg @snapshots; do
        btrfs subvolume create "/mnt/$sv"
    done
    umount /mnt

    local opts="compress=zstd,noatime,space_cache=v2"
    mount -o "subvol=@,$opts"           /dev/mapper/cryptroot /mnt
    mkdir -p /mnt/{boot,home,var/log,var/cache/pacman/pkg,.snapshots}
    mount -o "subvol=@home,$opts"       /dev/mapper/cryptroot /mnt/home
    mount -o "subvol=@log,$opts"        /dev/mapper/cryptroot /mnt/var/log
    mount -o "subvol=@pkg,$opts"        /dev/mapper/cryptroot /mnt/var/cache/pacman/pkg
    mount -o "subvol=@snapshots,$opts"  /dev/mapper/cryptroot /mnt/.snapshots

    mkfs.fat -F32 -n EFI "$BOOT_PART"
    mount "$BOOT_PART" /mnt/boot
}

install_base() {
    pacstrap -K /mnt \
        base base-devel linux linux-zen linux-firmware \
        btrfs-progs \
        networkmanager \
        sudo \
        git \
        vim \
        greetd \
        pipewire wireplumber \
        $(grep -v '^\s*#' "$SCRIPT_DIR/packages/base.txt" | grep -v '^\s*$')

    genfstab -U /mnt >> /mnt/etc/fstab
}

configure_system() {
    arch-chroot /mnt ln -sf "/usr/share/zoneinfo/$INSTALL_TIMEZONE" /etc/localtime
    arch-chroot /mnt hwclock --systohc

    echo "${INSTALL_LOCALE} UTF-8" >> /mnt/etc/locale.gen
    arch-chroot /mnt locale-gen
    echo "LANG=${INSTALL_LOCALE}" > /mnt/etc/locale.conf
    echo "KEYMAP=${INSTALL_KB_LAYOUT}" > /mnt/etc/vconsole.conf

    echo "$INSTALL_HOSTNAME" > /mnt/etc/hostname

    arch-chroot /mnt useradd -m -G wheel,audio,video,storage,optical "$INSTALL_USER"
    echo "$INSTALL_USER:$USER_PASSWORD" | arch-chroot /mnt chpasswd

    echo "%wheel ALL=(ALL:ALL) ALL" > /mnt/etc/sudoers.d/wheel
    chmod 440 /mnt/etc/sudoers.d/wheel

    arch-chroot /mnt systemctl enable NetworkManager

    # mkinitcpio: plymouth must come before autodetect; plymouth-encrypt replaces encrypt
    mkdir -p /mnt/etc/mkinitcpio.conf.d
    cat > /mnt/etc/mkinitcpio.conf.d/nomarchy.conf <<'EOF'
HOOKS=(base udev plymouth autodetect microcode modconf kms keyboard keymap consolefont block plymouth-encrypt filesystems fsck)
EOF
}

setup_plymouth() {
    local theme_src="$SCRIPT_DIR/default/plymouth"
    local theme_dst="/mnt/usr/share/plymouth/themes/nomarchy"

    mkdir -p "$theme_dst"
    cp "$theme_src"/*.png  "$theme_dst/"
    cp "$theme_src"/nomarchy.script  "$theme_dst/"
    cp "$theme_src"/nomarchy.plymouth "$theme_dst/"

    arch-chroot /mnt plymouth-set-default-theme nomarchy
    arch-chroot /mnt mkinitcpio -P
}

setup_bootloader() {
    arch-chroot /mnt bootctl install

    cat > /mnt/boot/loader/loader.conf <<'EOF'
default  arch.conf
timeout  3
console-mode max
editor   no
EOF

    local luks_uuid
    luks_uuid=$(blkid -s UUID -o value "$LUKS_PART")

    cat > /mnt/boot/loader/entries/arch.conf <<EOF
title   Arch Linux
linux   /vmlinuz-linux
initrd  /initramfs-linux.img
options cryptdevice=UUID=${luks_uuid}:cryptroot root=/dev/mapper/cryptroot rootflags=subvol=@ rw quiet splash
EOF

    cat > /mnt/boot/loader/entries/arch-zen.conf <<EOF
title   Arch Linux (zen)
linux   /vmlinuz-linux-zen
initrd  /initramfs-linux-zen.img
options cryptdevice=UUID=${luks_uuid}:cryptroot root=/dev/mapper/cryptroot rootflags=subvol=@ rw quiet splash
EOF

    echo "cryptdevice=UUID=${luks_uuid}:cryptroot root=/dev/mapper/cryptroot rootflags=subvol=@ rw quiet splash" \
        > /mnt/etc/kernel/cmdline
}

setup_greetd() {
    mkdir -p /mnt/etc/greetd
    cat > /mnt/etc/greetd/config.toml <<EOF
[terminal]
vt = 1

[default_session]
command = "/usr/bin/uwsm start hyprland.desktop"
user = "$INSTALL_USER"

[initial_session]
command = "/usr/bin/uwsm start hyprland.desktop"
user = "$INSTALL_USER"
EOF

    arch-chroot /mnt systemctl enable greetd
    arch-chroot /mnt systemctl mask getty@tty1.service
}

copy_setup_files() {
    local target="/mnt/home/$INSTALL_USER/.local/share/nomarchy"
    mkdir -p "$target"
    rsync -a --exclude='.git' --exclude='iso/' "$SCRIPT_DIR/" "$target/"
    arch-chroot /mnt chown -R "$INSTALL_USER:$INSTALL_USER" "/home/$INSTALL_USER/.local"
}

enable_user_setup() {
    # Temporary passwordless sudo so setup.sh can run unattended in the foot terminal.
    # setup.sh removes this rule in mark_complete() when it finishes.
    echo "$INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL" \
        | install -m 440 /dev/stdin /mnt/etc/sudoers.d/nomarchy-setup

    # Systemd user service: opens a foot terminal running setup.sh on first Hyprland login.
    # ConditionPathExists prevents it from firing again after setup.sh writes its sentinel.
    local svc_dir="/mnt/home/$INSTALL_USER/.config/systemd/user"
    local wants_dir="$svc_dir/graphical-session.target.wants"
    mkdir -p "$wants_dir"

    cat > "$svc_dir/nomarchy-setup.service" <<'EOF'
[Unit]
Description=nomarchy First Boot Setup
ConditionPathExists=!%h/.local/state/nomarchy/setup-done
After=graphical-session.target

[Service]
Type=oneshot
ExecStart=foot -- bash -c 'bash %h/.local/share/nomarchy/setup.sh; exec bash'
Restart=no

[Install]
WantedBy=graphical-session.target
EOF

    ln -s ../nomarchy-setup.service "$wants_dir/nomarchy-setup.service"
    arch-chroot /mnt chown -R "$INSTALL_USER:$INSTALL_USER" "/home/$INSTALL_USER/.config"
}

enable_boot_finalize() {
    cat > /mnt/etc/systemd/system/nomarchy-finalize.service <<'EOF'
[Unit]
Description=nomarchy Boot Finalization
DefaultDependencies=no
After=local-fs.target
Wants=local-fs.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/bin/nomarchy-finalize.sh

[Install]
WantedBy=multi-user.target
EOF

    cat > /mnt/usr/local/bin/nomarchy-finalize.sh <<'EOF'
#!/bin/bash
for hook in /usr/share/libalpm/hooks/*mkinitcpio*.hook.disabled; do
    [[ -f "$hook" ]] && mv "$hook" "${hook%.disabled}"
done
mkinitcpio -P
bootctl update
systemctl disable nomarchy-finalize.service
EOF

    chmod +x /mnt/usr/local/bin/nomarchy-finalize.sh
    arch-chroot /mnt systemctl enable nomarchy-finalize.service
}

main() {
    # Load TUI config if available, otherwise fall back to interactive prompts
    if ! load_tui_config; then
        log "No TUI config found — using interactive prompts."

        # Network check
        if ! ping -c1 -W3 archlinux.org &>/dev/null; then
            error "No internet connection. Connect via iwctl or dhcpcd and retry."
        fi

        # Prompt for install disk if not set
        if [[ -z "$INSTALL_DISK" ]]; then
            echo "Available disks:"
            lsblk -d -o NAME,SIZE,MODEL | grep -v NAME
            read -p "Enter target disk (e.g., /dev/sda): " INSTALL_DISK
        fi

        prompt_passwords
    else
        # Apply TUI-supplied keyboard layout (if loadkeys is available)
        if [[ -n "${INSTALL_KB_LAYOUT:-}" ]]; then
            loadkeys "$INSTALL_KB_LAYOUT" 2>/dev/null || true
        fi

        # Connect WiFi if credentials were provided
        setup_wifi

        # Network check (after potential WiFi setup)
        if ! ping -c1 -W3 archlinux.org &>/dev/null; then
            error "No internet connection. Check WiFi/ethernet and retry."
        fi

        # Validate required variables
        [[ -n "${INSTALL_DISK:-}" ]]      || error "INSTALL_DISK not set in TUI config"
        [[ -n "${LUKS_PASSPHRASE:-}" ]]   || error "LUKS_PASSPHRASE not set in TUI config"
        [[ -n "${USER_PASSWORD:-}" ]]     || error "USER_PASSWORD not set in TUI config"
        [[ -n "${INSTALL_USER:-}" ]]      || error "INSTALL_USER not set in TUI config"
        [[ -n "${INSTALL_HOSTNAME:-}" ]]  || error "INSTALL_HOSTNAME not set in TUI config"
        [[ -n "${INSTALL_TIMEZONE:-}" ]]  || error "INSTALL_TIMEZONE not set in TUI config"

        log "Disk       : $INSTALL_DISK"
        log "User       : $INSTALL_USER"
        log "Hostname   : $INSTALL_HOSTNAME"
        log "Timezone   : $INSTALL_TIMEZONE"
        log "KB Layout  : ${INSTALL_KB_LAYOUT:-default}"
    fi

    partition_disk
    setup_luks
    setup_btrfs
    install_base
    persist_wifi
    configure_system
    setup_plymouth
    setup_bootloader
    setup_greetd
    copy_setup_files
    enable_user_setup
    enable_boot_finalize

    log "Installation complete. Reboot and enjoy!"
}

main "$@"