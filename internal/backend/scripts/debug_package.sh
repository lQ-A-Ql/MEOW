#!/usr/bin/env bash
set -euo pipefail

rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/extract" "$OUT_DIR"
command -v xz >/dev/null || { echo "[ERROR] missing xz" >&2; exit 21; }
command -v dwarf2json >/dev/null || { echo "[ERROR] missing dwarf2json" >&2; exit 22; }
echo "VOLSYM_STAGE=extract"
case "$PACKAGE_FORMAT" in
  rpm)
    command -v rpm2cpio >/dev/null || { echo "[ERROR] missing rpm2cpio" >&2; exit 26; }
    command -v cpio >/dev/null || { echo "[ERROR] missing cpio" >&2; exit 27; }
    echo "VOLSYM_EXTRACT_TOTAL=1"
    ( cd "$WORK_DIR/extract" && rpm2cpio "$DEBUG_PACKAGE" | cpio -idmv ) 2>&1 | while IFS= read -r extracted; do
      if [ -n "$extracted" ]; then
        echo "VOLSYM_EXTRACT_FILE=1/1:$extracted"
      fi
    done
    ;;
  deb|ddeb|unknown|"")
    command -v dpkg-deb >/dev/null || { echo "[ERROR] missing dpkg-deb" >&2; exit 20; }
    command -v tar >/dev/null || { echo "[ERROR] missing tar" >&2; exit 25; }
    EXTRACT_TOTAL="$(dpkg-deb -c "$DEBUG_PACKAGE" | awk '$1 !~ /^d/ { count++ } END { print count + 0 }')"
    echo "VOLSYM_EXTRACT_TOTAL=$EXTRACT_TOTAL"
    EXTRACT_CURRENT=0
    dpkg-deb --fsys-tarfile "$DEBUG_PACKAGE" | tar -xvf - -C "$WORK_DIR/extract" | while IFS= read -r extracted; do
      if [ -n "$extracted" ] && [ "${extracted%/}" = "$extracted" ]; then
        EXTRACT_CURRENT=$((EXTRACT_CURRENT + 1))
        echo "VOLSYM_EXTRACT_FILE=$EXTRACT_CURRENT/$EXTRACT_TOTAL:$extracted"
      fi
    done
    ;;
  *)
    echo "[ERROR] unsupported debug package format: $PACKAGE_FORMAT" >&2
    exit 28
    ;;
esac
echo "VOLSYM_STAGE=find_vmlinux"
VMLINUX=""
for candidate in \
  "$WORK_DIR/extract/usr/lib/debug/boot/vmlinux-$KERNEL" \
  "$WORK_DIR/extract/usr/lib/debug/lib/modules/$KERNEL/vmlinux" \
  "$WORK_DIR/extract/usr/lib/debug/lib64/modules/$KERNEL/vmlinux"; do
  if [ -f "$candidate" ]; then
    VMLINUX="$candidate"
    break
  fi
done
if [ -z "$VMLINUX" ]; then
  VMLINUX="$(find "$WORK_DIR/extract" -type f \( -name "vmlinux" -o -name "vmlinux-*" -o -name "vmlinux*.gz" -o -name "vmlinux*.xz" -o -name "vmlinux*.zst" \) | head -n 1 || true)"
fi
if [ -z "$VMLINUX" ]; then
  echo "[ERROR] debug package extracted but vmlinux not found" >&2
  find "$WORK_DIR/extract" -type f | head -n 50 >&2 || true
  exit 23
fi
case "$VMLINUX" in
  *.gz)
    command -v gzip >/dev/null || { echo "[ERROR] missing gzip" >&2; exit 29; }
    gzip -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
  *.xz)
    xz -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
  *.zst)
    command -v zstd >/dev/null || { echo "[ERROR] missing zstd" >&2; exit 30; }
    zstd -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
esac
echo "VOLSYM_VMLINUX=$VMLINUX"
echo "VOLSYM_STAGE=dwarf2json"
dwarf2json linux --elf "$VMLINUX" > "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=compress"
xz -T0 -f -z "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=move"
mv -f "$WORK_DIR/symbol.json.xz" "$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_SYMBOL=$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_STAGE=done"
