#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
DESKTOP_BINARY="${2:-}"
OUTPUT_DIR="${3:-}"
ARCH="${4:-amd64}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

if [[ -z "$DESKTOP_BINARY" ]]; then
  DESKTOP_BINARY="$REPO_ROOT/cmd/ai-dev-manager-desktop/build/bin/adm-desktop-linux-$ARCH"
elif [[ "$DESKTOP_BINARY" != /* ]]; then
  DESKTOP_BINARY="$REPO_ROOT/$DESKTOP_BINARY"
fi

if [[ -z "$OUTPUT_DIR" ]]; then
  OUTPUT_DIR="$REPO_ROOT/dist"
elif [[ "$OUTPUT_DIR" != /* ]]; then
  OUTPUT_DIR="$REPO_ROOT/$OUTPUT_DIR"
fi

case "$ARCH" in
  amd64|arm64) ;;
  *)
    echo "unsupported architecture: $ARCH" >&2
    exit 2
    ;;
esac

if [[ ! -x "$DESKTOP_BINARY" ]]; then
  echo "Linux Desktop binary not found or not executable: $DESKTOP_BINARY" >&2
  exit 1
fi

artifact_version="$(printf '%s' "$VERSION" | sed -E 's/[^A-Za-z0-9.+~-]+/-/g')"
if [[ -z "$artifact_version" ]]; then
  artifact_version="dev"
fi

deb_source_version="${artifact_version#v}"
if [[ "$deb_source_version" =~ ^[0-9]+([.][0-9]+){1,2}([+~A-Za-z0-9.-]*)?$ ]]; then
  deb_version="$deb_source_version"
else
  deb_version="0.0.0~$deb_source_version"
fi

artifact_base="adm-$artifact_version-linux-$ARCH"
tar_path="$OUTPUT_DIR/$artifact_base.tar.gz"
deb_path="$OUTPUT_DIR/$artifact_base.deb"

tmp_root="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_root"
}
trap cleanup EXIT

portable_root="$tmp_root/$artifact_base"
mkdir -p "$portable_root/bin" "$portable_root/share/applications" "$portable_root/share/icons/hicolor/256x256/apps"

echo "Building Linux CLI..."
(
  cd "$REPO_ROOT"
  go build -trimpath -ldflags="-X=ai-dev-manager-v2/internal/version.Version=$VERSION -s -w" -o "$portable_root/bin/adm" ./cmd/ai-dev-manager
)

install -m 0755 "$DESKTOP_BINARY" "$portable_root/bin/adm-desktop"
install -m 0644 "$REPO_ROOT/assets/icons/ai-dev-manager-app.png" "$portable_root/share/icons/hicolor/256x256/apps/adm-desktop.png"

cat > "$portable_root/share/applications/adm-desktop.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Version=1.0
Name=AI Dev Manager
Comment=Local development control plane
Exec=adm-desktop
Icon=adm-desktop
Terminal=false
Categories=Development;
StartupNotify=true
EOF

cat > "$portable_root/README.txt" <<EOF
AI Dev Manager Linux portable package
Version: $VERSION
Architecture: $ARCH

Run:
  ./bin/adm --help
  ./bin/adm-desktop

Debian 13 runtime dependencies:
  sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0

Debian 12 / compatible Ubuntu runtime dependencies usually use:
  sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0

The portable archive does not bundle GTK/WebKitGTK.
The .deb package declares these runtime dependencies automatically.
EOF

mkdir -p "$OUTPUT_DIR"
tar -C "$tmp_root" -czf "$tar_path" "$artifact_base"

deb_root="$tmp_root/deb"
mkdir -p   "$deb_root/DEBIAN"   "$deb_root/usr/bin"   "$deb_root/usr/share/applications"   "$deb_root/usr/share/icons/hicolor/256x256/apps"   "$deb_root/usr/share/doc/adm"

install -m 0755 "$portable_root/bin/adm" "$deb_root/usr/bin/adm"
install -m 0755 "$portable_root/bin/adm-desktop" "$deb_root/usr/bin/adm-desktop"
install -m 0644 "$portable_root/share/icons/hicolor/256x256/apps/adm-desktop.png" "$deb_root/usr/share/icons/hicolor/256x256/apps/adm-desktop.png"

cat > "$deb_root/usr/share/applications/adm-desktop.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Version=1.0
Name=AI Dev Manager
Comment=Local development control plane
Exec=/usr/bin/adm-desktop
Icon=adm-desktop
Terminal=false
Categories=Development;
StartupNotify=true
EOF

cat > "$deb_root/usr/share/doc/adm/README.Debian" <<EOF
AI Dev Manager $VERSION

CLI:
  adm --help

Desktop:
  adm-desktop

The Desktop requires GTK 3 and WebKitGTK 4.1.
EOF

cat > "$deb_root/DEBIAN/control" <<EOF
Package: adm
Version: $deb_version
Section: devel
Priority: optional
Architecture: $ARCH
Maintainer: AI Dev Manager
Depends: libgtk-3-0t64 | libgtk-3-0, libwebkit2gtk-4.1-0
Description: AI Dev Manager local development control plane
 ADM provides Workspace, Environment, execution policy, MCP, Skill,
 Memory, verifier and runtime lifecycle management through CLI,
 Gateway and Desktop surfaces.
EOF

chmod 0755 "$deb_root/DEBIAN"
find "$deb_root" -type d -exec chmod 0755 {} +
dpkg-deb --root-owner-group --build "$deb_root" "$deb_path" >/dev/null

checksum_path="$OUTPUT_DIR/$artifact_base.sha256"
(
  cd "$OUTPUT_DIR"
  sha256sum "$(basename "$tar_path")" "$(basename "$deb_path")" > "$(basename "$checksum_path")"
)

echo "Linux portable artifact: $tar_path"
echo "Linux Debian artifact:   $deb_path"
echo "Linux checksums:         $checksum_path"
