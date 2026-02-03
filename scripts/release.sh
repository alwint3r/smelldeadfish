#!/usr/bin/env bash
set -euo pipefail

otlp_server_bin_name=smelldeadfish-otlp-server

usage() {
  cat <<'EOF'
Usage: scripts/release.sh --version vX.Y.Z

Stages release artifacts for the provided version.
This script does not run git or gh commands.
EOF
}

fatal() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fatal "missing required command: $1"
}

version=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      if [[ $# -lt 2 ]]; then
        fatal "missing value for --version"
      fi
      version="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fatal "unknown argument: $1"
      ;;
  esac
done

if [[ -z "$version" ]]; then
  usage
  fatal "--version is required"
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  fatal "version must match vX.Y.Z (got: $version)"
fi

require_cmd task
require_cmd shasum
require_cmd npm

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
cd "$repo_root"

echo "Staging release artifacts for $version"

task dist:embed
echo "Built embedded binaries with task dist:embed"

release_dir="dist/release/$version"
rm -rf "$release_dir"
mkdir -p "$release_dir"

cp "dist/prod-embed/bin/darwin_arm64/$otlp_server_bin_name" "$release_dir/${otlp_server_bin_name}_darwin_arm64"
cp "dist/prod-embed/bin/windows_amd64/$otlp_server_bin_name.exe" "$release_dir/${otlp_server_bin_name}_windows_amd64.exe"

(cd "$release_dir" && shasum -a 256 ${otlp_server_bin_name}_darwin_arm64 ${otlp_server_bin_name}_windows_amd64.exe > checksums.txt)
echo "Staged assets in $release_dir"

rm -rf internal/uiembed/dist

cat <<EOF
Next steps (run manually):
  Ensure tag $version exists on GitHub.
  gh release create $version --title "Smelldeadfish $version" --generate-notes
  gh release upload $version "$release_dir"/* --clobber
  gh release view $version --json url --jq .url
EOF
