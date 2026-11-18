// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/Cliftonz/provider-stripe/apis/v1beta1"
)

var schemeBuilder = append(runtime.SchemeBuilder{v1beta1.SchemeBuilder.AddToScheme}, generatedGroups()...)

var AddToScheme = schemeBuilder.AddToScheme
