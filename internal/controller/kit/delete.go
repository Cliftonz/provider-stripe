// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stripe/stripe-go/v87"
)

func (e *External[T]) Delete(ctx context.Context, mg T) (managed.ExternalDelete, error) {
	err := e.remove(ctx, meta.GetExternalName(mg))
	if notFound(err) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, err
}

const deactivateBody = "active=false"

func (d Deletion) ended(o object) bool {
	switch d.Mode {
	case Deactivate, DeleteElseDeactivate:
		return o["active"] == false
	case SetField:
		return o[d.Field] == d.Value
	case Action, DeleteDraftElseAction:
		return slices.Contains(d.EndStatuses, fmt.Sprint(o["status"])) || d.endTimestampPassed(o)
	case Delete, NoOp:
		return false
	default:
		return false
	}
}

func (d Deletion) endTimestampPassed(o object) bool {
	for _, f := range d.EndTimestamps {
		n, _ := o[f].(json.Number)
		if t, err := n.Int64(); err == nil && t <= time.Now().Unix() {
			return true
		}
	}
	return false
}

func notFound(err error) bool {
	var serr *stripe.Error
	return errors.As(err, &serr) && serr.HTTPStatusCode == http.StatusNotFound
}

func (e *External[T]) remove(ctx context.Context, id string) error {
	d := e.Descriptor.Deletion
	item := stripe.FormatURLPath(e.Descriptor.ItemPath, id)
	switch d.Mode {
	case Delete:
		return e.send(ctx, http.MethodDelete, item, "")
	case Deactivate:
		return e.send(ctx, http.MethodPost, item, deactivateBody)
	case Action:
		return e.send(ctx, http.MethodPost, item+"/"+d.Action, "")
	case SetField:
		return e.send(ctx, http.MethodPost, item, url.Values{d.Field: {d.Value}}.Encode())
	case DeleteDraftElseAction:
		return e.deleteDraftElseAction(ctx, item, d.Action)
	case DeleteElseDeactivate:
		return e.deleteElseDeactivate(ctx, item)
	case NoOp:
		return nil
	}
	panic(fmt.Sprintf("kit: unknown deletion mode %q", d.Mode))
}

func (e *External[T]) deleteDraftElseAction(ctx context.Context, item, action string) error {
	o, err := e.Client.call(ctx, http.MethodGet, item, "", stripe.Params{})
	if err != nil {
		return err
	}
	if o["status"] == "draft" {
		return e.send(ctx, http.MethodDelete, item, "")
	}
	return e.send(ctx, http.MethodPost, item+"/"+action, "")
}

func (e *External[T]) deleteElseDeactivate(ctx context.Context, item string) error {
	if err := e.send(ctx, http.MethodDelete, item, ""); err == nil || notFound(err) {
		return err
	}
	return e.send(ctx, http.MethodPost, item, deactivateBody)
}

func (e *External[T]) send(ctx context.Context, method, path, body string) error {
	_, err := e.Client.call(ctx, method, path, body, stripe.Params{})
	return err
}
