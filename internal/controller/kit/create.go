// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/form"
)

func (e *External[T]) Create(ctx context.Context, mg T) (managed.ExternalCreation, error) {
	if e.blocksLiveMoneyMovement() {
		return managed.ExternalCreation{}, ErrLiveMoneyMovement
	}
	values := &form.Values{}
	form.AppendTo(values, mg.GetParameters())
	var body []string
	for _, pair := range strings.Split(values.Encode(), "&") {
		key, _, _ := strings.Cut(pair, "=")
		top, _, _ := strings.Cut(key, "[")
		if !slices.Contains(e.Descriptor.UpdateOnlyFields, top) {
			body = append(body, pair)
		}
	}
	o, err := e.Client.call(ctx, http.MethodPost, e.Descriptor.CollectionPath, strings.Join(body, "&"), stripe.Params{IdempotencyKey: stripe.String(string(mg.GetUID()))})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, o["id"].(string))
	return managed.ExternalCreation{ConnectionDetails: e.Descriptor.connectionDetails(o)}, nil
}
