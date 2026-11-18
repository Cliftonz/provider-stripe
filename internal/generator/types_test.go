// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func load(t *testing.T, name string) ([]byte, []Kind, []byte) {
	t.Helper()
	spec, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	kinds, err := Classify(spec)
	if err != nil {
		t.Fatal(err)
	}
	header, err := os.ReadFile("../../hack/boilerplate.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	return spec, kinds, header
}

func generateTypes(t *testing.T, name string) (map[string][]byte, error) {
	t.Helper()
	return Types(load(t, name))
}

func TestTypes(t *testing.T) {
	got, err := generateTypes(t, "testdata/types/spec.json")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(readTree(t, "testdata/types/golden"), text(got)); diff != "" {
		t.Errorf("Types (-want +got):\n%s", diff)
	}
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func text(files map[string][]byte) map[string]string {
	out := map[string]string{}
	for name, b := range files {
		out[name] = string(b)
	}
	return out
}

func TestTypesRejectsUnexpectedShapes(t *testing.T) {
	cases := map[string]string{
		"request-shape.json":     "/v1/widgets: settings.mode",
		"response-shape.json":    "/v1/widgets/{id}: mode",
		"map-enum.json":          "/v1/widgets: modes",
		"bare-object.json":       "/v1/widgets: settings",
		"reference-type.json":    "/v1/widgets: product",
		"response-root.json":     "/v1/widgets/{id}: response schema",
		"additional-true.json":   "/v1/widgets: settings",
		"update-only-shape.json": "/v1/widgets/{id}: mode",
		"reference-target.json":  "/v1/widgets: customer: reference target /v1/customers",
	}
	for file, path := range cases {
		t.Run(file, func(t *testing.T) {
			_, err := generateTypes(t, "testdata/types/invalid/"+file)
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Errorf("Types error = %v, want error naming %s", err, path)
			}
		})
	}
}
