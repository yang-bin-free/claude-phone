#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -z "${ANDROID_HOME:-}" && -f android/local.properties ]]; then
  sdk_dir="$(awk -F '=' '/^sdk.dir=/ {print $2; exit}' android/local.properties)"
  if [[ -n "${sdk_dir}" ]]; then
    export ANDROID_HOME="${sdk_dir}"
  fi
fi

if [[ -z "${ANDROID_HOME:-}" ]]; then
  echo "ANDROID_HOME is required. Set it or create android/local.properties with sdk.dir=..." >&2
  exit 1
fi

if ! command -v gomobile >/dev/null 2>&1; then
  echo "gomobile is required. Install it with: go install golang.org/x/mobile/cmd/gomobile@latest" >&2
  exit 1
fi

mkdir -p build
if [[ -f build/claudelib.aar && "${REBUILD_AAR:-0}" != "1" ]]; then
  echo "Using existing build/claudelib.aar. Set REBUILD_AAR=1 to regenerate it."
  exit 0
fi

module_path="$(awk '/^module / {print $2; exit}' go.mod)"
go_version="$(awk '/^go / {print $2; exit}' go.mod)"
real_go="$(command -v go)"
wrapper_dir="$(mktemp -d)"
trap 'rm -rf "${wrapper_dir}"' EXIT

# gomobile workaround
# -------------------
# x/mobile's `gomobile bind` currently creates per-ABI temporary build
# directories such as:
#
#   $TMPDIR/gomobile-work-*/src-android-arm64/
#
# In each directory it writes a zero-byte `go.mod`, copies generated gobind
# sources under `gobind/`, and then runs `go mod tidy` before compiling the
# shared library. Go 1.26 rejects a zero-byte `go.mod` with:
#
#   go: error reading go.mod: missing module declaration
#
# The temporary directories are created by gomobile at runtime, so there is no
# stable file in this repo that can be pre-populated. Instead, this script puts a
# narrow `go` wrapper at the front of PATH only for this gomobile invocation. The
# wrapper patches exactly one case before delegating to the real Go binary:
#
#   - command is `go mod tidy`
#   - current directory contains a zero-byte `go.mod`
#   - current directory contains gomobile-generated `gobind/`
#
# It writes a valid temporary module and uses `replace` to point imports back at
# this repository. Normal `go build`, `go test`, and any non-gomobile command
# bypass this wrapper entirely. Remove this once x/mobile creates a valid
# temporary go.mod itself or the project switches to a patched gomobile binary.
cat >"${wrapper_dir}/go" <<EOF
#!/usr/bin/env bash
set -euo pipefail

if [[ "\${1:-}" == "mod" && "\${2:-}" == "tidy" && -f go.mod && ! -s go.mod && -d gobind ]]; then
  cat > go.mod <<GOMOD
module gomobilebind

go ${go_version}

require ${module_path} v0.0.0

replace ${module_path} => $(pwd)
GOMOD
fi

exec "${real_go}" "\$@"
EOF
chmod +x "${wrapper_dir}/go"

export PATH="${wrapper_dir}:${PATH}"
gomobile bind \
  -target=android \
  -androidapi=26 \
  -o build/claudelib.aar \
  "${module_path}/pkg/androidlib" \
  "${module_path}/pkg/androidlib/tailscale"

"$(dirname "$0")/test-android-aar.sh" build/claudelib.aar
