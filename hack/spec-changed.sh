#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

# usage: spec-changed.sh PINNED CANDIDATE
set -euo pipefail

old_version=$(jq -r '.info.version' "$1")
new_version=$(jq -r '.info.version' "$2")
if cmp -s "$1" "$2"; then changed=false; else changed=true; fi
echo "changed=$changed"
echo "old_version=$old_version"
echo "new_version=$new_version"
