// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type widgetObservation struct {
	ID         *string           `json:"id,omitempty"`
	UnitAmount *int64            `json:"unitAmount,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func TestObserveRenamesListedKeysIntoObservation(t *testing.T) {
	obs := widgetObservation{ID: ptr("stale")}
	Observe(map[string]any{"unit_amount": json.Number("9007199254740993"), "metadata": map[string]any{"team": "core"}, "secret": "sk"}, &obs, map[string]string{
		"id":          "id",
		"unit_amount": "unitAmount",
		"metadata":    "metadata",
	})
	want := widgetObservation{UnitAmount: ptr(int64(9007199254740993)), Metadata: map[string]string{"team": "core"}}
	if diff := cmp.Diff(want, obs); diff != "" {
		t.Errorf("Observe (-want +got):\n%s", diff)
	}
}

func TestObservePanicsOnShapeMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Observe did not panic on a response value that does not fit the observation")
		}
	}()
	Observe(map[string]any{"id": map[string]any{"object": "widget"}}, &widgetObservation{}, map[string]string{"id": "id"})
}
