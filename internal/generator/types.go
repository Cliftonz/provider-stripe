// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"strings"
)

const (
	apisModule  = "github.com/Cliftonz/provider-stripe/apis"
	groupSuffix = ".stripe.m.clifton.quest"
	version     = "v1alpha1"
	jsonType    = "JSON"
)

var references = map[string]string{
	"cardholder":                   "/v1/issuing/cardholders",
	"charge":                       "/v1/charges",
	"customer":                     "/v1/customers",
	"default_payment_method":       "/v1/payment_methods",
	"default_tax_rates":            "/v1/tax_rates",
	"financial_account":            "/v1/treasury/financial_accounts",
	"from_subscription":            "/v1/subscriptions",
	"invoice":                      "/v1/invoices",
	"location":                     "/v1/terminal/locations",
	"on_behalf_of":                 "/v1/accounts",
	"payment_intent":               "/v1/payment_intents",
	"payment_method":               "/v1/payment_methods",
	"payment_method_configuration": "/v1/payment_method_configurations",
	"personalization_design":       "/v1/issuing/personalization_designs",
	"price":                        "/v1/prices",
	"product":                      "/v1/products",
	"subscription":                 "/v1/subscriptions",
	"tax_rates":                    "/v1/tax_rates",
	"test_clock":                   "/v1/test_helpers/test_clocks",
	"value_list":                   "/v1/radar/value_lists",
}

var referenceExclusions = map[string][]string{
	"/v1/climate/orders":           {"product"},
	"/v1/plans":                    {"product"},
	"/v1/test_helpers/test_clocks": {"customer"},
}

var cardSecretFields = map[string][]string{"/v1/issuing/cards": {"cvc", "number"}}

func secretFields(path string, object *schema) []string {
	fields := slices.Clone(cardSecretFields[path])
	for _, name := range []string{"client_secret", "secret"} {
		if object.Properties[name] != nil {
			fields = append(fields, name)
		}
	}
	slices.Sort(fields)
	return fields
}

type schema struct {
	Type                 string               `json:"type"`
	Description          string               `json:"description"`
	Nullable             bool                 `json:"nullable"`
	Enum                 []any                `json:"enum"`
	Properties           map[string]*schema   `json:"properties"`
	AdditionalProperties additionalProperties `json:"additionalProperties"`
	Items                *schema              `json:"items"`
	AnyOf                []*schema            `json:"anyOf"`
	Required             []string             `json:"required"`
	Ref                  string               `json:"$ref"`
}

type additionalProperties struct {
	Schema *schema
}

func (a *additionalProperties) UnmarshalJSON(b []byte) error {
	if b[0] != '{' {
		return nil
	}
	return json.Unmarshal(b, &a.Schema)
}

type content map[string]struct {
	Schema *schema `json:"schema"`
}

type typesDoc struct {
	Paths map[string]struct {
		Get *struct {
			Responses map[string]struct {
				Content content `json:"content"`
			} `json:"responses"`
		} `json:"get"`
		Post *struct {
			RequestBody struct {
				Content content `json:"content"`
			} `json:"requestBody"`
		} `json:"post"`
	} `json:"paths"`
	Components struct {
		Schemas map[string]*schema `json:"schemas"`
	} `json:"components"`
}

type goField struct {
	Name     string
	Type     string
	JSON     string
	Form     string
	Doc      string
	Markers  []string
	Required bool
}

type goStruct struct {
	Name    string
	Markers []string
	Fields  []goField
}

type kindGen struct {
	path     string
	structs  []goStruct
	usesJSON bool
}

func Types(spec []byte, kinds []Kind, header []byte) (map[string][]byte, error) {
	var doc typesDoc
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	byPath := map[string]Kind{}
	groups := map[string][]string{}
	for _, k := range kinds {
		byPath[k.Path] = k
		groups[k.Group] = append(groups[k.Group], k.Name)
	}
	files := map[string][]byte{}
	jsonGroups := map[string]bool{}
	for _, k := range kinds {
		src, usesJSON, err := kindSource(doc, k, byPath, header)
		if err != nil {
			return nil, err
		}
		jsonGroups[k.Group] = jsonGroups[k.Group] || usesJSON
		files[k.Group+"/"+version+"/"+strings.ToLower(k.Name)+".go"] = src
	}
	for group, names := range groups {
		src, err := format.Source([]byte(registerSource(group, names, header)))
		if err != nil {
			return nil, err
		}
		files[group+"/"+version+"/register.go"] = src
		if jsonGroups[group] {
			files[group+"/"+version+"/json.go"] = []byte(string(header) + jsonSource)
		}
	}
	return files, nil
}

func kindSource(doc typesDoc, k Kind, byPath map[string]Kind, header []byte) ([]byte, bool, error) {
	g := &kindGen{path: k.Path}
	params, err := g.parameters(doc, k, byPath)
	if err != nil {
		return nil, false, err
	}
	g.structs = append([]goStruct{params}, g.structs...)

	object, err := responseObject(doc, k.ItemPath)
	if err != nil {
		return nil, false, err
	}
	obs, err := g.observations(k, object)
	if err != nil {
		return nil, false, err
	}
	g.structs = append(g.structs, obs)

	var b strings.Builder
	b.Write(header)
	b.WriteString("\n// Code generated by cmd/generator. DO NOT EDIT.\n\npackage " + version + "\n\nimport (\n")
	b.WriteString("metav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n\nxpv2 \"github.com/crossplane/crossplane/apis/v2/core/v2\"\n)\n")
	for _, s := range g.structs {
		writeStruct(&b, s)
	}
	writeKind(&b, k.Name, object.Description)
	src, err := format.Source([]byte(b.String()))
	return src, g.usesJSON, err
}

func paramSchema(doc typesDoc, k Kind, name string) (*schema, string) {
	if s, ok := doc.Paths[k.Path].Post.RequestBody.Content[formContentType].Schema.Properties[name]; ok {
		return s, k.Path
	}
	return doc.Paths[k.ItemPath].Post.RequestBody.Content[formContentType].Schema.Properties[name], k.ItemPath
}

func (g *kindGen) parameters(doc typesDoc, k Kind, byPath map[string]Kind) (goStruct, error) {
	params := goStruct{Name: k.Name + "Parameters"}
	for _, name := range slices.Sorted(slices.Values(append(slices.Clone(k.CreateFields), k.UpdateOnlyFields...))) {
		fields, structMarker, err := g.paramFields(doc, k, byPath, params.Name, name)
		if err != nil {
			return goStruct{}, err
		}
		params.Fields = append(params.Fields, fields...)
		if structMarker != "" {
			params.Markers = append(params.Markers, structMarker)
		}
	}
	return params, nil
}

func (g *kindGen) paramFields(doc typesDoc, k Kind, byPath map[string]Kind, parent, name string) ([]goField, string, error) {
	var s *schema
	s, g.path = paramSchema(doc, k, name)
	create := doc.Paths[k.Path].Post.RequestBody.Content[formContentType].Schema
	f, err := g.param(s, parent, name, name, slices.Contains(create.Required, name))
	if err != nil || f.Type == presenceType {
		return nil, "", err
	}
	if slices.Contains(k.CreateOnlyFields, name) && !isJSONType(f.Type) {
		f.Markers = append(f.Markers, fmt.Sprintf(`+kubebuilder:validation:XValidation:rule="self == oldSelf",message="%s is immutable"`, f.JSON))
	}
	target, isRef := references[name]
	if !isRef || slices.Contains(referenceExclusions[k.Path], name) {
		return []goField{f}, "", nil
	}
	targetKind, ok := byPath[target]
	if !ok {
		return nil, "", fmt.Errorf("%s: %s: reference target %s is not a generated kind", k.Path, name, target)
	}
	return reference(k.Path, f, targetKind)
}

func isJSONType(t string) bool {
	return t == jsonType || strings.HasSuffix(t, "*"+jsonType) || strings.HasSuffix(t, "]"+jsonType)
}

func (g *kindGen) observations(k Kind, object *schema) (goStruct, error) {
	obs := goStruct{Name: k.Name + "Observation"}
	secrets := secretFields(k.Path, object)
	for _, name := range slices.Sorted(maps.Keys(object.Properties)) {
		if slices.Contains(secrets, name) {
			continue
		}
		f, err := g.observation(object.Properties[name], k.ItemPath, name)
		if err != nil {
			return goStruct{}, err
		}
		obs.Fields = append(obs.Fields, f)
	}
	return obs, nil
}

func responseObject(doc typesDoc, itemPath string) (*schema, error) {
	s := doc.Paths[itemPath].Get.Responses["200"].Content["application/json"].Schema
	branches := append([]*schema{s}, s.AnyOf...)
	var names []string
	for _, b := range branches {
		name := strings.TrimPrefix(b.Ref, "#/components/schemas/")
		if name != "" && !strings.HasPrefix(name, "deleted_") {
			names = append(names, name)
		}
	}
	if len(names) != 1 {
		return nil, fmt.Errorf("%s: response schema is not one object reference", itemPath)
	}
	return doc.Components.Schemas[names[0]], nil
}

func reference(path string, f goField, target Kind) ([]goField, string, error) {
	refType := "*xpv2.NamespacedReference"
	refName := f.Name + "Ref"
	switch f.Type {
	case stringType, "*" + stringType:
	case "[]string":
		refType = "[]xpv2.NamespacedReference"
		refName = f.Name + "Refs"
	default:
		return nil, "", fmt.Errorf("%s: %s: reference field has type %s", path, f.Form, f.Type)
	}
	refJSON := f.JSON + strings.TrimPrefix(refName, f.Name)
	selJSON := f.JSON + "Selector"
	f.Markers = append([]string{fmt.Sprintf("+crossplane:generate:reference:type=%s/%s/%s.%s", apisModule, target.Group, version, target.Name)}, f.Markers...)
	var structMarker string
	if f.Required {
		structMarker = fmt.Sprintf(`+kubebuilder:validation:XValidation:rule="has(self.%s) || has(self.%s) || has(self.%s)",message="%s, %s or %s is required"`, f.JSON, refJSON, selJSON, f.JSON, refJSON, selJSON)
		f.Required = false
	}
	return []goField{
		f,
		{Name: refName, Type: refType, JSON: refJSON, Form: "-"},
		{Name: f.Name + "Selector", Type: "*xpv2.NamespacedSelector", JSON: selJSON, Form: "-"},
	}, structMarker, nil
}

func (g *kindGen) param(s *schema, parent, name, at string, required bool) (goField, error) {
	goName, jsonName, err := names(name)
	if err != nil {
		return goField{}, fmt.Errorf("%s: %s: %w", g.path, at, err)
	}
	typ, markers, err := g.paramType(s, parent+goName, at)
	if err != nil {
		return goField{}, err
	}
	if typ == presenceType {
		return goField{Type: presenceType}, nil
	}
	if !required && (typ == jsonType || !strings.HasPrefix(typ, "[]") && !strings.HasPrefix(typ, "map[")) {
		typ = "*" + typ
	}
	return goField{Name: goName, Type: typ, JSON: jsonName, Form: name, Doc: s.Description, Markers: markers, Required: required}, nil
}

func (g *kindGen) paramType(s *schema, typeName, at string) (string, []string, error) {
	if len(s.AnyOf) == 2 && isEmptyString(s.AnyOf[1]) {
		return g.paramType(s.AnyOf[0], typeName, at)
	}
	if len(s.AnyOf) == 2 && isEmptyString(s.AnyOf[0]) {
		return g.paramType(s.AnyOf[1], typeName, at)
	}
	if len(s.AnyOf) > 0 {
		g.usesJSON = true
		return jsonType, nil, nil
	}
	switch s.Type {
	case stringType, "integer", "number", "boolean":
		return scalarTypes[s.Type], enumMarkers(s), nil
	case "array":
		return g.arrayParamType(s, typeName, at)
	case objectType:
		return g.objectParamType(s, typeName, at)
	}
	return "", nil, fmt.Errorf("%s: %s: unexpected schema shape", g.path, at)
}

func (g *kindGen) arrayParamType(s *schema, typeName, at string) (string, []string, error) {
	elem, markers, err := g.paramType(s.Items, typeName, at+"[]")
	if err != nil {
		return "", nil, err
	}
	for i, m := range markers {
		markers[i] = strings.Replace(m, "+kubebuilder:validation:", "+kubebuilder:validation:items:", 1)
	}
	return "[]" + elem, markers, nil
}

func (g *kindGen) objectParamType(s *schema, typeName, at string) (string, []string, error) {
	if len(s.Properties) > 0 {
		return typeName, nil, g.addStruct(s, typeName, at)
	}
	if s.Properties != nil {
		return presenceType, nil, nil
	}
	value := s.AdditionalProperties.Schema
	if value == nil {
		return "", nil, fmt.Errorf("%s: %s: object without properties or an additionalProperties schema", g.path, at)
	}
	elem, markers, err := g.paramType(value, typeName, at+"{}")
	if err != nil {
		return "", nil, err
	}
	if len(markers) > 0 {
		return "", nil, fmt.Errorf("%s: %s: map values with validation markers", g.path, at)
	}
	return "map[string]" + elem, nil, nil
}

func (g *kindGen) addStruct(s *schema, typeName, at string) error {
	st := goStruct{Name: typeName}
	g.structs = append(g.structs, st)
	idx := len(g.structs) - 1
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		f, err := g.param(s.Properties[name], typeName, name, at+"."+name, slices.Contains(s.Required, name))
		if err != nil {
			return err
		}
		if f.Type == presenceType {
			continue
		}
		st.Fields = append(st.Fields, f)
	}
	g.structs[idx] = st
	return nil
}

func (g *kindGen) observation(s *schema, itemPath, name string) (goField, error) {
	goName, jsonName, err := names(name)
	if err != nil {
		return goField{}, fmt.Errorf("%s: %s: %w", itemPath, name, err)
	}
	f := goField{Name: goName, JSON: jsonName, Doc: s.Description}
	switch {
	case scalarTypes[s.Type] != "":
		f.Type = "*" + scalarTypes[s.Type]
	case isStringMap(s):
		f.Type = "map[string]string"
	case isExpandable(s):
		f.Type = "*string"
	case isFreeForm(s):
		g.usesJSON = true
		f.Type = "*" + jsonType
	default:
		return goField{}, fmt.Errorf("%s: %s: unexpected schema shape", itemPath, name)
	}
	return f, nil
}

func isStringMap(s *schema) bool {
	return s.Type == objectType && len(s.Properties) == 0 && s.AdditionalProperties.Schema != nil && s.AdditionalProperties.Schema.Type == stringType
}

func isFreeForm(s *schema) bool {
	return s.Type == objectType || s.Type == "array" || s.Ref != "" || len(s.AnyOf) > 0
}

const (
	stringType = "string"
	objectType = "object"
)

var scalarTypes = map[string]string{stringType: stringType, "integer": "int64", "number": "float64", "boolean": "bool"}

const presenceType = "presence"

var initialisms = []string{"api", "http", "https", "id", "ip", "json", "uri", "url", "uuid"}

func names(snake string) (string, string, error) {
	pascalName, err := pascal(snake)
	if err != nil {
		return "", "", err
	}
	var b strings.Builder
	for _, word := range strings.Split(snake, "_") {
		if slices.Contains(initialisms, word) {
			b.WriteString(strings.ToUpper(word))
		} else {
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return b.String(), lowerFirst(pascalName), nil
}

func isEmptyString(s *schema) bool {
	return s.Type == stringType && len(s.Enum) == 1 && s.Enum[0] == ""
}

func isExpandable(s *schema) bool {
	if len(s.AnyOf) == 0 || s.AnyOf[0].Type != stringType {
		return false
	}
	for _, branch := range s.AnyOf[1:] {
		if branch.Ref == "" {
			return false
		}
	}
	return true
}

func enumMarkers(s *schema) []string {
	if len(s.Enum) == 0 {
		return nil
	}
	values := make([]string, len(s.Enum))
	for i, v := range s.Enum {
		values[i] = fmt.Sprintf("%#v", v)
	}
	return []string{"+kubebuilder:validation:Enum=" + strings.Join(values, ";")}
}

func lowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func writeStruct(b *strings.Builder, s goStruct) {
	b.WriteString("\n")
	for _, m := range s.Markers {
		b.WriteString("// " + m + "\n")
	}
	b.WriteString("type " + s.Name + " struct {\n")
	for i, f := range s.Fields {
		if i > 0 {
			b.WriteString("\n")
		}
		writeComment(b, f.Doc)
		for _, m := range f.Markers {
			b.WriteString("// " + m + "\n")
		}
		jsonTag := f.JSON
		if !f.Required {
			jsonTag += ",omitempty"
		}
		tag := fmt.Sprintf(`json:"%s"`, jsonTag)
		if f.Form != "" {
			if !f.Required {
				b.WriteString("// +optional\n")
			}
			tag += fmt.Sprintf(` form:"%s"`, f.Form)
		}
		fmt.Fprintf(b, "%s %s `%s`\n", f.Name, f.Type, tag)
	}
	b.WriteString("}\n")
}

var dashes = strings.NewReplacer(" \xe2\x80\x94 ", ", ", "\xe2\x80\x94", ",", "\xe2\x80\x93", "-")

func writeComment(b *strings.Builder, text string) {
	if text == "" {
		return
	}
	for _, line := range strings.Split(dashes.Replace(text), "\n") {
		b.WriteString(strings.TrimRight("// "+line, " ") + "\n")
	}
}

func writeKind(b *strings.Builder, name, description string) {
	fmt.Fprintf(b, `
type %[1]sSpec struct {
	xpv2.ManagedResourceSpec `+"`json:\",inline\"`"+`
	ForProvider %[1]sParameters `+"`json:\"forProvider\"`"+`
}

type %[1]sStatus struct {
	xpv2.ManagedResourceStatus `+"`json:\",inline\"`"+`
	AtProvider %[1]sObservation `+"`json:\"atProvider,omitempty\"`"+`
}

// +kubebuilder:object:root=true

`, name)
	writeComment(b, description)
	fmt.Fprintf(b, `// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,stripe},path=%[2]s
type %[1]s struct {
	metav1.TypeMeta `+"`json:\",inline\"`"+`
	metav1.ObjectMeta `+"`json:\"metadata,omitempty\"`"+`

	Spec %[1]sSpec `+"`json:\"spec\"`"+`
	Status %[1]sStatus `+"`json:\"status,omitempty\"`"+`
}

// +kubebuilder:object:root=true

type %[1]sList struct {
	metav1.TypeMeta `+"`json:\",inline\"`"+`
	metav1.ListMeta `+"`json:\"metadata,omitempty\"`"+`
	Items []%[1]s `+"`json:\"items\"`"+`
}
`, name, plural(strings.ToLower(name)))
}

func registerSource(group string, names []string, header []byte) string {
	slices.Sort(names)
	known := make([]string, 0, 2*len(names))
	for _, n := range names {
		known = append(known, "&"+n+"{}", "&"+n+"List{}")
	}
	return fmt.Sprintf(`%s
// Code generated by cmd/generator. DO NOT EDIT.

// +kubebuilder:object:generate=true
// +groupName=%[2]s
// +versionName=%[3]s
package %[3]s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group   = %[2]q
	Version = %[3]q
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = runtime.NewSchemeBuilder()
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, %[4]s)
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	})
}
`, header, group+groupSuffix, version, strings.Join(known, ", "))
}

const jsonSource = `
// Code generated by cmd/generator. DO NOT EDIT.

package v1alpha1

import (
	"bytes"
	"encoding/json"

	"github.com/stripe/stripe-go/v87/form"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// +kubebuilder:validation:Type=""
// +kubebuilder:pruning:PreserveUnknownFields
type JSON struct {
	apiextensionsv1.JSON ` + "`json:\",inline\" form:\"-\"`" + `
}

func (j JSON) AppendTo(values *form.Values, keyParts []string) {
	d := json.NewDecoder(bytes.NewReader(j.Raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		panic(err)
	}
	form.AppendToPrefixed(values, v, keyParts)
}
`

func plural(s string) string {
	switch {
	case strings.HasSuffix(s, "s"), strings.HasSuffix(s, "x"), strings.HasSuffix(s, "ch"), strings.HasSuffix(s, "sh"):
		return s + "es"
	case strings.HasSuffix(s, "y") && len(s) > 1 && !strings.ContainsRune("aeiou", rune(s[len(s)-2])):
		return s[:len(s)-1] + "ies"
	}
	return s + "s"
}
