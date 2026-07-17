#!/bin/sh

set -eu

version=${1:-}

if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
	echo "error: invalid semantic version tag: $version" >&2
	exit 1
fi

case "$version" in
	*+*)
		echo "error: build metadata is not permitted in release tags: $version" >&2
		exit 1
		;;
esac

case "$version" in
	*-*)
		prerelease=${version#*-}
		old_ifs=$IFS
		IFS=.
		for identifier in $prerelease; do
			case "$identifier" in
				*[!0-9]*) ;;
				0 | [1-9] | [1-9][0-9]*) ;;
				*)
					echo "error: numeric prerelease identifiers must not contain leading zeroes: $version" >&2
					exit 1
					;;
			esac
		done
		IFS=$old_ifs
		;;
esac
