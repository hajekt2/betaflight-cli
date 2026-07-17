#!/bin/sh

set -eu

validator=./scripts/validate-version.sh

for version in v0.1.0 v1.0.0 v1.2.3-rc.1 v2.0.0-0; do
	"$validator" "$version"
done

for version in \
	'' \
	0.1.0 \
	v01.2.3 \
	v1.02.3 \
	v1.2.03 \
	v1.2 \
	v1.2.3- \
	v1.2.3-. \
	v1.2.3-a..b \
	v1.2.3-01 \
	v1.2.3-rc.01 \
	v1.2.3+build; do
	if "$validator" "$version" >/dev/null 2>&1; then
		echo "error: accepted invalid or unsupported release tag: $version" >&2
		exit 1
	fi
done

echo "release script tests passed"
