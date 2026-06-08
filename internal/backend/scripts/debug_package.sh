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
    _entry_log="$(mktemp)"
    _entry_detail_log="$(mktemp)"
    if rpm2cpio "$DEBUG_PACKAGE" | cpio -it > "$_entry_log"; then
      validate_archive_entries "$_entry_log"
    else
      echo "[ERROR] rpm archive listing failed (code $?)" >&2
      cat "$_entry_log" >&2 || true
      rm -f "$_entry_log"
      rm -f "$_entry_detail_log"
      exit 33
    fi
    if rpm2cpio "$DEBUG_PACKAGE" | cpio -itv > "$_entry_detail_log"; then
      validate_archive_entry_types "$_entry_detail_log"
    else
      echo "[ERROR] rpm archive detail listing failed (code $?)" >&2
      cat "$_entry_detail_log" >&2 || true
      rm -f "$_entry_log" "$_entry_detail_log"
      exit 37
    fi
    EXTRACT_TOTAL="$(wc -l < "$_entry_log" | tr -d ' ')"
    rm -f "$_entry_log" "$_entry_detail_log"
    echo "VOLSYM_EXTRACT_TOTAL=$EXTRACT_TOTAL"
    _extract_log="$(mktemp)"
    if ( cd "$WORK_DIR/extract" && rpm2cpio "$DEBUG_PACKAGE" | cpio --no-absolute-filenames --no-preserve-owner -idmv ) 2>&1 > "$_extract_log"; then
      EXTRACT_CURRENT=0
      while IFS= read -r extracted; do
        if [ -n "$extracted" ]; then
          EXTRACT_CURRENT=$((EXTRACT_CURRENT + 1))
          echo "VOLSYM_EXTRACT_FILE=$EXTRACT_CURRENT/$EXTRACT_TOTAL:$extracted"
        fi
      done < "$_extract_log"
    else
      echo "[ERROR] rpm extraction failed (code $?)" >&2
      cat "$_extract_log" >&2 || true
      rm -f "$_extract_log"
      exit 29
    fi
    rm -f "$_extract_log"
    ;;
  deb|ddeb|unknown|"")
    command -v dpkg-deb >/dev/null || { echo "[ERROR] missing dpkg-deb" >&2; exit 20; }
    command -v tar >/dev/null || { echo "[ERROR] missing tar" >&2; exit 25; }
    _entry_log="$(mktemp)"
    _entry_detail_log="$(mktemp)"
    if dpkg-deb --fsys-tarfile "$DEBUG_PACKAGE" | tar -tf - > "$_entry_log"; then
      validate_archive_entries "$_entry_log"
    else
      echo "[ERROR] deb archive listing failed (code $?)" >&2
      cat "$_entry_log" >&2 || true
      rm -f "$_entry_log" "$_entry_detail_log"
      exit 34
    fi
    if dpkg-deb --fsys-tarfile "$DEBUG_PACKAGE" | tar -tvf - > "$_entry_detail_log"; then
      validate_archive_entry_types "$_entry_detail_log"
    else
      echo "[ERROR] deb archive detail listing failed (code $?)" >&2
      cat "$_entry_detail_log" >&2 || true
      rm -f "$_entry_log" "$_entry_detail_log"
      exit 38
    fi
    EXTRACT_TOTAL="$(awk 'length($0) && substr($0, length($0), 1) != "/" { count++ } END { print count + 0 }' "$_entry_log")"
    rm -f "$_entry_log" "$_entry_detail_log"
    echo "VOLSYM_EXTRACT_TOTAL=$EXTRACT_TOTAL"
    EXTRACT_CURRENT=0
    _extract_log="$(mktemp)"
    if dpkg-deb --fsys-tarfile "$DEBUG_PACKAGE" | tar --no-same-owner --no-same-permissions -xvf - -C "$WORK_DIR/extract" > "$_extract_log"; then
      while IFS= read -r extracted; do
        if [ -n "$extracted" ] && [ "${extracted%/}" = "$extracted" ]; then
          EXTRACT_CURRENT=$((EXTRACT_CURRENT + 1))
          echo "VOLSYM_EXTRACT_FILE=$EXTRACT_CURRENT/$EXTRACT_TOTAL:$extracted"
        fi
      done < "$_extract_log"
    else
      echo "[ERROR] deb extraction failed (code $?)" >&2
      cat "$_extract_log" >&2 || true
      rm -f "$_extract_log"
      exit 30
    fi
    rm -f "$_extract_log"
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
ensure_path_under "$VMLINUX" "$WORK_DIR"
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
