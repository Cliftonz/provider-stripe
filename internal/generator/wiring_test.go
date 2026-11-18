// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWiring(t *testing.T) {
	apis, setup, err := Wiring(load(t, "testdata/types/spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(readTree(t, "testdata/wiring/golden/apis"), text(apis)); diff != "" {
		t.Errorf("Wiring apis (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(readTree(t, "testdata/wiring/golden/controller")["zz_setup.go"], string(setup)); diff != "" {
		t.Errorf("Wiring setup (-want +got):\n%s", diff)
	}
}

var descriptorBlock = regexp.MustCompile(`(?s)\t(\w+) = kit\.Descriptor\{\n(.*?)\n\t\}`)

func descriptors(t *testing.T, name string) map[string]string {
	t.Helper()
	_, setup, err := Wiring(load(t, name))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, m := range descriptorBlock.FindAllStringSubmatch(string(setup), -1) {
		out[m[1]] = m[2]
	}
	return out
}

func TestWiringMarksMoneyMovers(t *testing.T) {
	var got []string
	for name, body := range descriptors(t, "testdata/wiring/spec.json") {
		if regexp.MustCompile(`MovesMoney:\s+true`).MatchString(body) {
			got = append(got, name)
		}
	}
	if diff := cmp.Diff([]string{"coreTopup"}, got); diff != "" {
		t.Errorf("money movers (-want +got):\n%s", diff)
	}
}

func TestWiringSecretFields(t *testing.T) {
	d := descriptors(t, "testdata/wiring/spec.json")
	for name, want := range map[string]string{"issuingCard": `[]string{"cvc", "number"}`, "coreInvoice": "", "coreTopup": ""} {
		got := regexp.MustCompile(`SecretFields:\s+(.*),`).FindStringSubmatch(d[name])
		if want == "" && got != nil || want != "" && (got == nil || got[1] != want) {
			t.Errorf("%s SecretFields = %v, want %q", name, got, want)
		}
	}
}

func TestSecretFieldsLeaveObservation(t *testing.T) {
	spec, kinds, header := load(t, "testdata/wiring/spec.json")
	types, err := Types(spec, kinds, header)
	if err != nil {
		t.Fatal(err)
	}
	apis, _, err := Wiring(spec, kinds, header)
	if err != nil {
		t.Fatal(err)
	}
	card := string(types["issuing/v1alpha1/card.go"])
	if strings.Contains(card, `json:"number`) || strings.Contains(card, `json:"cvc`) || strings.Contains(string(apis["issuing/v1alpha1/kit.go"]), `"number"`) {
		t.Errorf("Card observation keeps number or cvc:\n%s\n%s", card, apis["issuing/v1alpha1/kit.go"])
	}
	if !strings.Contains(string(types["core/v1alpha1/invoice.go"]), `json:"number`) || !strings.Contains(string(apis["core/v1alpha1/kit.go"]), `"number": "number"`) {
		t.Errorf("Invoice observation lost number:\n%s\n%s", types["core/v1alpha1/invoice.go"], apis["core/v1alpha1/kit.go"])
	}
}

func TestWiringLateInitsOnlyEnumsWithinRequestEnum(t *testing.T) {
	got := regexp.MustCompile(`LateInitFields:\s+(.*),`).FindStringSubmatch(descriptors(t, "testdata/wiring/spec.json")["issuingCard"])
	if got == nil || got[1] != `[]string{"type"}` {
		t.Errorf("issuingCard LateInitFields = %v, want [\"type\"]", got)
	}
}
