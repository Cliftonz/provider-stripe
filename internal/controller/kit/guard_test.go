// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"errors"
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

func TestCreateLiveMoneyMovementGuard(t *testing.T) {
	cases := map[string]struct {
		apiKey      string
		movesMoney  bool
		allowLive   bool
		wantBlocked bool
	}{
		"TestKeyMoneyKindNotAllowed":    {apiKey: testAPIKey, movesMoney: true},
		"TestKeyMoneyKindAllowed":       {apiKey: testAPIKey, movesMoney: true, allowLive: true},
		"LiveKeyMoneyKindNotAllowed":    {apiKey: "sk_live_1", movesMoney: true, wantBlocked: true},
		"LiveKeyMoneyKindAllowed":       {apiKey: "sk_live_1", movesMoney: true, allowLive: true},
		"RestrictedLiveKeyMoneyKind":    {apiKey: "rk_live_1", movesMoney: true, wantBlocked: true},
		"LiveKeyNonMoneyKindNotAllowed": {apiKey: "sk_live_1"},
		"RestrictedLiveKeyNonMoneyKind": {apiKey: "rk_live_1"},
		"RestrictedTestKeyMoneyKind":    {apiKey: "rk_test_1", movesMoney: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv, got := stripeFake(t, http.StatusOK, `{"id":"pi_1"}`)
			d := products
			d.MovesMoney = tc.movesMoney
			e := External[*product]{
				Client:     NewClient(clients.Config{APIKey: tc.apiKey, BaseURL: srv.URL, AllowLiveMoneyMovement: tc.allowLive}, logging.NewNopLogger()),
				Descriptor: d,
			}
			mg := &product{}
			mg.SetUID("uid-123")

			_, err := e.Create(t.Context(), mg)

			if blocked := errors.Is(err, ErrLiveMoneyMovement); blocked != tc.wantBlocked {
				t.Errorf("blocked = %v (err %v), want %v", blocked, err, tc.wantBlocked)
			}
			if sent := len(*got) > 0; sent == tc.wantBlocked {
				t.Errorf("request sent = %v, want %v", sent, !tc.wantBlocked)
			}
		})
	}
}
