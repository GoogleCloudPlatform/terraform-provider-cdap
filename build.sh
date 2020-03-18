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

platforms=("linux/amd64" "darwin/amd64" "windows/amd64")

for platform in "${platforms[@]}"
do
    IFS="/"
    read -ra platform_split <<< "${platform}"
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}

    output_name="terraform-cdap-provider_${VERSION}_${GOOS}-${GOARCH}"
    echo "Building ${output_name}"

    env GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${OUTPUT_DIR}"/"${output_name}"
done
