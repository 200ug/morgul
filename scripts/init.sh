#!/usr/bin/env bash

CFG_DIR="$HOME/.config/morgul"

mkdir -p $CFG_DIR/modules $CFG_DIR/presets
[[ -d examples/modules ]] && cp examples/modules/* $CFG_DIR/modules/.
[[ -d examples/presets ]] && cp examples/presets/* $CFG_DIR/presets/.
[[ -f Dockerfile.base ]] && cp Dockerfile.base $CFG_DIR/.
[[ -f examples/colors.json ]] && cp examples/colors.json $CFG_DIR/.
