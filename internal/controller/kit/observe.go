// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/form"
)

func (e *External[T]) Observe(ctx context.Context, mg T) (managed.ExternalObservation, error) {
	o, err := e.find(ctx, mg)
	if err != nil || o == nil {
		return managed.ExternalObservation{}, err
	}
	if meta.WasDeleted(mg) {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}
	return e.observeExisting(ctx, mg, o)
}

func (e *External[T]) find(ctx context.Context, mg T) (object, error) {
	id := meta.GetExternalName(mg)
	if id == "" || meta.WasDeleted(mg) && e.Descriptor.Deletion.Mode == NoOp {
		return nil, nil
	}
	o, err := e.Client.call(ctx, http.MethodGet, stripe.FormatURLPath(e.Descriptor.ItemPath, id), "", stripe.Params{})
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if o["deleted"] == true || meta.WasDeleted(mg) && e.Descriptor.Deletion.ended(o) {
		return nil, nil
	}
	return o, nil
}

func (e *External[T]) observeExisting(ctx context.Context, mg T, o object) (managed.ExternalObservation, error) {
	if err := e.exhaustLists(ctx, o); err != nil {
		return managed.ExternalObservation{}, err
	}
	details := e.Descriptor.connectionDetails(o)
	mg.SetObservation(o)
	lateInitialized, err := lateInitialize(mg.GetParameters(), o, e.Descriptor.LateInitFields)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mg.SetConditions(xpv2.Available())
	_, upToDate := e.Descriptor.diff(mg.GetParameters(), o)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate, ResourceLateInitialized: lateInitialized, ConnectionDetails: details}, nil
}

func (e *External[T]) exhaustLists(ctx context.Context, o object) error {
	for _, k := range e.Descriptor.UpdatableFields {
		l, _ := o[k].(map[string]any)
		for l["object"] == "list" && l["has_more"] == true {
			data := l["data"].([]any)
			path, query, _ := strings.Cut(l["url"].(string), "?")
			page := url.Values{"limit": {"100"}, "starting_after": {fmt.Sprint(data[len(data)-1].(map[string]any)["id"])}}.Encode()
			next, err := e.Client.call(ctx, http.MethodGet, path, strings.TrimPrefix(query+"&"+page, "&"), stripe.Params{})
			if err != nil {
				return err
			}
			l["data"] = append(data, next["data"].([]any)...)
			l["has_more"] = next["has_more"]
		}
	}
	return nil
}

func lateInitialize(params any, o object, fields []string) (bool, error) {
	initialized := false
	for _, name := range fields {
		f := formField(reflect.ValueOf(params).Elem(), name)
		if !f.IsNil() || o[name] == nil {
			continue
		}
		v := reflect.New(f.Type().Elem())
		b, err := json.Marshal(o[name])
		if err == nil {
			err = json.Unmarshal(b, v.Interface())
		}
		if err != nil {
			return false, fmt.Errorf("late-initialize %s: %w", name, err)
		}
		f.Set(v)
		initialized = true
	}
	return initialized, nil
}

func formField(v reflect.Value, name string) reflect.Value {
	return v.Field(formFieldIndex(v.Type(), name))
}

func formFieldIndex(t reflect.Type, name string) int {
	for i := range t.NumField() {
		if strings.Split(t.Field(i).Tag.Get("form"), ",")[0] == name {
			return i
		}
	}
	return -1
}

func numeric(t reflect.Type, key string) bool {
	parts := strings.Split(strings.ReplaceAll(key, "]", ""), "[")
	for i := 0; ; i++ {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		switch {
		case t.Implements(reflect.TypeFor[form.Appender]()):
			return true
		case i == len(parts):
			return t.Kind() >= reflect.Int && t.Kind() <= reflect.Float64
		case t.Kind() == reflect.Struct:
			f := formFieldIndex(t, parts[i])
			if f < 0 {
				return false
			}
			t = t.Field(f).Type
		case t.Kind() == reflect.Slice || t.Kind() == reflect.Map:
			t = t.Elem()
		default:
			return false
		}
	}
}

func (d Descriptor) diff(params any, o object) (url.Values, bool) {
	body := d.desired(params)
	got, lengths := d.observed(o)
	for k := range got {
		if strings.HasPrefix(k, "metadata[") && !body.Has(k) {
			body.Set(k, "")
		}
	}
	return body, valuesMatch(reflect.TypeOf(params), body, got) && lengthsMatch(body, lengths)
}

func (d Descriptor) desired(params any) url.Values {
	all := &form.Values{}
	form.AppendTo(all, params)
	body := url.Values{}
	for k, v := range all.ToValues() {
		if slices.Contains(d.UpdatableFields, strings.SplitN(k, "[", 2)[0]) {
			body[k] = v
		}
	}
	return body
}

func (d Descriptor) observed(o object) (map[string]string, map[string]int) {
	got, lengths := map[string]string{}, map[string]int{}
	for k, v := range o {
		if slices.Contains(d.UpdatableFields, k) {
			flatten(k, v, got, lengths)
		}
	}
	return got, lengths
}

func valuesMatch(params reflect.Type, body url.Values, got map[string]string) bool {
	for k := range body {
		if !equal(got[k], body.Get(k), numeric(params, k)) {
			return false
		}
	}
	return true
}

func lengthsMatch(body url.Values, lengths map[string]int) bool {
	for prefix, n := range lengths {
		indices := map[string]bool{}
		for k := range body {
			if rest, ok := strings.CutPrefix(k, prefix+"["); ok {
				index, _, _ := strings.Cut(rest, "]")
				indices[index] = true
			}
		}
		if (len(indices) > 0 || body.Has(prefix)) && len(indices) != n {
			return false
		}
	}
	return true
}

func flatten(key string, v any, got map[string]string, lengths map[string]int) {
	switch v := v.(type) {
	case map[string]any:
		if v["object"] == "list" {
			flatten(key, v["data"], got, lengths)
			return
		}
		if id, ok := v["id"]; ok {
			got[key] = fmt.Sprint(id)
		}
		for k, e := range v {
			flatten(key+"["+k+"]", e, got, lengths)
		}
	case []any:
		lengths[key] = len(v)
		for i, e := range v {
			flatten(key+"["+strconv.Itoa(i)+"]", e, got, lengths)
		}
	case nil:
	default:
		got[key] = fmt.Sprint(v)
	}
}

var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func equal(a, b string, numeric bool) bool {
	if numeric && jsonNumber.MatchString(a) && jsonNumber.MatchString(b) {
		x, xok := new(big.Rat).SetString(a)
		y, yok := new(big.Rat).SetString(b)
		if xok && yok {
			return x.Cmp(y) == 0
		}
	}
	return a == b
}
