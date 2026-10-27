#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
icon_source="$project_dir/build/appicon.png"

if [ "$#" -gt 0 ]; then
	app_binary=$1
elif [ -x "$project_dir/build/bin/packet-drift" ]; then
	app_binary="$project_dir/build/bin/packet-drift"
elif [ -x "$project_dir/build/bin/packet-drift-dev-linux-amd64" ]; then
	app_binary="$project_dir/build/bin/packet-drift-dev-linux-amd64"
elif command -v packet-drift >/dev/null 2>&1; then
	app_binary=$(command -v packet-drift)
else
	printf '%s\n' 'Packet Drift binary not found. Build the app or pass its path.' >&2
	exit 1
fi

if [ ! -x "$app_binary" ]; then
	printf 'Not an executable: %s\n' "$app_binary" >&2
	exit 1
fi
if [ ! -f "$icon_source" ]; then
	printf 'App icon not found: %s\n' "$icon_source" >&2
	exit 1
fi

icon_dir="$HOME/.local/share/icons/hicolor/512x512/apps"
desktop_dir="$HOME/.local/share/applications"
install -D -m 644 "$icon_source" "$icon_dir/packet-drift.png"
install -d -m 755 "$desktop_dir"

escaped_binary=$(printf '%s' "$app_binary" | sed -e 's/\\/\\\\/g' -e 's/ /\\ /g' -e 's/&/\\&/g' -e 's|/|\\/|g')
sed "s/^Exec=.*/Exec=$escaped_binary/" "$project_dir/build/linux/packet-drift.desktop" > "$desktop_dir/packet-drift.desktop"
chmod 644 "$desktop_dir/packet-drift.desktop"

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -f "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true
fi
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$desktop_dir" >/dev/null 2>&1 || true
fi

printf 'Installed Packet Drift desktop icon for %s\n' "$app_binary"