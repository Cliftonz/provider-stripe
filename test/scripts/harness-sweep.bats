#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  cd "$BATS_TEST_TMPDIR"
  mkdir -p bin lists examples test/harness docs
  cp "$ROOT/test/harness/sweep.sh" test/harness/
  cp "$ROOT/docs/delete-modes.md" docs/
  export STRIPE_API_KEY=sk_test_abc
  OLD=$(($(date +%s) - 10800))
  RECENT=$(($(date +%s) - 3600))
  curl_stub 'exit 0'
}

curl_stub() {
  cat > bin/curl <<STUB
#!/usr/bin/env bash
echo "\$*" >> $PWD/curl.log
for arg; do case \$arg in https://*) url=\$arg ;; esac; done
path=\${url#https://api.stripe.com}
case \$path in
  *'?limit=100'*)
    after=\${path#*starting_after=}
    [ "\$after" = "\$path" ] && after=
    f="$PWD/lists/\$(echo "\${path%%\?*}" | tr / _)\${after:+@\$after}.json"
    if [ -f "\$f" ]; then cat "\$f"; else echo '{"data":[],"has_more":false}'; fi
    exit ;;
esac
$1
STUB
  chmod +x bin/curl
}

example() {
  printf '%s\n' "$@" > "examples/$RANDOM.yaml"
}

list() {
  jq -c --argjson more "${2:-false}" '{data: ., has_more: $more}' > "lists/$(echo "$1" | tr / _).json"
}

tagged() {
  jq -nc --arg id "$1" --argjson created "$2" --argjson extra "${3:-{\}}" '{id: $id, created: $created, metadata: {"crossplane-e2e": "true"}} + $extra'
}

sweep() {
  PATH="$PWD/bin:$PATH" run test/harness/sweep.sh examples/*.yaml
}

writes() {
  grep -v -- '?limit=100' curl.log || true
}

@test "deletes tagged objects older than 2 hours of a Delete kind and leaves recent or untagged ones" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Customer'
  {
    tagged cus_old "$OLD"
    tagged cus_recent "$RECENT"
    jq -nc --argjson created "$OLD" '{id: "cus_untagged", created: $created, metadata: {}}'
    jq -nc --argjson created "$OLD" '{id: "cus_false", created: $created, metadata: {"crossplane-e2e": "false"}}'
  } | jq -s . | list /v1/customers
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/customers/cus_old -X DELETE' ]
}

@test "lists every page, following the last fetched id, before cleaning up" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Customer'
  { tagged cus_a "$RECENT"; tagged cus_b "$OLD"; } | jq -s . | list /v1/customers true
  tagged cus_c "$OLD" | jq -s . | list /v1/customers@cus_b true
  tagged cus_d "$OLD" | jq -s . | list /v1/customers@cus_c
  sweep
  [ "$status" -eq 0 ]
  grep -qxF -- '-fsS -u sk_test_abc: https://api.stripe.com/v1/customers?limit=100&starting_after=cus_c' curl.log
  [ "$(grep -n -- "-X DELETE" curl.log | head -1 | cut -d: -f1)" -gt "$(grep -n -- "?limit=100" curl.log | tail -1 | cut -d: -f1)" ]
  [ "$(writes)" = "$(printf -- '-fsS -u sk_test_abc: https://api.stripe.com/v1/customers/%s -X DELETE\n' cus_b cus_c cus_d)" ]
}

@test "deactivates tagged objects of a Deactivate kind unless already inactive" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Price'
  { tagged price_on "$OLD" '{"active":true}'; tagged price_off "$OLD" '{"active":false}'; } | jq -s . | list /v1/prices
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/prices/price_on -d active=false' ]
}

@test "runs the action of an Action kind unless the object is in an end status" {
  example 'kind: PaymentIntent' 'apiVersion: core.stripe.m.clifton.quest/v1alpha1'
  { tagged pi_open "$OLD" '{"status":"requires_payment_method"}'; tagged pi_done "$OLD" '{"status":"succeeded"}'; } | jq -s . | list /v1/payment_intents
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/payment_intents/pi_open/cancel -X POST' ]
}

@test "skips an Action object whose end timestamp has passed" {
  example 'apiVersion: billing.stripe.m.clifton.quest/v1alpha1' 'kind: CreditGrant'
  {
    tagged cg_expired "$OLD" "{\"expires_at\":$RECENT}"
    tagged cg_live "$OLD" "{\"expires_at\":$(($(date +%s) + 3600)),\"voided_at\":null}"
  } | jq -s . | list /v1/billing/credit_grants
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/billing/credit_grants/cg_live/expire -X POST' ]
}

@test "sets the field of a SetField kind unless it already holds the value" {
  example 'apiVersion: issuing.stripe.m.clifton.quest/v1alpha1' 'kind: Card'
  { tagged ic_active "$OLD" '{"status":"active"}'; tagged ic_gone "$OLD" '{"status":"canceled"}'; } | jq -s . | list /v1/issuing/cards
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/issuing/cards/ic_active -d status=canceled' ]
}

@test "deletes draft invoices, voids open ones and skips ended ones" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Invoice'
  {
    tagged in_draft "$OLD" '{"status":"draft"}'
    tagged in_open "$OLD" '{"status":"open"}'
    tagged in_paid "$OLD" '{"status":"paid"}'
  } | jq -s . | list /v1/invoices
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = "$(printf '%s\n' '-fsS -u sk_test_abc: https://api.stripe.com/v1/invoices/in_draft -X DELETE' '-fsS -u sk_test_abc: https://api.stripe.com/v1/invoices/in_open/void -X POST')" ]
}

@test "lists exactly the Stripe kinds the examples create, by group and kind, except NoOp kinds" {
  example 'apiVersion: v1' 'kind: Secret' '---' 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product' 'spec:' '  kind: Price'
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product' '---' 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Price'
  example 'apiVersion: checkout.stripe.m.clifton.quest/v1alpha1' 'kind: Session'
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Charge'
  example 'apiVersion: stripe.m.clifton.quest/v1beta1' 'kind: ProviderConfig'
  sweep
  [ "$status" -eq 0 ]
  [ "$(sed 's/.*api.stripe.com//' curl.log | sort)" = "$(printf '%s\n' '/v1/checkout/sessions?limit=100' '/v1/prices?limit=100' '/v1/products?limit=100')" ]
}

@test "fails when a cleanup call fails" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Customer'
  tagged cus_old "$OLD" | jq -s . | list /v1/customers
  curl_stub 'exit 22'
  sweep
  [ "$status" -ne 0 ]
}

@test "fails when a list call fails" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product'
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Customer'
  printf '#!/usr/bin/env bash\necho "$*" >> %s/curl.log\nexit 22\n' "$PWD" > bin/curl
  sweep
  [ "$status" -ne 0 ]
  [ "$(wc -l < curl.log)" -eq 1 ]
}

@test "refuses anything but a test-mode key before calling the API" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product'
  for key in sk_live_abc rk_live_abc pk_test_abc ""; do
    STRIPE_API_KEY=$key sweep
    [ "$status" -ne 0 ]
  done
  unset STRIPE_API_KEY
  sweep
  [ "$status" -ne 0 ]
  [ ! -e curl.log ]
}

@test "accepts a restricted test-mode key" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product'
  STRIPE_API_KEY=rk_test_abc sweep
  [ "$status" -eq 0 ]
  grep -q -- '-u rk_test_abc:' curl.log
}

product() {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product'
  { tagged prod_on "$OLD" '{"active":true}'; tagged prod_off "$OLD" '{"active":false}'; } | jq -s . | list /v1/products
  curl_stub "case \"\$*\" in *DELETE*) printf $1 ;; *active=false*) exit ${2:-0} ;; esac"
  sweep
}

@test "deletes an object of a DeleteElseDeactivate kind and skips one already inactive" {
  product 200
  [ "$status" -eq 0 ]
  [ "$(writes)" = "-sS -o /dev/null -w %{http_code} -u sk_test_abc: https://api.stripe.com/v1/products/prod_on -X DELETE" ]
}

@test "deactivates an object of a DeleteElseDeactivate kind when Stripe rejects the delete" {
  product 400
  [ "$status" -eq 0 ]
  [ "$(writes)" = "$(printf '%s\n' '-sS -o /dev/null -w %{http_code} -u sk_test_abc: https://api.stripe.com/v1/products/prod_on -X DELETE' '-fsS -u sk_test_abc: https://api.stripe.com/v1/products/prod_on -d active=false')" ]
}

@test "treats a 404 on delete of a DeleteElseDeactivate kind as gone" {
  product 404
  [ "$status" -eq 0 ]
  grep -q -- "prod_on -X DELETE" curl.log
  ! grep -q active=false curl.log
}

@test "fails when the deactivate after a rejected delete fails" {
  product 400 22
  [ "$status" -ne 0 ]
}

@test "fails before calling the API when a created kind has no delete-modes row, naming it" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Customer'
  example 'apiVersion: billing.stripe.m.clifton.quest/v1alpha1' 'kind: Nonexistent'
  sweep
  [ "$status" -ne 0 ]
  [[ $output == *billing/Nonexistent* ]]
  [ ! -e curl.log ]
}

@test "reads examples and the delete-modes table with CRLF line endings" {
  printf 'apiVersion: core.stripe.m.clifton.quest/v1alpha1\r\nkind: Customer\r\n' > examples/crlf.yaml
  sed -i 's/$/\r/' docs/delete-modes.md
  tagged cus_old "$OLD" | jq -s . | list /v1/customers
  sweep
  [ "$status" -eq 0 ]
  [ "$(writes)" = '-fsS -u sk_test_abc: https://api.stripe.com/v1/customers/cus_old -X DELETE' ]
}

@test "fails without deactivating when the delete of a DeleteElseDeactivate kind cannot reach Stripe" {
  example 'apiVersion: core.stripe.m.clifton.quest/v1alpha1' 'kind: Product'
  tagged prod_on "$OLD" '{"active":true}' | jq -s . | list /v1/products
  curl_stub 'case "$*" in *DELETE*) printf 000; exit 7 ;; esac'
  sweep
  [ "$status" -ne 0 ]
  ! grep -q active=false curl.log
}
