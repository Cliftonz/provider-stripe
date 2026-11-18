// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

const (
	testAPIKey     = "sk_test_1"
	widgetItemPath = "/v1/widgets/wid_1"
	voidAction     = "void"
	badRequest     = `{"error":{"type":"invalid_request_error","message":"bad"}}`
)

func withDeletion(d Deletion) Descriptor {
	w := widgets
	w.Deletion = d
	return w
}

func routedExternal(t *testing.T, d Deletion, route func(method string) (int, string)) (*External[*widget], *[]request) {
	t.Helper()
	var got []request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		got = append(got, request{Method: r.Method, Path: r.URL.Path, Body: string(b)})
		status, body := route(r.Method)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &External[*widget]{Client: NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()), Descriptor: withDeletion(d)}, &got
}

func TestDeleteSendsRequestsForMode(t *testing.T) {
	cases := map[string]struct {
		deletion Deletion
		remote   string
		want     []request
	}{
		"Delete": {
			deletion: Deletion{Mode: Delete},
			want:     []request{{Method: http.MethodDelete, Path: widgetItemPath}},
		},
		"Deactivate": {
			deletion: Deletion{Mode: Deactivate},
			want:     []request{{Method: http.MethodPost, Path: widgetItemPath, Body: "active=false"}},
		},
		"Action": {
			deletion: Deletion{Mode: Action, Action: "cancel"},
			want:     []request{{Method: http.MethodPost, Path: widgetItemPath + "/cancel"}},
		},
		"SetField": {
			deletion: Deletion{Mode: SetField, Field: "status", Value: "canceled"},
			want:     []request{{Method: http.MethodPost, Path: widgetItemPath, Body: "status=canceled"}},
		},
		"DeleteDraftElseActionDraft": {
			deletion: Deletion{Mode: DeleteDraftElseAction, Action: voidAction},
			remote:   `{"id":"wid_1","status":"draft"}`,
			want: []request{
				{Method: http.MethodGet, Path: widgetItemPath},
				{Method: http.MethodDelete, Path: widgetItemPath},
			},
		},
		"DeleteDraftElseActionOpen": {
			deletion: Deletion{Mode: DeleteDraftElseAction, Action: voidAction},
			remote:   `{"id":"wid_1","status":"open"}`,
			want: []request{
				{Method: http.MethodGet, Path: widgetItemPath},
				{Method: http.MethodPost, Path: widgetItemPath + "/" + voidAction},
			},
		},
		"DeleteElseDeactivate": {
			deletion: Deletion{Mode: DeleteElseDeactivate},
			want:     []request{{Method: http.MethodDelete, Path: widgetItemPath}},
		},
		"NoOp": {
			deletion: Deletion{Mode: NoOp},
			want:     []request{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			remote := tc.remote
			if remote == "" {
				remote = `{"id":"wid_1"}`
			}
			e, got := widgetExternal(t, withDeletion(tc.deletion), http.StatusOK, remote)

			if _, err := e.Delete(t.Context(), namedWidget(widgetParams{})); err != nil {
				t.Fatal(err)
			}

			sent := make([]request, 0, len(*got))
			for _, r := range *got {
				sent = append(sent, request{Method: r.Method, Path: r.Path, Body: r.Body})
			}
			if diff := cmp.Diff(tc.want, sent); diff != "" {
				t.Errorf("requests (-want +got):\n%s", diff)
			}
		})
	}
}

var everyDeletion = map[string]Deletion{
	"Delete":                {Mode: Delete},
	"Deactivate":            {Mode: Deactivate},
	"Action":                {Mode: Action, Action: "cancel", EndStatuses: []string{"canceled"}},
	"SetField":              {Mode: SetField, Field: "status", Value: "canceled"},
	"DeleteDraftElseAction": {Mode: DeleteDraftElseAction, Action: voidAction, EndStatuses: []string{voidAction}},
	"DeleteElseDeactivate":  {Mode: DeleteElseDeactivate},
}

func TestDeleteNotFoundIsSuccess(t *testing.T) {
	for name, d := range everyDeletion {
		t.Run(name, func(t *testing.T) {
			e, _ := widgetExternal(t, withDeletion(d), http.StatusNotFound, gone)

			if _, err := e.Delete(t.Context(), namedWidget(widgetParams{})); err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

func TestDeleteFailureIsError(t *testing.T) {
	for name, d := range everyDeletion {
		t.Run(name, func(t *testing.T) {
			e, _ := widgetExternal(t, withDeletion(d), http.StatusBadRequest, badRequest)

			if _, err := e.Delete(t.Context(), namedWidget(widgetParams{})); err == nil {
				t.Error("err = nil, want error")
			}
		})
	}
}

func TestDeleteUnsetModePanics(t *testing.T) {
	e, _ := widgetExternal(t, widgets, http.StatusOK, `{"id":"wid_1"}`)
	defer func() {
		if recover() == nil {
			t.Error("recover() = nil, want panic")
		}
	}()

	_, _ = e.Delete(t.Context(), namedWidget(widgetParams{}))
}

func TestDeleteDraftElseActionNotFoundAfterReadIsSuccess(t *testing.T) {
	for _, status := range []string{"draft", "open"} {
		t.Run(status, func(t *testing.T) {
			e, _ := routedExternal(t, everyDeletion["DeleteDraftElseAction"], func(method string) (int, string) {
				if method == http.MethodGet {
					return http.StatusOK, `{"id":"wid_1","status":"` + status + `"}`
				}
				return http.StatusNotFound, gone
			})

			if _, err := e.Delete(t.Context(), namedWidget(widgetParams{})); err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

func TestDeleteElseDeactivateDeactivatesWhenDeleteRefused(t *testing.T) {
	cases := map[string]struct {
		postStatus int
		postBody   string
		wantErr    bool
	}{
		"Deactivated":      {postStatus: http.StatusOK, postBody: `{"id":"wid_1","active":false}`},
		"GoneOnDeactivate": {postStatus: http.StatusNotFound, postBody: gone},
		"DeactivateFails":  {postStatus: http.StatusBadRequest, postBody: badRequest, wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, got := routedExternal(t, everyDeletion["DeleteElseDeactivate"], func(method string) (int, string) {
				if method == http.MethodDelete {
					return http.StatusBadRequest, badRequest
				}
				return tc.postStatus, tc.postBody
			})

			_, err := e.Delete(t.Context(), namedWidget(widgetParams{}))

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, want error %v", err, tc.wantErr)
			}
			want := []request{
				{Method: http.MethodDelete, Path: widgetItemPath},
				{Method: http.MethodPost, Path: widgetItemPath, Body: "active=false"},
			}
			if diff := cmp.Diff(want, *got); diff != "" {
				t.Errorf("requests (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteElseDeactivateLifecycleObserveDeleteObserve(t *testing.T) {
	body := `{"id":"wid_1","active":true}`
	e, _ := routedExternal(t, everyDeletion["DeleteElseDeactivate"], func(method string) (int, string) {
		switch method {
		case http.MethodDelete:
			return http.StatusBadRequest, badRequest
		case http.MethodPost:
			body = `{"id":"wid_1","active":false}`
		}
		return http.StatusOK, body
	})
	w := deleting(namedWidget(widgetParams{}))

	exists := make([]bool, 0, 2)
	o, err := e.Observe(t.Context(), w)
	if err != nil {
		t.Fatal(err)
	}
	exists = append(exists, o.ResourceExists)
	if _, err := e.Delete(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	o, err = e.Observe(t.Context(), w)
	if err != nil {
		t.Fatal(err)
	}
	exists = append(exists, o.ResourceExists)

	if diff := cmp.Diff([]bool{true, false}, exists); diff != "" {
		t.Errorf("ResourceExists across Observe, Delete, Observe (-want +got):\n%s", diff)
	}
}
