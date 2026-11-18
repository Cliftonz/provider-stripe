# Decisions

| Date | Decision |
|------|----------|
| 2026-11-16 | Native crossplane-runtime v2 provider, not a Terraform wrapper. |
| 2026-11-16 | Scope is all creatable top-level Stripe resources from pinned spec tag v2600 (API version `2026-09-30.endive`). |
| 2026-11-16 | stripe-go is used only for transport: form encoding, retries, API version, idempotency. Its typed structs are unused. |
| 2026-11-16 | `cmd/generator` emits CRD types and per-kind descriptors; one generic kit controller consumes them. |
| 2026-11-16 | Managed resources use API group `<namespace>.stripe.m.clifton.quest`, with `core` for top-level resources, at `v1alpha1`. |
| 2026-11-16 | Config kinds use `stripe.m.clifton.quest/v1beta1`. |
| 2026-11-16 | All resources are namespaced only. |
| 2026-11-16 | Delete modes, in order: delete endpoint; else `active=false`; else a lifecycle action (cancel, expire, void, deactivate, close) per a reviewed table; else no-op. Non-deleting modes report the resource absent once deletion starts. |
| 2026-11-16 | Create-only fields are immutable via CEL. |
| 2026-11-16 | `Idempotency-Key` on create is the managed resource UID. |
| 2026-11-16 | The managed resource owns all metadata keys on the Stripe object. |
| 2026-11-16 | Credentials Secret is JSON `{apiKey, stripeAccount?, baseURL?}`. |
| 2026-11-16 | With a live key (`sk_live_` or `rk_live_`), money-moving kinds refuse to reconcile unless ProviderConfig sets `allowLiveMoneyMovement=true`. Test keys are never blocked. |
| 2026-11-16 | Unions of `X \| ""` collapse to optional `X`; all other unions are free-form JSON. |
| 2026-11-16 | Cross-resource references come from a curated table. |
| 2026-11-16 | `secret` and `client_secret` fields appear only in the connection secret. |
| 2026-11-16 | Optional non-nullable fields are late-initialized. |
| 2026-11-16 | Weekly spec tracking to the latest tag, with a matching stripe-go bump, on a `spec/<tag>` PR. No auto-merge. |
| 2026-11-16 | E2E uses stripe-mock in kind (wiring only), plus an opt-in SaaS test-key mode. |
| 2026-11-16 | Never file issues or PRs upstream. |
| 2026-11-16 | Scope narrowed to 67 kinds: of the 69 creatable kinds with an item GET, Token (`/v1/tokens`, PCI) and File (`/v1/files`, multipart) are excluded for v0.1.0. |
| 2026-11-16 | Creatable endpoints without an item GET (tax settings, balance settings, sessions, and similar) are a v0.2 follow-up. |
| 2026-11-16 | API group names drop underscores: `billingportal`, `productcatalog`, `threedsecure`, `testhelpers`, `applepay`, `financialconnections`. |
| 2026-11-16 | `expand` is never a CRD field. |
| 2026-11-16 | Update-only fields are part of `forProvider` and are applied by the first Update after create. |
| 2026-11-16 | Delete overrides: Invoice deletes drafts and voids finalized ones; Issuing Card sets `status=canceled`; Issuing Cardholder sets `status=inactive`; Apps Install uses uninstall. |
| 2026-11-16 | The per-kind delete table is generated into `docs/delete-modes.md`. |
| 2026-11-16 | An empty `baseURL` means `https://api.stripe.com`. |
| 2026-11-16 | stripe-go default retries are kept. |
| 2026-11-16 | stripe-go logs go to the provider logger at debug level. |
| 2026-11-16 | Stripe telemetry is off by default, opt-in per ProviderConfig via `spec.enableTelemetry`. |
| 2026-11-16 | Observed numbers decode exactly, with no float rounding. |
| 2026-11-16 | The generator fails loudly on unexpected spec shapes. |
| 2026-11-16 | `status.atProvider` is typed at the top level (scalars, enums, expandable refs as ID strings); nested objects and arrays are free-form JSON. |
