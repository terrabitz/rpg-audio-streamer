#!/bin/sh
set -eu

prefix=${1:?provide an absolute installation directory}
case "$prefix" in
  /*) ;;
  *) echo "FFmpeg installation directory must be absolute" >&2; exit 1 ;;
esac

version=7.1.5
digest=de668509caf9e35e3cd162473441fdb29538c6d96ed080292b3cf9e6fc5d558f
workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT HUP INT TERM

curl -fsSL --retry 3 "https://ffmpeg.org/releases/ffmpeg-$version.tar.xz" -o "$workdir/ffmpeg.tar.xz"
printf '%s  %s\n' "$digest" "$workdir/ffmpeg.tar.xz" | sha256sum -c -
tar -xJf "$workdir/ffmpeg.tar.xz" -C "$workdir"
cd "$workdir/ffmpeg-$version"

./configure --prefix="$prefix" \
  --enable-shared --disable-static \
  --disable-autodetect --disable-doc --disable-debug \
  --disable-devices --enable-indev=lavfi \
  --disable-ffplay --enable-version3 --enable-openssl --enable-libmp3lame
make -j2 ffmpeg ffprobe
make install
rm -rf "$prefix/lib/pkgconfig"
mkdir -p "$prefix/licenses"
cp COPYING.* LICENSE.md "$prefix/licenses/"
