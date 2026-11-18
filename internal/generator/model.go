// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

type DeleteMode string

const (
	Delete                DeleteMode = "Delete"
	Deactivate            DeleteMode = "Deactivate"
	Action                DeleteMode = "Action"
	SetField              DeleteMode = "SetField"
	DeleteDraftElseAction DeleteMode = "DeleteDraftElseAction"
	DeleteElseDeactivate  DeleteMode = "DeleteElseDeactivate"
	NoOp                  DeleteMode = "NoOp"
)

type Deletion struct {
	Mode   DeleteMode
	Action string
	Field  string
	Value  string

	EndStatuses   []string
	EndTimestamps []string
}

type Kind struct {
	Name             string
	Group            string
	Path             string
	ItemPath         string
	CreateFields     []string
	UpdatableFields  []string
	CreateOnlyFields []string
	UpdateOnlyFields []string
	Deletion         Deletion
}

const (
	statusCanceled  = "canceled"
	statusSucceeded = "succeeded"
	statusFailed    = "failed"
)

var excludedPaths = []string{"/v1/files", "/v1/tokens"}

var deletionOverrides = map[string]Deletion{
	"/v1/apps/installs":       {Mode: Action, Action: "uninstall"},
	"/v1/invoices":            {Mode: DeleteDraftElseAction, Action: "void"},
	"/v1/products":            {Mode: DeleteElseDeactivate},
	"/v1/issuing/cardholders": {Mode: SetField, Field: "status", Value: "inactive"},
	"/v1/issuing/cards":       {Mode: SetField, Field: "status", Value: statusCanceled},
}

var endStates = map[string]Deletion{
	"/v1/apps/installs":                  {EndStatuses: []string{"install_failed", "uninstalling"}},
	"/v1/billing/alerts":                 {EndStatuses: []string{"archived", "inactive"}},
	"/v1/billing/credit_grants":          {EndTimestamps: []string{"voided_at", "expires_at"}},
	"/v1/billing/feedback_options":       {EndStatuses: []string{"inactive"}},
	"/v1/billing/meters":                 {EndStatuses: []string{"inactive"}},
	"/v1/checkout/sessions":              {EndStatuses: []string{"complete", "expired"}},
	"/v1/climate/orders":                 {EndStatuses: []string{statusCanceled, "delivered"}},
	"/v1/credit_notes":                   {EndStatuses: []string{"void"}},
	"/v1/identity/verification_sessions": {EndStatuses: []string{statusCanceled, "verified"}},
	"/v1/invoices":                       {EndStatuses: []string{"paid", "void"}},
	"/v1/payment_intents":                {EndStatuses: []string{statusCanceled, statusSucceeded}},
	"/v1/payouts":                        {EndStatuses: []string{statusCanceled, statusFailed, "paid"}},
	"/v1/quotes":                         {EndStatuses: []string{"accepted", statusCanceled}},
	"/v1/refunds":                        {EndStatuses: []string{statusCanceled, statusFailed, statusSucceeded}},
	"/v1/setup_intents":                  {EndStatuses: []string{statusCanceled, statusSucceeded}},
	"/v1/subscription_schedules":         {EndStatuses: []string{statusCanceled, "completed", "released"}},
	"/v1/three_d_secure/authentications": {EndStatuses: []string{statusCanceled, "error", statusFailed, statusSucceeded}},
	"/v1/topups":                         {EndStatuses: []string{statusCanceled, statusFailed, "reversed", statusSucceeded}},
	"/v1/treasury/financial_accounts":    {EndStatuses: []string{"closed"}},
	"/v1/treasury/inbound_transfers":     {EndStatuses: []string{statusCanceled, statusFailed, statusSucceeded}},
	"/v1/treasury/outbound_payments":     {EndStatuses: []string{statusCanceled, statusFailed, "posted", "returned"}},
	"/v1/treasury/outbound_transfers":    {EndStatuses: []string{statusCanceled, statusFailed, "posted", "returned"}},
}

var lifecycleActions = []string{"cancel", "expire", "void", "deactivate", "close"}

var collectionPath = regexp.MustCompile(`^/v1(/[a-z_]+)+$`)

const itemParam = `/\{[^}/]+\}`

const formContentType = "application/x-www-form-urlencoded"

type operation struct {
	RequestBody struct {
		Content map[string]struct {
			Schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

type pathItem struct {
	Get    *operation `json:"get"`
	Post   *operation `json:"post"`
	Delete *operation `json:"delete"`
}

func Classify(spec []byte) ([]Kind, error) {
	var doc struct {
		Paths map[string]pathItem `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	var kinds []Kind
	for path, collection := range doc.Paths {
		if collection.Post == nil || !collectionPath.MatchString(path) || slices.Contains(excludedPaths, path) {
			continue
		}
		itemPaths := matchPaths(doc.Paths, regexp.QuoteMeta(path)+itemParam)
		if len(itemPaths) > 1 {
			return nil, fmt.Errorf("%s: multiple item paths %v", path, itemPaths)
		}
		if len(itemPaths) == 0 || doc.Paths[itemPaths[0]].Get == nil {
			continue
		}
		k, err := classifyKind(doc.Paths, path, itemPaths[0])
		if err != nil {
			return nil, err
		}
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(a, b Kind) int { return strings.Compare(a.Path, b.Path) })
	return kinds, nil
}

func classifyKind(paths map[string]pathItem, path, itemPath string) (Kind, error) {
	segments := strings.Split(strings.TrimPrefix(path, "/v1/"), "/")
	name, err := pascal(strings.TrimSuffix(segments[len(segments)-1], "s"))
	if err != nil {
		return Kind{}, fmt.Errorf("%s: %w", path, err)
	}
	createFields, err := fields(path, paths[path].Post)
	if err != nil {
		return Kind{}, err
	}
	k := Kind{
		Name:         name,
		Group:        "core",
		Path:         path,
		ItemPath:     itemPath,
		CreateFields: createFields,
	}
	if len(segments) > 1 {
		k.Group = strings.ReplaceAll(segments[0], "_", "")
	}
	item := paths[itemPath]
	if item.Post != nil {
		if k.UpdatableFields, err = fields(itemPath, item.Post); err != nil {
			return Kind{}, err
		}
	}
	k.CreateOnlyFields = difference(k.CreateFields, k.UpdatableFields)
	k.UpdateOnlyFields = difference(k.UpdatableFields, k.CreateFields)
	k.Deletion, err = deletion(paths, path, item, k.UpdatableFields)
	return k, err
}

func deletion(paths map[string]pathItem, path string, item pathItem, updatable []string) (Deletion, error) {
	d, err := deletionMode(paths, path, item, updatable)
	if err != nil || (d.Mode != Action && d.Mode != DeleteDraftElseAction) {
		return d, err
	}
	end, ok := endStates[path]
	if !ok {
		return Deletion{}, fmt.Errorf("%s: no end state for %s deletion", path, d.Mode)
	}
	d.EndStatuses, d.EndTimestamps = end.EndStatuses, end.EndTimestamps
	return d, nil
}

func deletionMode(paths map[string]pathItem, path string, item pathItem, updatable []string) (Deletion, error) {
	if d, ok := deletionOverrides[path]; ok {
		return d, nil
	}
	if item.Delete != nil {
		return Deletion{Mode: Delete}, nil
	}
	if slices.Contains(updatable, "active") {
		return Deletion{Mode: Deactivate}, nil
	}
	for _, action := range lifecycleActions {
		var posts []string
		for _, p := range matchPaths(paths, regexp.QuoteMeta(path)+itemParam+"/"+action) {
			if paths[p].Post != nil {
				posts = append(posts, p)
			}
		}
		if len(posts) > 1 {
			return Deletion{}, fmt.Errorf("%s: multiple %s action paths %v", path, action, posts)
		}
		if len(posts) == 1 {
			return Deletion{Mode: Action, Action: action}, nil
		}
	}
	return Deletion{Mode: NoOp}, nil
}

func matchPaths(paths map[string]pathItem, pattern string) []string {
	re := regexp.MustCompile("^" + pattern + "$")
	var matches []string
	for p := range paths {
		if re.MatchString(p) {
			matches = append(matches, p)
		}
	}
	slices.Sort(matches)
	return matches
}

func fields(path string, op *operation) ([]string, error) {
	form, ok := op.RequestBody.Content[formContentType]
	if !ok || form.Schema.Properties == nil {
		return nil, fmt.Errorf("%s: request body is not inline %s properties", path, formContentType)
	}
	delete(form.Schema.Properties, "expand")
	return slices.Sorted(maps.Keys(form.Schema.Properties)), nil
}

func difference(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}

func pascal(snake string) (string, error) {
	var b strings.Builder
	for _, word := range strings.Split(snake, "_") {
		if word == "" {
			return "", fmt.Errorf("empty word in name %q", snake)
		}
		b.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return b.String(), nil
}

func DeleteTable(kinds []Kind) string {
	var b strings.Builder
	b.WriteString("| kind | path | group | delete mode | action | field | value | end statuses | end timestamps |\n|------|------|-------|-------------|--------|-------|-------|--------------|----------------|\n")
	for _, k := range kinds {
		d := k.Deletion
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", k.Name, k.Path, k.Group, d.Mode, d.Action, d.Field, d.Value, strings.Join(d.EndStatuses, ", "), strings.Join(d.EndTimestamps, ", "))
	}
	return b.String()
}

func CheckEntries(spec []byte) error {
	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return err
	}
	var missing []string
	for _, p := range slices.Concat(excludedPaths, slices.Collect(maps.Keys(deletionOverrides)), slices.Collect(maps.Keys(endStates)), slices.Collect(maps.Values(references)), slices.Collect(maps.Keys(referenceExclusions))) {
		if _, ok := doc.Paths[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		missing = slices.Compact(missing)
		return fmt.Errorf("entries missing from spec: %v", missing)
	}
	return nil
}
