// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	groupCore   = "core"
	fieldAmount = "amount"
)

func classifyFile(t *testing.T, name string) ([]Kind, error) {
	t.Helper()
	spec, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return Classify(spec)
}

func classifyFixture(t *testing.T) []Kind {
	t.Helper()
	kinds, err := classifyFile(t, "testdata/spec.json")
	if err != nil {
		t.Fatal(err)
	}
	return kinds
}

func TestClassify(t *testing.T) {
	want := []Kind{
		{
			Name:             "Install",
			Group:            "apps",
			Path:             "/v1/apps/installs",
			ItemPath:         "/v1/apps/installs/{id}",
			CreateFields:     []string{"app"},
			CreateOnlyFields: []string{"app"},
			Deletion:         Deletion{Mode: Action, Action: "uninstall", EndStatuses: []string{"install_failed", "uninstalling"}},
		},
		{
			Name:             "Charge",
			Group:            groupCore,
			Path:             "/v1/charges",
			ItemPath:         "/v1/charges/{charge}",
			CreateFields:     []string{fieldAmount, "description"},
			UpdatableFields:  []string{"description"},
			CreateOnlyFields: []string{fieldAmount},
			Deletion:         Deletion{Mode: NoOp},
		},
		{
			Name:             "Invoice",
			Group:            groupCore,
			Path:             "/v1/invoices",
			ItemPath:         "/v1/invoices/{invoice}",
			CreateFields:     []string{"customer"},
			CreateOnlyFields: []string{"customer"},
			Deletion:         Deletion{Mode: DeleteDraftElseAction, Action: "void", EndStatuses: []string{"paid", "void"}},
		},
		{
			Name:             "Cardholder",
			Group:            "issuing",
			Path:             "/v1/issuing/cardholders",
			ItemPath:         "/v1/issuing/cardholders/{cardholder}",
			CreateFields:     []string{"name"},
			CreateOnlyFields: []string{"name"},
			Deletion:         Deletion{Mode: SetField, Field: "status", Value: "inactive"},
		},
		{
			Name:             "Card",
			Group:            "issuing",
			Path:             "/v1/issuing/cards",
			ItemPath:         "/v1/issuing/cards/{card}",
			CreateFields:     []string{"currency"},
			CreateOnlyFields: []string{"currency"},
			Deletion:         Deletion{Mode: SetField, Field: "status", Value: "canceled"},
		},
		{
			Name:             "Payout",
			Group:            groupCore,
			Path:             "/v1/payouts",
			ItemPath:         "/v1/payouts/{payout}",
			CreateFields:     []string{fieldAmount},
			CreateOnlyFields: []string{fieldAmount},
			Deletion:         Deletion{Mode: NoOp},
		},
		{
			Name:             "Price",
			Group:            groupCore,
			Path:             "/v1/prices",
			ItemPath:         "/v1/prices/{price}",
			CreateFields:     []string{"active", "currency", "nickname"},
			UpdatableFields:  []string{"active", "nickname"},
			CreateOnlyFields: []string{"currency"},
			Deletion:         Deletion{Mode: Deactivate},
		},
		{
			Name:             "Product",
			Group:            groupCore,
			Path:             "/v1/products",
			ItemPath:         "/v1/products/{id}",
			CreateFields:     []string{"active", "name", "type"},
			UpdatableFields:  []string{"active", "default_price", "name"},
			CreateOnlyFields: []string{"type"},
			UpdateOnlyFields: []string{"default_price"},
			Deletion:         Deletion{Mode: DeleteElseDeactivate},
		},
		{
			Name:             "TestClock",
			Group:            "testhelpers",
			Path:             "/v1/test_helpers/test_clocks",
			ItemPath:         "/v1/test_helpers/test_clocks/{test_clock}",
			CreateFields:     []string{"frozen_time"},
			CreateOnlyFields: []string{"frozen_time"},
			Deletion:         Deletion{Mode: Delete},
		},
		{
			Name:             "InboundTransfer",
			Group:            "treasury",
			Path:             "/v1/treasury/inbound_transfers",
			ItemPath:         "/v1/treasury/inbound_transfers/{id}",
			CreateFields:     []string{fieldAmount},
			CreateOnlyFields: []string{fieldAmount},
			Deletion:         Deletion{Mode: Action, Action: "cancel", EndStatuses: []string{"canceled", "failed", "succeeded"}},
		},
	}
	if diff := cmp.Diff(want, classifyFixture(t)); diff != "" {
		t.Errorf("Classify (-want +got):\n%s", diff)
	}
}

func TestClassifyRejectsUnexpectedShapes(t *testing.T) {
	cases := map[string]string{
		"two-item-paths.json":   "/v1/widgets: multiple item paths",
		"two-action-paths.json": "/v1/widgets: multiple cancel action paths",
		"empty-name-word.json":  "/v1/widget__parts: empty word in name",
		"multipart-create.json": "/v1/widgets: request body is not inline",
		"ref-item-post.json":    "/v1/widgets/{id}: request body is not inline",
	}
	for file, want := range cases {
		t.Run(file, func(t *testing.T) {
			_, err := classifyFile(t, "testdata/invalid/"+file)
			if err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Errorf("Classify error = %v, want prefix %q", err, want)
			}
		})
	}
}

func TestDeleteTable(t *testing.T) {
	want, err := os.ReadFile("testdata/delete-modes.md")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(want), DeleteTable(classifyFixture(t))); diff != "" {
		t.Errorf("DeleteTable (-want +got):\n%s", diff)
	}
}

func TestCheckEntries(t *testing.T) {
	cases := map[string]string{
		"testdata/spec.json":          "",
		"testdata/stale-entries.json": "entries missing from spec: [/v1/apps/installs /v1/billing/alerts /v1/billing/credit_grants /v1/billing/feedback_options /v1/billing/meters /v1/checkout/sessions /v1/climate/orders /v1/credit_notes /v1/identity/verification_sessions /v1/issuing/cardholders /v1/payouts /v1/quotes /v1/radar/value_lists /v1/refunds /v1/setup_intents /v1/subscription_schedules /v1/three_d_secure/authentications /v1/topups /v1/treasury/inbound_transfers /v1/treasury/outbound_payments /v1/treasury/outbound_transfers]",
	}
	for file, want := range cases {
		t.Run(file, func(t *testing.T) {
			spec, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if err := CheckEntries(spec); err != nil {
				got = err.Error()
			}
			if got != want {
				t.Errorf("CheckEntries error = %q, want %q", got, want)
			}
		})
	}
}

func TestClassifyRequiresEndState(t *testing.T) {
	spec := `{"paths": {
		"/v1/widgets": {"post": {"requestBody": {"content": {"application/x-www-form-urlencoded": {"schema": {"properties": {"a": {}}}}}}}},
		"/v1/widgets/{id}": {"get": {}},
		"/v1/widgets/{id}/cancel": {"post": {}}
	}}`
	_, err := Classify([]byte(spec))
	if want := "/v1/widgets: no end state"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("Classify error = %v, want prefix %q", err, want)
	}
}
