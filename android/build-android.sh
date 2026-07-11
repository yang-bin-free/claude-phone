#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ -z "${JAVA_HOME:-}" || ! -x "${JAVA_HOME}/bin/java" ]]; then
  if [[ -x /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home/bin/java ]]; then
    export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
  elif [[ -x /usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home/bin/java ]]; then
    export JAVA_HOME=/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
  else
    echo "JDK 17 is required. Install openjdk@17 or set JAVA_HOME to a JDK 17+ home." >&2
    exit 1
  fi
fi

java_version="$("${JAVA_HOME}/bin/java" -version 2>&1 | awk -F '"' '/version/ {print $2; exit}')"
java_major="${java_version%%.*}"
if [[ "${java_major}" == "1" ]]; then
  java_major="$(echo "${java_version}" | awk -F '.' '{print $2}')"
fi
if [[ "${java_major}" -lt 17 ]]; then
  if [[ -x /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home/bin/java ]]; then
    export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
  elif [[ -x /usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home/bin/java ]]; then
    export JAVA_HOME=/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
  else
    echo "JDK 17 is required, but JAVA_HOME points to ${java_version}." >&2
    exit 1
  fi
fi

"../scripts/build-android-aar.sh"
exec ./gradlew "$@"
