// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"errors"
	"strings"
)

var ErrLiveMoneyMovement = errors.New("refusing to create a money-moving resource with a live Stripe API key: set allowLiveMoneyMovement on the ProviderConfig to permit it")

func (e *External[T]) blocksLiveMoneyMovement() bool {
	c := e.Client.config
	live := strings.HasPrefix(c.APIKey, "sk_live_") || strings.HasPrefix(c.APIKey, "rk_live_")
	return e.Descriptor.MovesMoney && live && !c.AllowLiveMoneyMovement
}
