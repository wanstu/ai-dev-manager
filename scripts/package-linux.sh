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

package_version="${artifact_version#v}"
if [[ ! "$package_version" =~ ^[0-9]+([.][0-9]+){1,2}([+~A-Za-z0-9.-]*)?$ ]]; then
  package_version="0.0.0~$package_version"
fi

tmp_root="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_root"
}
trap cleanup EXIT

echo "Building Linux CLI..."
(
  cd "$REPO_ROOT"
  go build -trimpath \
    -ldflags="-X=ai-dev-manager-v2/internal/version.Version=$VERSION -s -w" \
    -o "$tmp_root/adm" ./cmd/ai-dev-manager
)

cat > "$tmp_root/adm-desktop.desktop" <<'EOF'
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

mkdir -p "$OUTPUT_DIR"

description=$'AI Dev Manager local development control plane\nADM provides Workspace, Environment, execution policy, MCP, Skill, Memory, verifier and runtime lifecycle management through CLI, Gateway and Desktop surfaces.'

(
  cd "$REPO_ROOT"
  go run github.com/wanstu/wails-desktop-kit/cmd/desktopkit package linux \
    --input "$DESKTOP_BINARY" \
    --dist "$OUTPUT_DIR" \
    --app-name adm-desktop \
    --asset-base "adm-$artifact_version" \
    --package-name adm \
    --package-version "$package_version" \
    --arch "$ARCH" \
    --formats deb,tar.gz \
    --description "$description" \
    --maintainer "AI Dev Manager" \
    --section devel \
    --priority optional \
    --depends "libgtk-3-0t64 | libgtk-3-0, libwebkit2gtk-4.1-0" \
    --desktop-file "$tmp_root/adm-desktop.desktop" \
    --icon "$REPO_ROOT/assets/icons/ai-dev-manager-app.png" \
    --extra-bin "adm=$tmp_root/adm"
)

echo "Linux packages:"
find "$OUTPUT_DIR" -maxdepth 1 -type f -name "adm-$artifact_version-linux-$ARCH*" -print | sort
