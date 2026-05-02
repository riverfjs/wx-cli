#!/bin/bash
# Install voice dependencies (user-level, no sudo required)
# Usage: bash setup-voice.sh
set -eo pipefail

echo "=== wx-cli voice setup ==="

echo "Installing edge-tts..."
pip install --break-system-packages -q edge-tts 2>/dev/null \
  || pip install --user -q edge-tts 2>/dev/null \
  || pip install -q edge-tts
python3 -c "import edge_tts" && echo "  OK" || { echo "  FAILED"; exit 1; }

echo ""
echo "Done. Voice reply is ready."
