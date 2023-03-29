#!/bin/bash

# This script downloads the latest version of JFrog CLI, unless a requested version is provided as an argument.
# By default JFrog CLI is downloaded from releases.jfrog.io.
# The script also supports downloading from a remote repository, by setting the following environment variables:
# 1. JF_RELEASES_REPO - name of the remote repository, pointing to "https://releases.jfrog.io/artifactory/"
# 2. JF_URL - URL of the JFrog Platform that includes the remote repository.
# 3. Credentials to the JFrog Platform:
#   a. JF_ACCESS_TOKEN
#   OR
#   b. JF_USER & JF_PASSWORD

CLI_OS="na"
CLI_UNAME="na"
CLI_MAJOR_VER="v2-jf"
VERSION="[RELEASE]"

if [ $# -eq 1 ]
then
    VERSION=$1
    echo "Downloading version $VERSION of JFrog CLI..."
else
    echo "Downloading the latest version of JFrog CLI..."
fi

# Download from remote repository if required.
if [ -z "${JF_RELEASES_REPO}" ]
then
    BASE_URL="https://releases.jfrog.io/artifactory"
    AUTH_HEADER=""
    echo "Downloading directly from releases.jfrog.io..."
else
    # Make sure URL does not contain duplicate separators.
    BASE_URL="${JF_URL%/}/artifactory/${JF_RELEASES_REPO%/}"
    if [ -n "${JF_ACCESS_TOKEN}" ]; then
        AUTH_HEADER="Authorization: Bearer ${JF_ACCESS_TOKEN}"
    else
        AUTH_HEADER="Authorization: Basic $(printf ${JF_USER}:${JF_PASSWORD} | base64)"
    fi
    echo "Downloading from remote repository '${JF_RELEASES_REPO}'..."
fi
echo ""

# Build the URL to the executable matching the OS.
if $(echo "${OSTYPE}" | grep -q msys); then
    CLI_OS="windows"
    URL="${BASE_URL}/jfrog-cli/${CLI_MAJOR_VER}/${VERSION}/jfrog-cli-windows-amd64/jf.exe"
    FILE_NAME="jf.exe"
elif $(echo "${OSTYPE}" | grep -q darwin); then
    CLI_OS="mac"
    if [[ $(uname -m) == 'arm64' ]]; then
      URL="${BASE_URL}/jfrog-cli/${CLI_MAJOR_VER}/${VERSION}/jfrog-cli-mac-arm64/jf"
    else
      URL="${BASE_URL}/jfrog-cli/${CLI_MAJOR_VER}/${VERSION}/jfrog-cli-mac-386/jf"
    fi
    FILE_NAME="jf"
else
    CLI_OS="linux"
    MACHINE_TYPE="$(uname -m)"
    case $MACHINE_TYPE in
        i386 | i486 | i586 | i686 | i786 | x86)
            ARCH="386"
            ;;
        amd64 | x86_64 | x64)
            ARCH="amd64"
            ;;
        arm | armv7l)
            ARCH="arm"
            ;;
        aarch64)
            ARCH="arm64"
            ;;
        s390x)
            ARCH="s390x"
            ;;
        ppc64)
           ARCH="ppc64"
           ;;
        ppc64le)
           ARCH="ppc64le"
           ;;
        *)
            echo "Unknown machine type: $MACHINE_TYPE"
            exit -1
            ;;
    esac
    URL="${BASE_URL}/jfrog-cli/${CLI_MAJOR_VER}/${VERSION}/jfrog-cli-${CLI_OS}-${ARCH}/jf"
    FILE_NAME="jf"
fi

# Download with curl and verify exit code and status code.
status_code=$(curl -XGET "$URL" -L -k -g -w "%{http_code}" -f -O -H "${AUTH_HEADER}")
exitCode=$?
if [ $exitCode -ne 0 ]; then
  echo "Error: Failed downloading JFrog CLI"
  exit $exitCode
fi

if [ $status_code != "200" ]; then
  echo "Error: Received unexpected HTTP status code: $status_code"
  exit 1
fi
chmod u+x $FILE_NAME
