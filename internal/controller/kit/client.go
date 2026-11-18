// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/stripe/stripe-go/v87"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

type Client struct {
	backend stripe.Backend
	config  clients.Config
}

func NewClient(c clients.Config, log logging.Logger) *Client {
	bc := &stripe.BackendConfig{LeveledLogger: debugLogger{log}, EnableTelemetry: stripe.Bool(c.EnableTelemetry)}
	if c.BaseURL != "" {
		bc.URL = stripe.String(c.BaseURL)
	}
	return &Client{backend: stripe.GetBackendWithConfig(stripe.APIBackend, bc), config: c}
}

type debugLogger struct{ log logging.Logger }

func (l debugLogger) Debugf(format string, v ...any) { l.log.Debug(fmt.Sprintf(format, v...)) }
func (l debugLogger) Infof(format string, v ...any)  { l.log.Debug(fmt.Sprintf(format, v...)) }
func (l debugLogger) Warnf(format string, v ...any)  { l.log.Debug(fmt.Sprintf(format, v...)) }
func (l debugLogger) Errorf(format string, v ...any) { l.log.Debug(fmt.Sprintf(format, v...)) }

type object map[string]any

func (object) SetLastResponse(*stripe.APIResponse) {}

func (o *object) UnmarshalJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode((*map[string]any)(o))
}

func (c *Client) call(ctx context.Context, method, path, body string, p stripe.Params) (object, error) {
	p.Context = ctx
	if c.config.StripeAccount != "" {
		p.StripeAccount = stripe.String(c.config.StripeAccount)
	}
	var o object
	err := c.backend.CallRaw(method, path, c.config.APIKey, []byte(body), &p, &o)
	return o, err
}
