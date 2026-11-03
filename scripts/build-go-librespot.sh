#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION="v0.10.6-0.20261102145552-522adcae8d65"
MODULE="github.com/devgianlu/go-librespot"
MODULE_DIR="$(go env GOPATH)/pkg/mod/${MODULE}@${VERSION}"
BUILD_DIR="${ROOT_DIR}/build/audio/go-librespot-src"
PATCH_FILE="${ROOT_DIR}/build/patches/go-librespot-search-radio.patch"
OUTPUT="${ROOT_DIR}/bin/go-librespot"

if [ ! -d "${MODULE_DIR}" ]; then
	go mod download "${MODULE}@${VERSION}"
fi

rm -rf "${BUILD_DIR}"
mkdir -p "$(dirname "${BUILD_DIR}")" "${ROOT_DIR}/bin"
cp -a "${MODULE_DIR}" "${BUILD_DIR}"
chmod -R u+w "${BUILD_DIR}"

if ! patch -p1 --batch --forward -d "${BUILD_DIR}" < "${PATCH_FILE}"; then
	echo "failed to apply the go-librespot search/radio patch" >&2
	exit 1
fi

(
	cd "${BUILD_DIR}"
	go build -trimpath -o "${OUTPUT}" ./cmd/daemon
)

chmod +x "${OUTPUT}"
