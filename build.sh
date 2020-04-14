#!/bin/bash
# Usage: ./build.sh -o /tmp/cdap_provider -v v1.0.0

set -e

OUTPUT_DIR="."
VERSION=""

while getopts 'o:v:' c
do
  case $c in
    o) OUTPUT_DIR=${OPTARG} ;;
    v) VERSION=${OPTARG} ;;
    *)
      echo "Invalid flag ${OPTARG}"
      exit 1
      ;;
  esac
done

if [[ -z ${VERSION} ]]; then
    echo "-v must be set"
    exit 1
fi

SUPPORTED_OS=("linux" "darwin" "windows")
ARCH="amd64"

for OS in "${SUPPORTED_OS[@]}"
do
    output_name="terraform-provider-cdap_${VERSION}_${OS}-${ARCH}"
    echo "Building ${output_name}"
    env GOOS="${OS}" GOARCH="${ARCH}" go build -o "${OUTPUT_DIR}/${output_name}"
done
