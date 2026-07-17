#!/bin/sh

set -eu

GO=${GO:-go}
DIST_DIR=${DIST_DIR:-dist}
VERSION=${VERSION:-}
BINARY=betaflight-cli
repository_dir=$(pwd)

./scripts/validate-version.sh "$VERSION"

for command in "$GO" tar zip unzip; do
	if ! command -v "$command" >/dev/null 2>&1; then
		echo "error: required command not found: $command" >&2
		exit 1
	fi
done

for file in LICENSE README.md THIRD_PARTY_NOTICES.md; do
	if [ ! -f "$file" ]; then
		echo "error: required release file is missing: $file" >&2
		exit 1
	fi
done

release_dir="$repository_dir/$DIST_DIR/release"
version_without_prefix=${VERSION#v}
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

rm -rf "$release_dir"
mkdir -p "$release_dir"

module_sources="$work_dir/go-module-sources.txt"
module_inventory="$work_dir/go-modules.txt"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
	target_os=${target%/*}
	target_arch=${target#*/}
	GOOS=$target_os GOARCH=$target_arch "$GO" list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}' ./cmd/betaflight-cli
done | sort -u > "$module_sources"
cut -d '|' -f 1-2 "$module_sources" > "$module_inventory"
if [ ! -s "$module_inventory" ]; then
	echo "error: Go module inventory is empty" >&2
	exit 1
fi

common_dir="$work_dir/common"
mkdir -p "$common_dir/third_party_licenses"
cp LICENSE README.md THIRD_PARTY_NOTICES.md "$common_dir/"
cp "$module_inventory" "$common_dir/go-modules.txt"

while IFS='|' read -r module version module_dir; do
	[ -n "$module" ] || continue
	if [ -z "$module_dir" ] || [ ! -d "$module_dir" ]; then
		echo "error: module source is unavailable for $module $version" >&2
		exit 1
	fi
	license_files=$(find "$module_dir" -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'COPYING*' -o -iname 'NOTICE*' \) -print | sort)
	if [ -z "$license_files" ]; then
		echo "error: no license file found for $module $version" >&2
		exit 1
	fi
	safe_module=$(printf '%s' "$module" | tr '/@' '__')
	for license_file in $license_files; do
		license_name=$(basename "$license_file")
		cp "$license_file" "$common_dir/third_party_licenses/${safe_module}_${version}_${license_name}"
	done
done < "$module_sources"

package_target() {
	os=$1
	arch=$2
	extension=$3
	archive_format=$4
	source_binary="$repository_dir/$DIST_DIR/${BINARY}-${os}-${arch}${extension}"
	package_name="${BINARY}_${version_without_prefix}_${os}_${arch}"
	package_dir="$work_dir/$package_name"

	if [ ! -f "$source_binary" ]; then
		echo "error: release binary is missing: $source_binary" >&2
		exit 1
	fi

	cp -R "$common_dir" "$package_dir"
	cp "$source_binary" "$package_dir/${BINARY}${extension}"
	chmod 0755 "$package_dir/${BINARY}${extension}"

	case "$archive_format" in
		tar.gz)
			archive="$release_dir/${package_name}.tar.gz"
			COPYFILE_DISABLE=1 tar -C "$work_dir" -czf "$archive" "$package_name"
			contents=$(tar -tzf "$archive")
			;;
		zip)
			archive="$release_dir/${package_name}.zip"
			(cd "$work_dir" && zip -q -X -r "$archive" "$package_name")
			contents=$(unzip -Z1 "$archive")
			;;
		*)
			echo "error: unsupported archive format: $archive_format" >&2
			exit 1
			;;
	esac

	for required in "/${BINARY}${extension}" /LICENSE /README.md /THIRD_PARTY_NOTICES.md /go-modules.txt /third_party_licenses/; do
		if ! printf '%s\n' "$contents" | grep -F "$package_name$required" >/dev/null; then
			echo "error: $archive is missing $required" >&2
			exit 1
		fi
	done

	rm -rf "$package_dir"
}

package_target linux amd64 "" tar.gz
package_target linux arm64 "" tar.gz
package_target darwin amd64 "" tar.gz
package_target darwin arm64 "" tar.gz
package_target windows amd64 .exe zip
package_target windows arm64 .exe zip

cp "$module_inventory" "$release_dir/go-modules.txt"

(
	cd "$release_dir"
	files=$(find . -maxdepth 1 -type f ! -name SHA256SUMS -print | sed 's#^./##' | sort)
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum $files > SHA256SUMS
	else
		shasum -a 256 $files > SHA256SUMS
	fi
)

echo "packaged $VERSION release archives in $DIST_DIR/release"
