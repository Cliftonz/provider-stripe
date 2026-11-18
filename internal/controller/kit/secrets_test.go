// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/google/go-cmp/cmp"
)

func withSecretFields(fields ...string) Descriptor {
	d := widgets
	d.SecretFields = fields
	return d
}

func TestCreatePublishesDescriptorSecretFieldsAsConnectionDetails(t *testing.T) {
	cases := map[string]struct {
		descriptor Descriptor
		response   string
		want       managed.ConnectionDetails
	}{
		"WebhookEndpointSecret": {
			descriptor: withSecretFields("secret"),
			response:   `{"id":"we_1","object":"webhook_endpoint","url":"https://example.com/hook","secret":"whsec_abc"}`,
			want:       managed.ConnectionDetails{"secret": []byte("whsec_abc")},
		},
		"IssuingCardNumberAndCVC": {
			descriptor: withSecretFields("number", "cvc"),
			response:   `{"id":"ic_1","object":"issuing.card","number":"4242424242424242","cvc":"123"}`,
			want:       managed.ConnectionDetails{"number": []byte("4242424242424242"), "cvc": []byte("123")},
		},
		"KindWithoutSecretFields": {
			descriptor: widgets,
			response:   `{"id":"in_1","object":"invoice","number":"INV-0001","secret":"not_listed"}`,
			want:       managed.ConnectionDetails{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, _ := widgetExternal(t, tc.descriptor, http.StatusOK, tc.response)
			w := &widget{}
			w.SetUID("uid-123")

			c, err := e.Create(t.Context(), w)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.want, c.ConnectionDetails); diff != "" {
				t.Errorf("connection details (-want +got):\n%s", diff)
			}
		})
	}
}

func TestObserveMovesDescriptorSecretFieldsFromObservationToConnectionDetails(t *testing.T) {
	cases := map[string]struct {
		descriptor      Descriptor
		wantDetails     managed.ConnectionDetails
		wantObservation map[string]any
	}{
		"IssuingCardNumberAndCVC": {
			descriptor:      withSecretFields("number", "cvc"),
			wantDetails:     managed.ConnectionDetails{"number": []byte("4242424242424242"), "cvc": []byte("123")},
			wantObservation: map[string]any{"id": "wid_1", "object": "widget", "currency": "usd"},
		},
		"KindWithoutSecretFieldsKeepsNumber": {
			descriptor:      widgets,
			wantDetails:     managed.ConnectionDetails{},
			wantObservation: map[string]any{"id": "wid_1", "object": "widget", "currency": "usd", "number": "4242424242424242", "cvc": "123"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, w := observeWidget(t, tc.descriptor, inSyncWidget(), `{"id":"wid_1","object":"widget","currency":"usd","number":"4242424242424242","cvc":"123"}`)

			if diff := cmp.Diff(tc.wantDetails, o.ConnectionDetails); diff != "" {
				t.Errorf("connection details (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantObservation, w.observation); diff != "" {
				t.Errorf("observation (-want +got):\n%s", diff)
			}
		})
	}
}
