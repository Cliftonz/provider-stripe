// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Cliftonz/provider-stripe/apis/v1beta1"
)

const (
	errGetProviderConfig  = "cannot get referenced ProviderConfig"
	errTrackUsage         = "cannot track ProviderConfig usage"
	errExtractCredentials = "cannot extract credentials"
)

var (
	ErrMalformedCredentials = errors.New("Stripe credentials are malformed JSON")
	ErrMissingAPIKey        = errors.New("Stripe credentials are missing apiKey")

	ErrUnknownProviderConfigKind = errors.New("providerConfigRef kind must be ProviderConfig or ClusterProviderConfig")
)

type Config struct {
	APIKey        string
	StripeAccount string
	BaseURL       string

	AllowLiveMoneyMovement bool
	EnableTelemetry        bool
}

func GetConfig(ctx context.Context, c client.Client, mg resource.ModernManaged) (*Config, error) {
	pcSpec, err := resolveProviderConfig(ctx, c, mg)
	if err != nil {
		return nil, err
	}
	data, err := resource.CommonCredentialExtractor(ctx, pcSpec.Credentials.Source, c, pcSpec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}
	var creds struct {
		APIKey        string `json:"apiKey"`
		StripeAccount string `json:"stripeAccount"`
		BaseURL       string `json:"baseURL"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedCredentials, err)
	}
	if creds.APIKey == "" {
		return nil, ErrMissingAPIKey
	}
	return &Config{
		APIKey:                 creds.APIKey,
		StripeAccount:          creds.StripeAccount,
		BaseURL:                creds.BaseURL,
		AllowLiveMoneyMovement: pcSpec.AllowLiveMoneyMovement,
		EnableTelemetry:        pcSpec.EnableTelemetry,
	}, nil
}

func resolveProviderConfig(ctx context.Context, c client.Client, mg resource.ModernManaged) (*v1beta1.ProviderConfigSpec, error) {
	configRef := mg.GetProviderConfigReference()
	var pcSpec v1beta1.ProviderConfigSpec
	switch configRef.Kind {
	case v1beta1.ProviderConfigKind:
		pc := &v1beta1.ProviderConfig{}
		if err := c.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: mg.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetProviderConfig)
		}
		pcSpec = pc.Spec
		pcSpec.Credentials.SecretRef.Namespace = mg.GetNamespace()
	case v1beta1.ClusterProviderConfigKind:
		pc := &v1beta1.ClusterProviderConfig{}
		if err := c.Get(ctx, types.NamespacedName{Name: configRef.Name}, pc); err != nil {
			return nil, errors.Wrap(err, errGetProviderConfig)
		}
		pcSpec = pc.Spec
	default:
		return nil, ErrUnknownProviderConfigKind
	}
	t := resource.NewProviderConfigUsageTracker(c, &v1beta1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}
	return &pcSpec, nil
}
