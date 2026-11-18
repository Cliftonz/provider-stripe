| kind | path | group | delete mode | action | field | value | end statuses | end timestamps |
|------|------|-------|-------------|--------|-------|-------|--------------|----------------|
| Install | /v1/apps/installs | apps | Action | uninstall |  |  | install_failed, uninstalling |  |
| Charge | /v1/charges | core | NoOp |  |  |  |  |  |
| Invoice | /v1/invoices | core | DeleteDraftElseAction | void |  |  | paid, void |  |
| Cardholder | /v1/issuing/cardholders | issuing | SetField |  | status | inactive |  |  |
| Card | /v1/issuing/cards | issuing | SetField |  | status | canceled |  |  |
| Payout | /v1/payouts | core | NoOp |  |  |  |  |  |
| Price | /v1/prices | core | Deactivate |  |  |  |  |  |
| Product | /v1/products | core | DeleteElseDeactivate |  |  |  |  |  |
| TestClock | /v1/test_helpers/test_clocks | testhelpers | Delete |  |  |  |  |  |
| InboundTransfer | /v1/treasury/inbound_transfers | treasury | Action | cancel |  |  | canceled, failed, succeeded |  |
