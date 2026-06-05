#!/usr/bin/env bash
set -euo pipefail

rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR" "$OUT_DIR"
command -v xz >/dev/null || { echo "[ERROR] missing xz" >&2; exit 21; }
command -v dwarf2json >/dev/null || { echo "[ERROR] missing dwarf2json" >&2; exit 22; }
test -f "$VMLINUX" || { echo "[ERROR] vmlinux not found: $VMLINUX" >&2; exit 24; }
echo "VOLSYM_VMLINUX=$VMLINUX"
echo "VOLSYM_STAGE=dwarf2json"
dwarf2json linux --elf "$VMLINUX" > "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=compress"
xz -T0 -f -z "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=move"
mv -f "$WORK_DIR/symbol.json.xz" "$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_SYMBOL=$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_STAGE=done"
