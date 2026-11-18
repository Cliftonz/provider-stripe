// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import "github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

func (d Descriptor) connectionDetails(o object) managed.ConnectionDetails {
	c := managed.ConnectionDetails{}
	for _, k := range d.SecretFields {
		if v, ok := o[k].(string); ok {
			c[k] = []byte(v)
			delete(o, k)
		}
	}
	return c
}
