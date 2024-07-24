#!/usr/bin/env bash

set -o errexit
set -o nounset

__usage() {
  cat <<EOF
USAGE:

${0} [SUBPROJECT_PATH]

EOF
  exit 1
}

[ -z "${1}" ] && __usage

SUBPATH="${1}"

echo "./${SUBPATH}"/* | xargs -n1 | sed "s@^\./${SUBPATH}/@@"
