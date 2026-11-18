// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

type request struct {
	Method string
	Path   string
	Body   string
	Header http.Header
}

func stripeFake(t *testing.T, status int, body string) (*httptest.Server, *[]request) {
	t.Helper()
	var got []request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		got = append(got, request{Method: r.Method, Path: r.URL.Path, Body: string(b), Header: r.Header.Clone()})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Request-Id", fmt.Sprintf("req_%d", len(got)))
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func requestAt(t *testing.T, got []request, i int) request {
	t.Helper()
	if len(got) <= i {
		t.Fatalf("requests = %d, want at least %d", len(got), i+1)
	}
	return got[i]
}

type debugLog struct {
	debug []string
}

func (l *debugLog) Info(string, ...any)              {}
func (l *debugLog) Debug(msg string, _ ...any)       { l.debug = append(l.debug, msg) }
func (l *debugLog) WithValues(...any) logging.Logger { return l }

type productParams struct {
	Name string `form:"name"`
}

type product struct {
	fake.ModernManaged
	params      productParams
	observation map[string]any
}

func (p *product) GetParameters() any              { return p.params }
func (p *product) SetObservation(o map[string]any) { p.observation = o }

var products = Descriptor{CollectionPath: "/v1/products", ItemPath: "/v1/products/%s"}

func external(c clients.Config) External[*product] {
	return External[*product]{Client: NewClient(c, logging.NewNopLogger()), Descriptor: products}
}

func named(id string) *product {
	mg := &product{}
	mg.SetAnnotations(map[string]string{"crossplane.io/external-name": id})
	return mg
}

func TestRequestsCarryKeyVersionAndAccount(t *testing.T) {
	srv, got := stripeFake(t, http.StatusOK, `{"id":"prod_1"}`)
	e := external(clients.Config{APIKey: testAPIKey, StripeAccount: "acct_1", BaseURL: srv.URL})

	if _, err := e.Observe(t.Context(), named("prod_1")); err != nil {
		t.Fatal(err)
	}

	r := requestAt(t, *got, 0)
	want := map[string]string{"Authorization": "Bearer sk_test_1", "Stripe-Version": "2026-09-30.endive", "Stripe-Account": "acct_1"}
	for k, v := range want {
		if h := r.Header.Get(k); h != v {
			t.Errorf("header %s = %q, want %q", k, h, v)
		}
	}
}

func TestRequestsOmitAccountWhenUnset(t *testing.T) {
	srv, got := stripeFake(t, http.StatusOK, `{"id":"prod_1"}`)
	e := external(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL})

	if _, err := e.Observe(t.Context(), named("prod_1")); err != nil {
		t.Fatal(err)
	}

	if h, ok := requestAt(t, *got, 0).Header["Stripe-Account"]; ok {
		t.Errorf("Stripe-Account header = %q, want absent", h)
	}
}

func secondRequestTelemetry(t *testing.T, enable bool) (string, bool) {
	t.Helper()
	srv, got := stripeFake(t, http.StatusOK, `{"id":"prod_1"}`)
	e := external(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL, EnableTelemetry: enable})
	for range 2 {
		if _, err := e.Observe(t.Context(), named("prod_1")); err != nil {
			t.Fatal(err)
		}
	}
	h, ok := requestAt(t, *got, 1).Header["X-Stripe-Client-Telemetry"]
	return strings.Join(h, ""), ok
}

func TestTelemetryOffByDefault(t *testing.T) {
	if h, ok := secondRequestTelemetry(t, false); ok {
		t.Errorf("X-Stripe-Client-Telemetry = %q, want absent", h)
	}
}

func TestTelemetryReportsPreviousRequestWhenEnabled(t *testing.T) {
	if h, _ := secondRequestTelemetry(t, true); !strings.Contains(h, "req_1") {
		t.Errorf("X-Stripe-Client-Telemetry = %q, want it to report req_1", h)
	}
}

type hostRecorder struct{ host string }

func (h *hostRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	h.host = r.URL.Scheme + "://" + r.URL.Host
	return nil, errors.New("recorded")
}

func TestEmptyBaseURLTargetsStripeAPI(t *testing.T) {
	rec := &hostRecorder{}
	orig := http.DefaultTransport
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = orig })
	e := external(clients.Config{APIKey: testAPIKey})

	_, _ = e.Observe(t.Context(), named("prod_1"))

	if want := "https://api.stripe.com"; rec.host != want {
		t.Errorf("host = %q, want %q", rec.host, want)
	}
}

func TestStripeLogsGoToProviderLoggerAtDebug(t *testing.T) {
	srv, _ := stripeFake(t, http.StatusNotFound, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such product"}}`)
	log := &debugLog{}
	e := External[*product]{Client: NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, log), Descriptor: products}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := make(chan []byte)
	go func() {
		b, _ := io.ReadAll(r)
		stderr <- b
	}()
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = orig
		_ = w.Close()
		if b := <-stderr; len(b) != 0 {
			t.Errorf("stderr = %q, want empty", b)
		}
		_ = r.Close()
	})

	if _, err := e.Observe(t.Context(), named("prod_1")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(log.debug, "\n"), "No such product") {
		t.Errorf("debug logs = %q, want the Stripe error", log.debug)
	}
}
