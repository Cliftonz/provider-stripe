// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package apis_test

import (
	"os"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/Cliftonz/provider-stripe/apis"
)

func TestAddToSchemeRegistersConfigKinds(t *testing.T) {
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"ProviderConfig", "ClusterProviderConfig", "ProviderConfigUsage"} {
		gvk := schema.GroupVersionKind{Group: "stripe.m.clifton.quest", Version: "v1beta1", Kind: kind}
		if !s.Recognizes(gvk) {
			t.Errorf("scheme does not recognize %s", gvk)
		}
	}
}

func TestAddToSchemeRegistersEveryCRDKind(t *testing.T) {
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../package/crds/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no CRDs found: %v", err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(b, &crd); err != nil {
			t.Fatal(err)
		}
		for _, v := range crd.Spec.Versions {
			gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: v.Name, Kind: crd.Spec.Names.Kind}
			if !s.Recognizes(gvk) {
				t.Errorf("scheme does not recognize %s", gvk)
			}
		}
	}
}
