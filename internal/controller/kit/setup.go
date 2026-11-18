// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

type modernResource interface {
	Resource
	resource.ModernManaged
}

func Setup[T modernResource](mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, mg T, list resource.ManagedList, d Descriptor) error {
	name := managed.ControllerName(gvk.GroupKind().String())
	log := o.Logger.WithValues("controller", name)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[T](connector[T](mgr.GetClient(), d, log)),
		managed.WithInitializers(),
		managed.WithLogger(log),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // crossplane-runtime has not adopted the new events API
		managed.WithMetricRecorder(o.MetricOptions.MRMetrics),
	}
	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}
	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if err := mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, list, o.MetricOptions.PollStateMetricInterval)); err != nil {
		return err
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(gvk), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(mg).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func SetupGated[T modernResource](mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, mg T, list resource.ManagedList, d Descriptor) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, gvk, mg, list, d); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconciler", "gvk", gvk.String())
		}
	}, gvk)
	return nil
}

func connector[T modernResource](kube client.Client, d Descriptor, log logging.Logger) managed.TypedExternalConnectorFn[T] {
	return func(ctx context.Context, mg T) (managed.TypedExternalClient[T], error) {
		c, err := clients.GetConfig(ctx, kube, mg)
		if err != nil {
			return nil, err
		}
		return &External[T]{Client: NewClient(*c, log), Descriptor: d}, nil
	}
}

func (e *External[T]) Disconnect(context.Context) error {
	return nil
}
