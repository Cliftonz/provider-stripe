<!--
SPDX-FileCopyrightText: 2026 The provider-stripe authors

SPDX-License-Identifier: Apache-2.0
-->

# Decisions

- The default target runs `stripe/stripe-mock`, pinned by digest in `images.env`, as a Deployment and Service in the `stripe-mock` namespace of the kind cluster. The provider reaches it at `http://stripe-mock.stripe-mock:12111` with the placeholder key `sk_test_123`, which `up.sh` writes to `.out/credentials.json` as the ProviderConfig secret body.
- stripe-mock is stateless: it answers every read with a fixture and never forgets or changes an object. So against it uptest asserts Ready only, with update, import and delete skipped; it checks wiring, not Stripe behavior.
- `E2E_TARGET=saas` runs the full uptest lifecycle against real Stripe with `STRIPE_API_KEY`. In CI the key comes from the `stripe-test` GitHub environment and is passed only to SaaS runs. `up.sh` and `sweep.sh` refuse any key that does not start with `sk_test_` or `rk_test_`, before creating the cluster or calling the API.
- Before a SaaS run, `sweep.sh` cleans up objects leaked by earlier runs: objects carrying metadata `crossplane-e2e: "true"` and created more than 2 hours ago, so concurrent runs do not sweep each other. It covers exactly the kinds the examples create, matched by group and kind against `docs/delete-modes.md`, and cleans each with that kind's delete mode; NoOp kinds and kinds without metadata are never swept. Objects already in their end state, by the same rule as the provider's deletion, are skipped. Every page of a list is fetched before any cleanup, so a cleaned-up object is never a paging cursor. A created kind with no row in the table, a failed list or cleanup call, or a delete that cannot reach Stripe fails the run. A `DeleteElseDeactivate` kind (Product) is deleted; when Stripe rejects the delete with anything but 404, it is set `active=false` instead, and a 404 counts as gone.
- Examples annotated `stripe.m.clifton.quest/e2e-saas: skip` create kinds a plain Stripe test account cannot. In SaaS mode `e2e.sh` leaves them out of the list it passes to the sweep and to uptest; the stripe-mock run still applies every example.
- `e2e.sh` runs uptest once per example file under `examples/namespaced/*/v1alpha1/`.
- `E2E_PACKAGE` makes `e2e.sh` install a published package with `runtimeconfig.yaml` instead of building and deploying a local xpkg; `make e2e-release VERSION=...` sets it.
