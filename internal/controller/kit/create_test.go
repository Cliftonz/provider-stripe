// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

func TestCreatePostsParametersWithUIDIdempotencyKeyAndRecordsID(t *testing.T) {
	srv, got := stripeFake(t, http.StatusOK, `{"id":"prod_new","name":"Gold Plan"}`)
	e := external(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL})
	mg := &product{params: productParams{Name: "Gold Plan"}}
	mg.SetUID("uid-123")

	if _, err := e.Create(t.Context(), mg); err != nil {
		t.Fatal(err)
	}

	r := requestAt(t, *got, 0)
	if want := []string{http.MethodPost, "/v1/products", "name=Gold+Plan", "uid-123"}; !reflect.DeepEqual([]string{r.Method, r.Path, r.Body, r.Header.Get("Idempotency-Key")}, want) {
		t.Errorf("request = %v %v %v idempotency %v, want %v", r.Method, r.Path, r.Body, r.Header.Get("Idempotency-Key"), want)
	}
	if name := meta.GetExternalName(mg); name != "prod_new" {
		t.Errorf("external name = %q, want %q", name, "prod_new")
	}
}

type pricedProductParams struct {
	Name         string            `form:"name"`
	DefaultPrice string            `form:"default_price"`
	Metadata     map[string]string `form:"metadata"`
}

type pricedProduct struct {
	product
	params pricedProductParams
}

func (p *pricedProduct) GetParameters() any { return p.params }

func TestCreateOmitsUpdateOnlyFields(t *testing.T) {
	srv, got := stripeFake(t, http.StatusOK, `{"id":"prod_new"}`)
	e := External[*pricedProduct]{
		Client:     NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()),
		Descriptor: Descriptor{CollectionPath: "/v1/products", UpdateOnlyFields: []string{"default_price", "metadata"}},
	}
	mg := &pricedProduct{params: pricedProductParams{Name: "Gold", DefaultPrice: "price_1", Metadata: map[string]string{"tier": "gold"}}}
	mg.SetUID("uid-123")

	if _, err := e.Create(t.Context(), mg); err != nil {
		t.Fatal(err)
	}

	if body := requestAt(t, *got, 0).Body; body != "name=Gold" {
		t.Errorf("body = %q, want %q", body, "name=Gold")
	}
}

func TestCreateFailureIsError(t *testing.T) {
	srv, _ := stripeFake(t, http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"bad"}}`)
	e := external(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL})
	mg := &product{}
	mg.SetUID("uid-123")

	if _, err := e.Create(t.Context(), mg); err == nil {
		t.Fatal("err = nil, want error")
	}
	if name := meta.GetExternalName(mg); name != "" {
		t.Errorf("external name = %q, want empty", name)
	}
}
