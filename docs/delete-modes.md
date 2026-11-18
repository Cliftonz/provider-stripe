| kind | path | group | delete mode | action | field | value | end statuses | end timestamps |
|------|------|-------|-------------|--------|-------|-------|--------------|----------------|
| Account | /v1/accounts | core | Delete |  |  |  |  |  |
| Domain | /v1/apple_pay/domains | applepay | Delete |  |  |  |  |  |
| Install | /v1/apps/installs | apps | Action | uninstall |  |  | install_failed, uninstalling |  |
| Alert | /v1/billing/alerts | billing | Action | deactivate |  |  | archived, inactive |  |
| CreditGrant | /v1/billing/credit_grants | billing | Action | expire |  |  |  | voided_at, expires_at |
| FeedbackOption | /v1/billing/feedback_options | billing | Action | deactivate |  |  | inactive |  |
| Meter | /v1/billing/meters | billing | Action | deactivate |  |  | inactive |  |
| Configuration | /v1/billing_portal/configurations | billingportal | Deactivate |  |  |  |  |  |
| Charge | /v1/charges | core | NoOp |  |  |  |  |  |
| Session | /v1/checkout/sessions | checkout | Action | expire |  |  | complete, expired |  |
| Order | /v1/climate/orders | climate | Action | cancel |  |  | canceled, delivered |  |
| Coupon | /v1/coupons | core | Delete |  |  |  |  |  |
| CreditNote | /v1/credit_notes | core | Action | void |  |  | void |  |
| Customer | /v1/customers | core | Delete |  |  |  |  |  |
| Feature | /v1/entitlements/features | entitlements | Deactivate |  |  |  |  |  |
| FileLink | /v1/file_links | core | NoOp |  |  |  |  |  |
| Session | /v1/financial_connections/sessions | financialconnections | NoOp |  |  |  |  |  |
| Request | /v1/forwarding/requests | forwarding | NoOp |  |  |  |  |  |
| VerificationSession | /v1/identity/verification_sessions | identity | Action | cancel |  |  | canceled, verified |  |
| Invoiceitem | /v1/invoiceitems | core | Delete |  |  |  |  |  |
| Invoice | /v1/invoices | core | DeleteDraftElseAction | void |  |  | paid, void |  |
| Cardholder | /v1/issuing/cardholders | issuing | SetField |  | status | inactive |  |  |
| Card | /v1/issuing/cards | issuing | SetField |  | status | canceled |  |  |
| Dispute | /v1/issuing/disputes | issuing | NoOp |  |  |  |  |  |
| PersonalizationDesign | /v1/issuing/personalization_designs | issuing | NoOp |  |  |  |  |  |
| LinkAccountSession | /v1/link_account_sessions | core | NoOp |  |  |  |  |  |
| PaymentIntent | /v1/payment_intents | core | Action | cancel |  |  | canceled, succeeded |  |
| PaymentLink | /v1/payment_links | core | Deactivate |  |  |  |  |  |
| PaymentMethodConfiguration | /v1/payment_method_configurations | core | Deactivate |  |  |  |  |  |
| PaymentMethodDomain | /v1/payment_method_domains | core | NoOp |  |  |  |  |  |
| PaymentMethod | /v1/payment_methods | core | NoOp |  |  |  |  |  |
| Payout | /v1/payouts | core | Action | cancel |  |  | canceled, failed, paid |  |
| Plan | /v1/plans | core | Delete |  |  |  |  |  |
| Price | /v1/prices | core | Deactivate |  |  |  |  |  |
| TrialOffer | /v1/product_catalog/trial_offers | productcatalog | Deactivate |  |  |  |  |  |
| Product | /v1/products | core | DeleteElseDeactivate |  |  |  |  |  |
| PromotionCode | /v1/promotion_codes | core | Deactivate |  |  |  |  |  |
| Quote | /v1/quotes | core | Action | cancel |  |  | accepted, canceled |  |
| ValueListItem | /v1/radar/value_list_items | radar | Delete |  |  |  |  |  |
| ValueList | /v1/radar/value_lists | radar | Delete |  |  |  |  |  |
| Refund | /v1/refunds | core | Action | cancel |  |  | canceled, failed, succeeded |  |
| ReportRun | /v1/reporting/report_runs | reporting | NoOp |  |  |  |  |  |
| SetupIntent | /v1/setup_intents | core | Action | cancel |  |  | canceled, succeeded |  |
| ShippingRate | /v1/shipping_rates | core | Deactivate |  |  |  |  |  |
| Source | /v1/sources | core | NoOp |  |  |  |  |  |
| SubscriptionItem | /v1/subscription_items | core | Delete |  |  |  |  |  |
| SubscriptionSchedule | /v1/subscription_schedules | core | Action | cancel |  |  | canceled, completed, released |  |
| Subscription | /v1/subscriptions | core | Delete |  |  |  |  |  |
| Calculation | /v1/tax/calculations | tax | NoOp |  |  |  |  |  |
| Location | /v1/tax/locations | tax | NoOp |  |  |  |  |  |
| Registration | /v1/tax/registrations | tax | NoOp |  |  |  |  |  |
| TaxId | /v1/tax_ids | core | Delete |  |  |  |  |  |
| TaxRate | /v1/tax_rates | core | Deactivate |  |  |  |  |  |
| Configuration | /v1/terminal/configurations | terminal | Delete |  |  |  |  |  |
| Location | /v1/terminal/locations | terminal | Delete |  |  |  |  |  |
| Reader | /v1/terminal/readers | terminal | Delete |  |  |  |  |  |
| TestClock | /v1/test_helpers/test_clocks | testhelpers | Delete |  |  |  |  |  |
| Authentication | /v1/three_d_secure/authentications | threedsecure | Action | cancel |  |  | canceled, error, failed, succeeded |  |
| Topup | /v1/topups | core | Action | cancel |  |  | canceled, failed, reversed, succeeded |  |
| Transfer | /v1/transfers | core | NoOp |  |  |  |  |  |
| CreditReversal | /v1/treasury/credit_reversals | treasury | NoOp |  |  |  |  |  |
| DebitReversal | /v1/treasury/debit_reversals | treasury | NoOp |  |  |  |  |  |
| FinancialAccount | /v1/treasury/financial_accounts | treasury | Action | close |  |  | closed |  |
| InboundTransfer | /v1/treasury/inbound_transfers | treasury | Action | cancel |  |  | canceled, failed, succeeded |  |
| OutboundPayment | /v1/treasury/outbound_payments | treasury | Action | cancel |  |  | canceled, failed, posted, returned |  |
| OutboundTransfer | /v1/treasury/outbound_transfers | treasury | Action | cancel |  |  | canceled, failed, posted, returned |  |
| WebhookEndpoint | /v1/webhook_endpoints | core | Delete |  |  |  |  |  |
