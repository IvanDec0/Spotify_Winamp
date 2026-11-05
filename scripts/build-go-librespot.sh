#!/usr/bin/env bash
set -euo pipefail

CHECK_ONLY=false
for ARG in "$@"; do
	case "${ARG}" in
		--check) CHECK_ONLY=true ;;
		*)
			echo "unknown argument: ${ARG}" >&2
			echo "usage: $0 [--check]" >&2
			exit 2
			;;
	esac
done

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION="v0.10.6-0.20261104085532-16b65bb4cd2b"
MODULE="github.com/devgianlu/go-librespot"
MODULE_DIR="$(go env GOPATH)/pkg/mod/${MODULE}@${VERSION}"
BUILD_DIR="${ROOT_DIR}/build/audio/go-librespot-src"
OUTPUT="${ROOT_DIR}/bin/go-librespot"

if [ ! -d "${MODULE_DIR}" ]; then
	go mod download "${MODULE}@${VERSION}"
fi

rm -rf "${BUILD_DIR}"
mkdir -p "$(dirname "${BUILD_DIR}")" "${ROOT_DIR}/bin"
cp -a "${MODULE_DIR}" "${BUILD_DIR}"
chmod -R u+w "${BUILD_DIR}"

for PATCH_FILE in \
	"${ROOT_DIR}/build/patches/go-librespot-search-radio.patch" \
	"${ROOT_DIR}/build/patches/go-librespot-search-groups.patch" \
	"${ROOT_DIR}/build/patches/go-librespot-library-collection.patch" \
	"${ROOT_DIR}/build/patches/go-librespot-http-proxy.patch"; do
	if ! patch -p1 --batch --forward --fuzz=0 -d "${BUILD_DIR}" < "${PATCH_FILE}"; then
		echo "failed to apply $(basename "${PATCH_FILE}")" >&2
		exit 1
	fi
done

if [ "${CHECK_ONLY}" = true ]; then
	echo "go-librespot patches apply cleanly to ${VERSION}; skipping build (--check)"
	exit 0
fi

(
	cd "${BUILD_DIR}"
	# The source is copied inside the Cadence repository, so VCS stamping would
	# report Cadence's revision as the engine version. Inject the real pin and
	# disable VCS stamping.
	go build -trimpath -buildvcs=false \
		-ldflags "-s -w -X github.com/devgianlu/go-librespot.version=${VERSION#v}" \
		-o "${OUTPUT}" ./cmd/daemon
)

chmod +x "${OUTPUT}"
