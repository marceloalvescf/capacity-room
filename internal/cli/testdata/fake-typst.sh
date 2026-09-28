#!/bin/sh
# fake-typst.sh stands in for the real typst binary in tests. It is run with
# an empty environment (internal/typst always clears it), so every command it
# runs is named by its full path.
#
# --version answers with the pinned version. Any "compile" invocation copies
# the file named by "--input data=<path>" (interpreted relative to --root) to
# its last argument, after writing the line "%PDF-fake" first.
if [ "$1" = "--version" ]; then
	/bin/echo 'typst 0.15.1 (fake)'
	exit 0
fi

root=""
data_value=""
prev=""
last=""
for arg in "$@"; do
	if [ "$prev" = "--root" ]; then
		root="$arg"
	fi
	if [ "$prev" = "--input" ]; then
		case "$arg" in
		data=*)
			data_value="${arg#data=}"
			;;
		esac
	fi
	prev="$arg"
	last="$arg"
done

output="$last"
/bin/echo '%PDF-fake' >"$output"
/bin/cat "$root$data_value" >>"$output"
