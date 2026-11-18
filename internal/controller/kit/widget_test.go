// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/stripe/stripe-go/v87/form"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

type widgetShipping struct {
	Carrier *string `form:"carrier"`
	Days    *int64  `form:"days"`
}

type widgetItem struct {
	Price    *string `form:"price"`
	Quantity *int64  `form:"quantity"`
}

type widgetJSON struct {
	apiextensionsv1.JSON `json:",inline" form:"-"`
}

func (j widgetJSON) AppendTo(values *form.Values, keyParts []string) {
	d := json.NewDecoder(bytes.NewReader(j.Raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		panic(err)
	}
	form.AppendToPrefixed(values, v, keyParts)
}

func rawJSON(s string) *widgetJSON {
	return &widgetJSON{apiextensionsv1.JSON{Raw: []byte(s)}}
}

type widgetCurrencyOption struct {
	UnitAmount *int64 `form:"unit_amount"`
}

type widgetParams struct {
	Currency        string                          `form:"currency"`
	Amount          *int64                          `form:"amount"`
	Active          *bool                           `form:"active"`
	Description     *string                         `form:"description"`
	Metadata        map[string]string               `form:"metadata"`
	Shipping        *widgetShipping                 `form:"shipping"`
	Tags            []string                        `form:"tags"`
	Archived        *bool                           `form:"archived"`
	Items           []widgetItem                    `form:"items"`
	Rate            *float64                        `form:"rate"`
	Config          *widgetJSON                     `form:"config"`
	CurrencyOptions map[string]widgetCurrencyOption `form:"currency_options"`
}

type widget struct {
	fake.ModernManaged
	params      widgetParams
	observation map[string]any
}

func (w *widget) GetParameters() any              { return &w.params }
func (w *widget) SetObservation(o map[string]any) { w.observation = o }

var widgets = Descriptor{
	CollectionPath:  "/v1/widgets",
	ItemPath:        "/v1/widgets/%s",
	UpdatableFields: strings.Fields("amount active description metadata shipping tags archived items rate config currency_options"),
	LateInitFields:  []string{"active"},
}

const remoteWidget = `{"id":"wid_1","object":"widget","currency":"usd","amount":9007199254740993,"active":true,"description":"remote","metadata":{"team":"core"},"shipping":{"carrier":"ups","days":3,"tracked":true},"tags":["a","b"],"archived":false}`

func ptr[T any](v T) *T { return &v }

func inSyncWidget() widgetParams {
	return widgetParams{
		Currency: "usd",
		Amount:   ptr(int64(9007199254740993)),
		Active:   ptr(true),
		Metadata: map[string]string{"team": "core"},
		Shipping: &widgetShipping{Carrier: ptr("ups"), Days: ptr(int64(3))},
		Tags:     []string{"a", "b"},
	}
}

func widgetExternal(t *testing.T, d Descriptor, status int, body string) (*External[*widget], *[]request) {
	t.Helper()
	srv, got := stripeFake(t, status, body)
	return &External[*widget]{Client: NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()), Descriptor: d}, got
}

func namedWidget(p widgetParams) *widget {
	w := &widget{params: p}
	w.SetAnnotations(map[string]string{"crossplane.io/external-name": "wid_1"})
	return w
}

func observeWidget(t *testing.T, d Descriptor, p widgetParams, body string) (managed.ExternalObservation, *widget) {
	t.Helper()
	e, _ := widgetExternal(t, d, http.StatusOK, body)
	w := namedWidget(p)
	o, err := e.Observe(t.Context(), w)
	if err != nil {
		t.Fatal(err)
	}
	return o, w
}
