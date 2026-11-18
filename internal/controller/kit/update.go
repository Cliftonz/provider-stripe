// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"
	"net/http"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stripe/stripe-go/v87"
)

func (e *External[T]) Update(ctx context.Context, mg T) (managed.ExternalUpdate, error) {
	path := stripe.FormatURLPath(e.Descriptor.ItemPath, meta.GetExternalName(mg))
	o, err := e.Client.call(ctx, http.MethodGet, path, "", stripe.Params{})
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	body, _ := e.Descriptor.diff(mg.GetParameters(), o)
	_, err = e.Client.call(ctx, http.MethodPost, path, body.Encode(), stripe.Params{})
	return managed.ExternalUpdate{}, err
}
