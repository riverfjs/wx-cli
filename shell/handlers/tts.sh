#!/bin/bash
# Text-to-Speech → MP3 audio file
# Usage: bash tts.sh "text" [output.mp3]
# Outputs: file path on stdout
set -eo pipefail
export PATH="$HOME/.local/bin:$PATH"

TEXT="$1"
OUTPUT="${2:-$(mktemp /tmp/wx-tts-XXXXXX.mp3)}"
VOICE="${TTS_VOICE:-zh-CN-XiaoxiaoNeural}"

if [[ -z "$TEXT" ]]; then
  echo "Usage: bash tts.sh \"text\" [output.mp3]" >&2
  exit 1
fi

edge-tts --voice "$VOICE" --text "$TEXT" --write-media "$OUTPUT" 2>/dev/null

echo "$OUTPUT"
