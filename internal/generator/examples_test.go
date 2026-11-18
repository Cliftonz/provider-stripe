// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExamples(t *testing.T) {
	got, err := Examples(load(t, "testdata/examples/spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(readTree(t, "testdata/examples/golden"), text(got)); diff != "" {
		t.Errorf("Examples (-want +got):\n%s", diff)
	}
}
