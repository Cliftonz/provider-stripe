// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"log"
	"maps"
	"os"
	"path/filepath"

	"github.com/Cliftonz/provider-stripe/internal/generator"
)

func main() {
	specPath := flag.String("spec", "", "path to Stripe OpenAPI spec3.json")
	deleteTable := flag.String("delete-table", "", "write the delete-mode table to this path")
	out := flag.String("out", "", "write generated API type sources under this directory")
	setupOut := flag.String("setup-out", "internal/controller/zz_setup.go", "write the generated controller registration source to this path")
	examplesOut := flag.String("examples-out", "", "write generated example manifests under this directory")
	headerPath := flag.String("header", "hack/boilerplate.go.txt", "license header for generated sources")
	flag.Parse()

	spec, err := os.ReadFile(*specPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := generator.CheckEntries(spec); err != nil {
		log.Fatal(err)
	}
	kinds, err := generator.Classify(spec)
	if err != nil {
		log.Fatal(err)
	}
	if *deleteTable != "" {
		write(*deleteTable, []byte(generator.DeleteTable(kinds)))
	}
	if *out == "" {
		return
	}
	header, err := os.ReadFile(*headerPath)
	if err != nil {
		log.Fatal(err)
	}
	writeSources(spec, kinds, header, *out, *setupOut)
	if *examplesOut != "" {
		writeExamples(spec, kinds, header, *examplesOut)
	}
}

func writeSources(spec []byte, kinds []generator.Kind, header []byte, out, setupOut string) {
	files, err := generator.Types(spec, kinds, header)
	if err != nil {
		log.Fatal(err)
	}
	wiring, setup, err := generator.Wiring(spec, kinds, header)
	if err != nil {
		log.Fatal(err)
	}
	maps.Copy(files, wiring)
	groups, err := generator.Registration(kinds, header)
	if err != nil {
		log.Fatal(err)
	}
	files["zz_generated.groups.go"] = groups
	dirs, err := filepath.Glob(filepath.Join(out, "*", "v1alpha1"))
	if err != nil {
		log.Fatal(err)
	}
	for _, dir := range dirs {
		removeAll(filepath.Dir(dir))
	}
	writeTree(out, files)
	write(setupOut, setup)
}

func writeExamples(spec []byte, kinds []generator.Kind, header []byte, out string) {
	examples, err := generator.Examples(spec, kinds, header)
	if err != nil {
		log.Fatal(err)
	}
	removeAll(out)
	writeTree(out, examples)
}

func removeAll(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Fatal(err)
	}
}

func writeTree(dir string, files map[string][]byte) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		log.Fatal(err)
	}
	for name, src := range files {
		if err := root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			log.Fatal(err)
		}
		if err := root.WriteFile(name, src, 0o600); err != nil {
			log.Fatal(err)
		}
	}
	if err := root.Close(); err != nil {
		log.Fatal(err)
	}
}

func write(path string, src []byte) {
	writeTree(filepath.Dir(path), map[string][]byte{filepath.Base(path): src})
}
