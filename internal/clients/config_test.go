// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Cliftonz/provider-stripe/apis/v1beta1"
)

const (
	namespace          = "team-a"
	providerConfigName = "default"
	testAPIKey         = "sk_test_123"
)

func credentialsSpec(secretNamespace string) v1beta1.ProviderConfigSpec {
	return v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
		Source: xpv2.CredentialsSourceSecret,
		CommonCredentialSelectors: xpv2.CommonCredentialSelectors{SecretRef: &xpv2.SecretKeySelector{
			SecretReference: xpv2.SecretReference{Name: "stripe-creds", Namespace: secretNamespace},
			Key:             "credentials",
		}},
	}}
}

func kubeClient(t *testing.T, pc client.Object, secretNamespace, credentials string) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1beta1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "stripe-creds", Namespace: secretNamespace},
		Data:       map[string][]byte{"credentials": []byte(credentials)},
	}
	return kfake.NewClientBuilder().WithScheme(s).WithObjects(pc, secret).Build()
}

func managed(kind string) *fake.ModernManaged {
	mg := &fake.ModernManaged{ObjectMeta: metav1.ObjectMeta{Name: "customer", Namespace: namespace, UID: "mg-uid"}}
	mg.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: kind, Name: providerConfigName})
	return mg
}

func namespacedProviderConfig() *v1beta1.ProviderConfig {
	return &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: providerConfigName, Namespace: namespace},
		Spec:       credentialsSpec(""),
	}
}

func TestGetConfigSuccess(t *testing.T) {
	liveSpec := credentialsSpec("")
	liveSpec.AllowLiveMoneyMovement = true
	telemetrySpec := credentialsSpec("")
	telemetrySpec.EnableTelemetry = true
	cases := map[string]struct {
		pc              client.Object
		kind            string
		secretNamespace string
		credentials     string
		want            *Config
	}{
		"ProviderConfigAllFields": {
			pc:              namespacedProviderConfig(),
			kind:            v1beta1.ProviderConfigKind,
			secretNamespace: namespace,
			credentials:     `{"apiKey":"sk_test_123","stripeAccount":"acct_123","baseURL":"http://stripe-mock:12111"}`,
			want:            &Config{APIKey: testAPIKey, StripeAccount: "acct_123", BaseURL: "http://stripe-mock:12111"},
		},
		"ProviderConfigAPIKeyOnly": {
			pc:              namespacedProviderConfig(),
			kind:            v1beta1.ProviderConfigKind,
			secretNamespace: namespace,
			credentials:     `{"apiKey":"sk_test_123"}`,
			want:            &Config{APIKey: testAPIKey},
		},
		"ProviderConfigAllowLiveMoneyMovement": {
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: providerConfigName, Namespace: namespace},
				Spec:       liveSpec,
			},
			kind:            v1beta1.ProviderConfigKind,
			secretNamespace: namespace,
			credentials:     `{"apiKey":"sk_live_123"}`,
			want:            &Config{APIKey: "sk_live_123", AllowLiveMoneyMovement: true},
		},
		"ProviderConfigEnableTelemetry": {
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: providerConfigName, Namespace: namespace},
				Spec:       telemetrySpec,
			},
			kind:            v1beta1.ProviderConfigKind,
			secretNamespace: namespace,
			credentials:     `{"apiKey":"sk_test_123"}`,
			want:            &Config{APIKey: testAPIKey, EnableTelemetry: true},
		},
		"ClusterProviderConfig": {
			pc: &v1beta1.ClusterProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: providerConfigName},
				Spec:       credentialsSpec("crossplane-system"),
			},
			kind:            v1beta1.ClusterProviderConfigKind,
			secretNamespace: "crossplane-system",
			credentials:     `{"apiKey":"sk_test_123"}`,
			want:            &Config{APIKey: testAPIKey},
		},
		"ProviderConfigSecretRefNamespaceReplacedByManagedNamespace": {
			pc: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: providerConfigName, Namespace: namespace},
				Spec:       credentialsSpec("elsewhere"),
			},
			kind:            v1beta1.ProviderConfigKind,
			secretNamespace: namespace,
			credentials:     `{"apiKey":"sk_test_123"}`,
			want:            &Config{APIKey: testAPIKey},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kube := kubeClient(t, tc.pc, tc.secretNamespace, tc.credentials)
			got, err := GetConfig(context.Background(), kube, managed(tc.kind))
			if err != nil {
				t.Fatalf("GetConfig() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GetConfig() -want +got:\n%s", diff)
			}
		})
	}
}

func TestGetConfigTracksUsage(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"apiKey":"sk_test_123"}`)
	if _, err := GetConfig(context.Background(), kube, managed(v1beta1.ProviderConfigKind)); err != nil {
		t.Fatalf("GetConfig() unexpected error: %v", err)
	}
	usages := &v1beta1.ProviderConfigUsageList{}
	if err := kube.List(context.Background(), usages, client.InNamespace(namespace)); err != nil {
		t.Fatal(err)
	}
	if len(usages.Items) != 1 {
		t.Fatalf("ProviderConfigUsage count = %d, want 1", len(usages.Items))
	}
	u := usages.Items[0]
	wantRef := xpv2.ProviderConfigReference{Kind: v1beta1.ProviderConfigKind, Name: providerConfigName}
	if diff := cmp.Diff(wantRef, u.ProviderConfigReference); diff != "" {
		t.Errorf("ProviderConfigUsage providerConfigRef -want +got:\n%s", diff)
	}
	if u.ResourceReference.Name != "customer" {
		t.Errorf("ProviderConfigUsage resourceRef.name = %q, want %q", u.ResourceReference.Name, "customer")
	}
}

func TestGetConfigCredentialErrors(t *testing.T) {
	cases := map[string]struct {
		credentials string
		want        error
	}{
		"MalformedJSON": {
			credentials: `{"apiKey":`,
			want:        ErrMalformedCredentials,
		},
		"MissingAPIKey": {
			credentials: `{"stripeAccount":"acct_123"}`,
			want:        ErrMissingAPIKey,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kube := kubeClient(t, namespacedProviderConfig(), namespace, tc.credentials)
			_, err := GetConfig(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
			if !errors.Is(err, tc.want) {
				t.Fatalf("GetConfig() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestGetConfigUnknownProviderConfigKind(t *testing.T) {
	for _, kind := range []string{"UnregisteredKind", "ProviderConfigUsage"} {
		t.Run(kind, func(t *testing.T) {
			kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"apiKey":"sk_test_123"}`)
			_, err := GetConfig(context.Background(), kube, managed(kind))
			if !errors.Is(err, ErrUnknownProviderConfigKind) {
				t.Fatalf("GetConfig() error = %v, want %v", err, ErrUnknownProviderConfigKind)
			}
		})
	}
}
