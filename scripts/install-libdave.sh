#!/bin/sh
set -eu

prefix=${1:?provide an absolute installation directory}
case "$prefix" in
  /*) ;;
  *) echo "libdave installation directory must be absolute" >&2; exit 1 ;;
esac

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    asset=libdave-Linux-X64-boringssl.zip
    digest=33157b8cbadcdd3c6cb4df3be18e8bbe7b86e3c65d57d0f1bc9dadc62a768d6a
    library=libdave.so
    ;;
  Linux-aarch64|Linux-arm64)
    asset=libdave-Linux-ARM64-boringssl.zip
    digest=fc119512aefac9cb7652634c16ecb0833a3c4bf91d87b9ec8d6acda03d454349
    library=libdave.so
    ;;
  Darwin-x86_64)
    asset=libdave-macOS-X64-boringssl.zip
    digest=996dad1e812ef76ba7b73fdd26cff30af5dbc1fc1f2389a1107752212c41950b
    library=libdave.dylib
    ;;
  Darwin-arm64)
    asset=libdave-macOS-ARM64-boringssl.zip
    digest=05166ca29afb30b4bcb7dd1d320b63ff14f2f4a179050ced8b91305ff86cc991
    library=libdave.dylib
    ;;
  *) echo "unsupported libdave platform: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

archive=$(mktemp)
trap 'rm -f "$archive"' EXIT HUP INT TERM
curl -fsSL "https://github.com/discord/libdave/releases/download/v1.1.0/cpp/$asset" -o "$archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$archive")
else
  actual=$(shasum -a 256 "$archive")
fi
actual=${actual%% *}
if [ "$actual" != "$digest" ]; then
  echo "libdave archive checksum mismatch" >&2
  exit 1
fi
mkdir -p "$prefix/include" "$prefix/lib/pkgconfig" "$prefix/share/licenses/libdave"
unzip -p "$archive" include/dave/dave.h > "$prefix/include/dave.h"
unzip -p "$archive" "lib/$library" > "$prefix/lib/$library"
unzip -jo "$archive" 'licenses/*' -d "$prefix/share/licenses/libdave" >/dev/null
printf 'prefix=%s\nlibdir=${prefix}/lib\nincludedir=${prefix}/include\n\nName: dave\nDescription: Discord libdave\nVersion: 1.1.0\nLibs: -L${libdir} -ldave\nCflags: -I${includedir}\n' "$prefix" > "$prefix/lib/pkgconfig/dave.pc"
echo "Installed libdave 1.1.0 in $prefix"
