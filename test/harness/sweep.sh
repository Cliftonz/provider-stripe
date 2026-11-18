#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

if ! [[ ${STRIPE_API_KEY:-} =~ ^(sk|rk)_test_ ]]; then
  echo "STRIPE_API_KEY must be a Stripe test-mode key (sk_test_ or rk_test_)" >&2
  exit 1
fi

api() {
  curl -fsS -u "$STRIPE_API_KEY:" "https://api.stripe.com$1" "${@:2}"
}

# shellcheck disable=SC2016
stale='
def ended:
  if $mode == "Deactivate" or $mode == "DeleteElseDeactivate" then .active == false
  elif $mode == "SetField" then .[$field] == $value
  elif $mode == "Action" or $mode == "DeleteDraftElseAction" then
    (.status | tostring) as $status | any($statuses | split(",")[]; . == $status)
    or any(.[$timestamps | split(",")[]]; . != null and . <= now)
  else false end;
.data[] | select(.metadata["crossplane-e2e"] == "true" and .created < now - 7200 and (ended | not)) | "\(.id)\t\(.status)"'

sweep_kind() {
  local path=$1 mode=$2 action=$3 field=$4 value=$5 statuses=$6 timestamps=$7 query=limit=100 page pages='' id status code
  while :; do
    page="$(api "$path?$query")"
    pages+=$page
    [ "$(jq .has_more <<< "$page")" = true ] || break
    query="limit=100&starting_after=$(jq -r '.data[-1].id' <<< "$page")"
  done
  jq -r --arg mode "$mode" --arg field "$field" --arg value "$value" --arg statuses "$statuses" --arg timestamps "$timestamps" "$stale" <<< "$pages" \
    | while IFS=$'\t' read -r id status; do
      echo "e2e-sweep $mode $path/$id"
      case $mode in
        Delete) api "$path/$id" -X DELETE ;;
        Deactivate) api "$path/$id" -d active=false ;;
        Action) api "$path/$id/$action" -X POST ;;
        SetField) api "$path/$id" -d "$field=$value" ;;
        DeleteDraftElseAction)
          if [ "$status" = draft ]; then api "$path/$id" -X DELETE; else api "$path/$id/$action" -X POST; fi ;;
        DeleteElseDeactivate)
          code="$(curl -sS -o /dev/null -w '%{http_code}' -u "$STRIPE_API_KEY:" "https://api.stripe.com$path/$id" -X DELETE)"
          case $code in
            2?? | 404) ;;
            *) api "$path/$id" -d active=false ;;
          esac ;;
      esac > /dev/null
    done
}

awk '
  { sub(/\r$/, "") }
  (FNR == 1 || /^---/) && group != "" && kind != "" { print group "|" kind }
  FNR == 1 || /^---/ { group = kind = "" }
  /^apiVersion: [a-z]+\.stripe\.m\.clifton\.quest\// { group = $2; sub(/\..*/, "", group) }
  /^kind: / { kind = $2 }
  END { if (group != "" && kind != "") print group "|" kind }
' "$@" | sort -u \
  | awk -F' *[|] *' '
    { sub(/\r$/, "") }
    NR == FNR { created[$0]; next }
    /^\| [A-Z]/ && ($4 "|" $2) in created {
      delete created[$4 "|" $2]
      gsub(/, /, ",", $9); gsub(/, /, ",", $10)
      if ($5 != "NoOp") rows[++n] = $3 "|" $5 "|" $6 "|" $7 "|" $8 "|" $9 "|" $10
    }
    END {
      for (key in created) { sub(/\|/, "/", key); print "no docs/delete-modes.md row for " key > "/dev/stderr"; failed = 1 }
      if (failed) exit 1
      for (i = 1; i <= n; i++) print rows[i]
    }
  ' - "$(dirname "$0")/../../docs/delete-modes.md" \
  | while IFS='|' read -r path mode action field value statuses timestamps; do
    sweep_kind "$path" "$mode" "$action" "$field" "$value" "$statuses" "$timestamps"
  done
