// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import "github.com/crossplane/crossplane-runtime/v2/pkg/resource"

type Descriptor struct {
	CollectionPath   string
	ItemPath         string
	UpdatableFields  []string
	LateInitFields   []string
	UpdateOnlyFields []string
	Deletion         Deletion
	MovesMoney       bool
	SecretFields     []string
}

type DeleteMode string

const (
	Delete                DeleteMode = "Delete"
	Deactivate            DeleteMode = "Deactivate"
	Action                DeleteMode = "Action"
	SetField              DeleteMode = "SetField"
	DeleteDraftElseAction DeleteMode = "DeleteDraftElseAction"
	NoOp                  DeleteMode = "NoOp"
	DeleteElseDeactivate  DeleteMode = "DeleteElseDeactivate"
)

type Deletion struct {
	Mode   DeleteMode
	Action string
	Field  string
	Value  string

	EndStatuses   []string
	EndTimestamps []string
}

type Resource interface {
	resource.Managed
	GetParameters() any
	SetObservation(map[string]any)
}

type External[T Resource] struct {
	Client     *Client
	Descriptor Descriptor
}
