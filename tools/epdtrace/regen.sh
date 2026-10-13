#!/bin/sh
# Regenerate internal/epd/testdata/trace/*.txt from an upstream Inkycal checkout.
# usage: tools/epdtrace/regen.sh <path to aceinnolab/Inkycal checkout> model:w:h ...
set -eu
up="$1"; shift
here="$(cd "$(dirname "$0")" && pwd)"
out="$here/../../internal/epd/testdata/trace"
for spec in "$@"; do
	model="${spec%%:*}"; rest="${spec#*:}"; w="${rest%%:*}"; h="${rest#*:}"
	python3 -I "$here/trace.py" "$up/inkycal/display/drivers" "$model" "$w" "$h" > "$out/$model.txt"
	echo "$model: $(wc -l < "$out/$model.txt") ops"
done
