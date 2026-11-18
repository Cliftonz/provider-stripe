// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import "encoding/json"

func Observe(o map[string]any, into any, keys map[string]string) {
	renamed := map[string]any{}
	for from, to := range keys {
		renamed[to] = o[from]
	}
	b, err := json.Marshal(renamed)
	if err == nil {
		err = json.Unmarshal(b, into)
	}
	if err != nil {
		panic(err)
	}
}
