// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

func TestUpdatePostsUpdatableFieldsAndRemovesUnownedMetadata(t *testing.T) {
	e, got := widgetExternal(t, widgets, http.StatusOK, remoteWidget)
	p := inSyncWidget()
	p.Currency = "eur"
	p.Archived = ptr(true)
	p.Metadata = map[string]string{"tier": "gold"}

	if _, err := e.Update(t.Context(), namedWidget(p)); err != nil {
		t.Fatal(err)
	}

	r := requestAt(t, *got, max(len(*got)-1, 0))
	if want := []string{http.MethodPost, widgetItemPath}; !cmp.Equal([]string{r.Method, r.Path}, want) {
		t.Errorf("request = %v %v, want %v", r.Method, r.Path, want)
	}
	body, err := url.ParseQuery(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"amount":            {"9007199254740993"},
		"active":            {"true"},
		"archived":          {"true"},
		"metadata[team]":    {""},
		"metadata[tier]":    {"gold"},
		"shipping[carrier]": {"ups"},
		"shipping[days]":    {"3"},
		"tags[0]":           {"a"},
		"tags[1]":           {"b"},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Errorf("body (-want +got):\n%s", diff)
	}
}

func TestUpdateReadFailureIsErrorAndSendsNoUpdate(t *testing.T) {
	e, got := widgetExternal(t, widgets, http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"bad"}}`)

	if _, err := e.Update(t.Context(), namedWidget(inSyncWidget())); err == nil {
		t.Fatal("err = nil, want error")
	}
	for _, r := range *got {
		if r.Method == http.MethodPost {
			t.Errorf("request = %v %v, want no update sent", r.Method, r.Path)
		}
	}
}

func TestUpdateWriteFailureIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, remoteWidget)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"bad"}}`)
	}))
	t.Cleanup(srv.Close)
	e := &External[*widget]{Client: NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()), Descriptor: widgets}

	if _, err := e.Update(t.Context(), namedWidget(inSyncWidget())); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestUpdateSendsEmptyArrayToClearIt(t *testing.T) {
	e, got := widgetExternal(t, widgets, http.StatusOK, remoteWidget)
	p := inSyncWidget()
	p.Tags = []string{}

	if _, err := e.Update(t.Context(), namedWidget(p)); err != nil {
		t.Fatal(err)
	}

	body, err := url.ParseQuery(requestAt(t, *got, max(len(*got)-1, 0)).Body)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{""}, body["tags"]); diff != "" {
		t.Errorf("tags (-want +got):\n%s", diff)
	}
}
