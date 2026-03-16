# Overwrite parts of the nomarchy-menu with user-specific submenus.
# See $NOMARCHY_PATH/bin/nomarchy-menu for functions that can be overwritten.
#
# WARNING: Overwritten functions will obviously not be updated when Nomarchy changes.
#
# Example of minimal system menu:
#
# show_system_menu() {
#   case $(menu "System" "  Lock\n󰐥  Shutdown") in
#   *Lock*) nomarchy-lock-screen ;;
#   *Shutdown*) nomarchy-system-shutdown ;;
#   *) back_to show_main_menu ;;
#   esac
# }
#
# Example of overriding just the about menu action: (Using zsh instead of bash (default))
#
# show_about() {
#   exec nomarchy-launch-or-focus-tui "zsh -c 'fastfetch; read -k 1'"
# }
