#!/bin/sh
# Regenerate internal/epd/testdata/trace/<model>.txt for the 12.48" drivers (trace12.py).
# usage: tools/epdtrace/regen12.sh <path to aceinnolab/Inkycal checkout> model:w:h ...
set -eu
up="$1"; shift
here="$(cd "$(dirname "$0")" && pwd)"
out="$here/../../internal/epd/testdata/trace"
for spec in "$@"; do
	model="${spec%%:*}"; rest="${spec#*:}"; w="${rest%%:*}"; h="${rest#*:}"
	python3 -I "$here/trace12.py" "$up/inkycal/display/drivers" "$model" "$w" "$h" > "$out/$model.txt"
	echo "$model: $(wc -l < "$out/$model.txt") ops"
done
