// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"net/http"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Cliftonz/provider-stripe/apis/v1beta1"
)

func TestConnectorBuildsExternalFromProviderConfig(t *testing.T) {
	srv, got := stripeFake(t, http.StatusOK, remoteWidget)
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1beta1.SchemeBuilder.AddToScheme} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "team-a"},
		Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
			Source: xpv2.CredentialsSourceSecret,
			CommonCredentialSelectors: xpv2.CommonCredentialSelectors{SecretRef: &xpv2.SecretKeySelector{
				SecretReference: xpv2.SecretReference{Name: "stripe-creds"},
				Key:             "credentials",
			}},
		}},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "stripe-creds", Namespace: "team-a"},
		Data:       map[string][]byte{"credentials": []byte(`{"apiKey":"sk_test_conn","baseURL":"` + srv.URL + `"}`)},
	}
	kube := kfake.NewClientBuilder().WithScheme(s).WithObjects(pc, secret).Build()

	w := namedWidget(inSyncWidget())
	w.SetNamespace("team-a")
	w.SetName("widget")
	w.SetUID("widget-uid")
	w.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: v1beta1.ProviderConfigKind, Name: "default"})

	e, err := connector[*widget](kube, widgets, logging.NewNopLogger()).Connect(t.Context(), w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Observe(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	r := requestAt(t, *got, 0)
	if r.Path != widgetItemPath || r.Header.Get("Authorization") != "Bearer sk_test_conn" {
		t.Errorf("request = %s %s with Authorization %q, want /v1/widgets/wid_1 with Bearer sk_test_conn", r.Method, r.Path, r.Header.Get("Authorization"))
	}
}
