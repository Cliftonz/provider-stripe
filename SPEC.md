# provider-stripe

## What it does

provider-stripe lets a Kubernetes cluster running Crossplane v2 declare Stripe objects as namespaced managed resources and keep them reconciled against a Stripe account. It covers 67 kinds from pinned spec tag v2600, API version `2026-09-30.endive`: every creatable top-level Stripe resource that has an item GET, except Token (`/v1/tokens`, PCI scope) and File (`/v1/files`, multipart upload), which are excluded from v0.1.0. Creatable endpoints without an item GET (such as tax settings, balance settings, and sessions) are deferred to v0.2.

## Why it is shaped this way

### Native provider, generated surface, one controller

The provider is built directly on crossplane-runtime v2 rather than wrapping Terraform. Stripe's API is large, regular, and fully described by its OpenAPI spec, so hand-writing 67 controllers would be slow and inconsistent. Instead, `cmd/generator` reads the pinned spec and emits the CRD types and a descriptor for each kind, and a single generic kit controller consumes those descriptors. Adding or changing a kind is a generator run, not new controller code. The generator fails loudly on any spec shape it does not expect, so a Stripe change can never silently produce a wrong CRD.

### stripe-go for transport only

stripe-go is pinned to the release whose API version matches the spec tag. It supplies form encoding, retries (its defaults are kept), the `Stripe-Version` header, and idempotency handling. Its typed structs are not used; the generated CRD types are the single source of shape, so the provider never has to reconcile two models of the same object. stripe-go logs go to the provider logger at debug level. Stripe telemetry is off by default and can be enabled per ProviderConfig with `spec.enableTelemetry`.

### API groups and versions

Managed resources live in one group per Stripe namespace, `<namespace>.stripe.m.clifton.quest`, with top-level Stripe resources in `core.stripe.m.clifton.quest`. Group names drop underscores, so Stripe's `billing_portal` becomes `billingportal` (likewise `productcatalog`, `threedsecure`, `testhelpers`, `applepay`, `financialconnections`). Managed resources are served at `v1alpha1`. Configuration kinds live in `stripe.m.clifton.quest/v1beta1`. Everything is namespaced; there are no cluster-scoped managed resources.

### Credentials

A ProviderConfig points at a Secret whose JSON body is `{"apiKey": ..., "stripeAccount": ..., "baseURL": ...}`; only `apiKey` is required. `stripeAccount` targets a connected account. An empty `baseURL` means `https://api.stripe.com`; setting it lets tests aim at stripe-mock.

### Money safety

With a live key (`sk_live_` or `rk_live_`), money-moving kinds refuse to reconcile unless the ProviderConfig sets `allowLiveMoneyMovement: true`. Test keys are never blocked. This makes the dangerous case an explicit, reviewable opt-in.

### Create, update, delete semantics

- Create sends `Idempotency-Key` equal to the managed resource UID, so a retried create after a lost response cannot duplicate the object.
- Fields Stripe accepts only on create are immutable in the CRD via CEL, so the API server rejects edits the provider could never apply.
- Fields Stripe accepts only on update are still part of `forProvider`; the first update after create applies them, so users declare one desired state regardless of which call Stripe wants it on.
- The managed resource owns every metadata key on the Stripe object; keys absent from the spec are removed.
- Optional fields Stripe defaults and never returns as null are late-initialized from the observed object.
- Deletion follows a per-kind table, generated into `docs/delete-modes.md`. The default order of preference is: the delete endpoint; else setting `active=false`; else a lifecycle action (cancel, expire, void, deactivate, close); else a no-op. Reviewed overrides: Invoice deletes drafts and voids finalized invoices; Issuing Card sets `status=canceled`; Issuing Cardholder sets `status=inactive`; Apps Install uninstalls. For kinds that do not truly delete, the provider reports the resource absent once deletion starts so the finalizer can clear.

### Schema mapping

- `expand` is never a CRD field; the provider decides what it reads.
- A union of `X | ""` collapses to an optional `X`.
- Every other union is free-form JSON, since CRD schemas cannot express it faithfully.
- `status.atProvider` is typed at the top level (scalars, enums, and expandable references as ID strings); nested objects and arrays are free-form JSON. This keeps the commonly inspected fields typed without generating hundreds of nested status types.
- Observed numbers decode exactly, with no float rounding, so amounts and counts never drift.
- Cross-resource references (for example a Price's product) come from a curated table rather than heuristics, so only references that are known correct get `Ref` and `Selector` fields.
- Secret-bearing fields (`secret`, `client_secret`) are never written to status; they appear only in the connection secret.

### Tracking Stripe

A weekly job checks for the latest Stripe spec tag. When one appears it bumps stripe-go to the matching release, regenerates, and opens a `spec/<tag>` pull request. Nothing auto-merges; a human reviews every spec change.

### Testing

End-to-end tests run in kind against stripe-mock, which verifies wiring (requests are well-formed and the reconcile loop completes) but not real Stripe behavior. An opt-in mode runs the same suite against real Stripe with a test-mode key.

### Boundaries

This project never files issues or pull requests against Stripe, Crossplane, or any other upstream project.
