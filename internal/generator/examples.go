// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	farFuture     = 4102444800
	nameField     = "name"
	currencyField = "currency"
	amountField   = "amount"
	typeField     = "type"
	e2eName       = "e2e"
	usd           = "usd"
)

type exampleSchema struct {
	Type       string                    `json:"type"`
	Format     string                    `json:"format"`
	Enum       []any                     `json:"enum"`
	Example    any                       `json:"example"`
	Properties map[string]*exampleSchema `json:"properties"`
	Items      *exampleSchema            `json:"items"`
	AnyOf      []*exampleSchema          `json:"anyOf"`
	Required   []string                  `json:"required"`
}

var placeholders = map[string]any{stringType: "example", "integer": 1, "number": 1, "boolean": true}

var e2eMetadata = map[string]any{"crossplane-e2e": "true"}

var saasSkips = []string{
	"/v1/accounts",
	"/v1/apple_pay/",
	"/v1/apps/",
	"/v1/billing/alerts",
	"/v1/billing/feedback_options",
	"/v1/climate/",
	"/v1/credit_notes",
	"/v1/file_links",
	"/v1/financial_connections/",
	"/v1/forwarding/",
	"/v1/identity/",
	"/v1/issuing/",
	"/v1/link_account_sessions",
	"/v1/payouts",
	"/v1/product_catalog/trial_offers",
	"/v1/promotion_codes",
	"/v1/radar/value_lists",
	"/v1/reporting/",
	"/v1/sources",
	"/v1/subscription_schedules",
	"/v1/subscriptions",
	"/v1/tax/calculations",
	"/v1/tax/locations",
	"/v1/terminal/readers",
	"/v1/three_d_secure/",
	"/v1/topups",
	"/v1/transfers",
	"/v1/treasury/",
}

type manifest = map[string]any

var (
	customerRef  = manifest{nameField: "example-core-customer"}
	e2eLineItems = []any{manifest{"quantity": 1, "priceData": manifest{currencyField: usd, "unitAmount": 2000, "productData": manifest{nameField: e2eName}}}}
)

var exampleOverrides = map[string]manifest{
	"/v1/billing/credit_grants": {
		amountField:           manifest{typeField: "monetary", "monetary": manifest{currencyField: usd, "value": 1000}},
		"applicabilityConfig": manifest{"scope": manifest{"priceType": "metered"}},
		"category":            "promotional",
		"customerRef":         customerRef,
	},
	"/v1/billing_portal/configurations": {"features": manifest{"invoiceHistory": manifest{"enabled": true}}},
	"/v1/charges":                       {amountField: 2000, currencyField: usd, "source": "tok_visa"},
	"/v1/checkout/sessions":             {"mode": "payment", "successUrl": "https://example.com", "lineItems": e2eLineItems},
	"/v1/coupons":                       {"percentOff": 25, "duration": "once"},
	"/v1/invoiceitems":                  {"customerRef": customerRef, amountField: 2000, currencyField: usd},
	"/v1/invoices":                      {"customerRef": customerRef},
	"/v1/payment_intents":               {amountField: 2000, currencyField: usd},
	"/v1/payment_links":                 {"lineItems": e2eLineItems},
	"/v1/payment_method_configurations": {nameField: e2eName},
	"/v1/payment_method_domains":        {"domainName": "example.com"},
	"/v1/payment_methods":               {typeField: "card", "card": manifest{"token": "tok_visa"}},
	"/v1/plans":                         {amountField: 2000, "product": manifest{nameField: e2eName}},
	"/v1/prices":                        {"productRef": manifest{nameField: "example-core-product"}, "unitAmount": 2000, currencyField: usd},
	"/v1/quotes":                        {"customerRef": customerRef},
	"/v1/refunds":                       {"chargeRef": manifest{nameField: "example-core-charge"}},
	"/v1/setup_intents":                 {"paymentMethod": "pm_card_visa"},
	"/v1/shipping_rates":                {typeField: "fixed_amount", "fixedAmount": manifest{amountField: 500, currencyField: usd}},
	"/v1/tax/registrations":             {"countryOptions": manifest{"us": manifest{"state": "CA", typeField: "state_sales_tax"}}},
	"/v1/tax_ids":                       {typeField: "eu_vat", "value": "DE123456789"},
	"/v1/terminal/configurations":       {nameField: e2eName},
	"/v1/terminal/locations": {
		"displayName": e2eName,
		"address":     manifest{"line1": "1272 Valencia Street", "city": "San Francisco", "state": "CA", "country": "US", "postalCode": "94110"},
	},
}

func Examples(spec []byte, kinds []Kind, header []byte) (map[string][]byte, error) {
	var doc struct {
		Paths map[string]struct {
			Post *struct {
				RequestBody struct {
					Content map[string]struct {
						Schema *exampleSchema `json:"schema"`
					} `json:"content"`
				} `json:"requestBody"`
			} `json:"post"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	byPath := map[string]Kind{}
	creates := map[string]*exampleSchema{}
	for _, k := range kinds {
		byPath[k.Path] = k
		creates[k.Path] = doc.Paths[k.Path].Post.RequestBody.Content[formContentType].Schema
	}
	files := map[string][]byte{}
	for _, k := range kinds {
		params := exampleParams(k, creates[k.Path], byPath)
		id := k.Group + "/" + version + "/" + strings.ToLower(k.Name)
		files[id+".yaml"] = exampleDocument(k, id, params, saasSkipped(k.Path, creates), header)
	}
	return files, nil
}

func exampleParams(k Kind, create *exampleSchema, byPath map[string]Kind) map[string]any {
	params := map[string]any{}
	for _, name := range create.Required {
		_, jsonName, _ := names(name)
		target, isRef := references[name]
		switch {
		case isRef && !slices.Contains(referenceExclusions[k.Path], name) && create.Properties[name].Type == "array":
			params[jsonName+"Refs"] = []any{map[string]any{nameField: exampleName(byPath[target])}}
		case isRef && !slices.Contains(referenceExclusions[k.Path], name):
			params[jsonName+"Ref"] = map[string]any{nameField: exampleName(byPath[target])}
		default:
			if v, ok := exampleValue(name, create.Properties[name]); ok {
				params[jsonName] = v
			}
		}
	}
	if slices.Contains(k.CreateFields, "metadata") || slices.Contains(k.UpdateOnlyFields, "metadata") {
		params["metadata"] = e2eMetadata
	}
	maps.Copy(params, exampleOverrides[k.Path])
	return params
}

func saasSkipped(path string, creates map[string]*exampleSchema) bool {
	if slices.ContainsFunc(saasSkips, func(prefix string) bool { return strings.HasPrefix(path, prefix) }) {
		return true
	}
	for _, name := range creates[path].Required {
		target, isRef := references[name]
		if isRef && !slices.Contains(referenceExclusions[path], name) && saasSkipped(target, creates) {
			return true
		}
	}
	return false
}

func exampleDocument(k Kind, id string, params map[string]any, skip bool, header []byte) []byte {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(string(header), "//", "#"))
	fmt.Fprintf(&b, `
# Code generated by cmd/generator. DO NOT EDIT.

apiVersion: %s%s/%s
kind: %s
metadata:
  name: %s
  namespace: default
  annotations:
    meta.upbound.io/example-id: %s
`, k.Group, groupSuffix, version, k.Name, exampleName(k), id)
	if skip {
		b.WriteString("    stripe.m.clifton.quest/e2e-saas: skip\n")
	}
	b.WriteString(`spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
`)
	writeYAML(&b, map[string]any{"forProvider": params}, "  ")
	return []byte(b.String())
}

func exampleName(k Kind) string {
	return "example-" + k.Group + "-" + strings.ToLower(k.Name)
}

func exampleValue(name string, s *exampleSchema) (any, bool) {
	switch {
	case len(s.AnyOf) > 0:
		return exampleValue(name, firstNonEmptyBranch(s.AnyOf))
	case s.Example != nil:
		return s.Example, true
	case len(s.Enum) > 0:
		return s.Enum[0], true
	case s.Type == "array":
		v, _ := exampleValue(name, s.Items)
		return []any{v}, true
	case s.Type == objectType && s.Properties != nil && len(s.Properties) == 0:
		return nil, false
	case s.Type == objectType:
		return requiredValues(s), true
	}
	return placeholder(name, s), true
}

func firstNonEmptyBranch(branches []*exampleSchema) *exampleSchema {
	for _, branch := range branches {
		if branch.Type != stringType || len(branch.Enum) != 1 || branch.Enum[0] != "" {
			return branch
		}
	}
	panic("anyOf has only empty-string branches")
}

func requiredValues(s *exampleSchema) map[string]any {
	object := map[string]any{}
	for _, name := range s.Required {
		if v, ok := exampleValue(name, s.Properties[name]); ok {
			_, jsonName, _ := names(name)
			object[jsonName] = v
		}
	}
	return object
}

func placeholder(name string, s *exampleSchema) any {
	switch {
	case s.Format == "unix-time":
		return farFuture
	case name == currencyField || strings.HasSuffix(name, "currencies") || s.Format == currencyField:
		return usd
	case name == "country" || strings.HasSuffix(name, "countries"):
		return "US"
	case name == "email":
		return "e2e@example.com"
	case name == "url" || strings.HasSuffix(name, "_url"):
		return "https://example.com"
	}
	return placeholders[s.Type]
}

func writeYAML(b *strings.Builder, m map[string]any, indent string) {
	for _, key := range slices.Sorted(maps.Keys(m)) {
		switch v := m[key].(type) {
		case map[string]any:
			if len(v) == 0 {
				b.WriteString(indent + key + ": {}\n")
				continue
			}
			b.WriteString(indent + key + ":\n")
			writeYAML(b, v, indent+"  ")
		case []any:
			b.WriteString(indent + key + ":\n")
			writeItems(b, v, indent+"  ")
		default:
			b.WriteString(indent + key + ": " + scalar(v) + "\n")
		}
	}
}

func writeItems(b *strings.Builder, items []any, indent string) {
	for _, item := range items {
		object, isObject := item.(map[string]any)
		if !isObject {
			b.WriteString(indent + "- " + scalar(item) + "\n")
			continue
		}
		var nested strings.Builder
		writeYAML(&nested, object, indent+"  ")
		b.WriteString(indent + "- " + strings.TrimPrefix(nested.String(), indent+"  "))
	}
}

func scalar(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(out)
}
