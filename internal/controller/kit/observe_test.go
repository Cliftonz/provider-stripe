// SPDX-FileCopyrightText: 2026 The provider-stripe authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Cliftonz/provider-stripe/internal/clients"
)

func observe(t *testing.T, status int, body, externalName string) (managed.ExternalObservation, *product, []request, error) {
	t.Helper()
	srv, got := stripeFake(t, status, body)
	e := external(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL})
	mg := &product{}
	if externalName != "" {
		mg = named(externalName)
	}
	o, err := e.Observe(t.Context(), mg)
	return o, mg, *got, err
}

func TestObserveWithoutExternalNameReportsAbsentWithoutCallingStripe(t *testing.T) {
	o, _, got, err := observe(t, http.StatusOK, `{}`, "")
	if err != nil {
		t.Fatal(err)
	}
	if o.ResourceExists {
		t.Error("ResourceExists = true, want false")
	}
	if len(got) != 0 {
		t.Errorf("requests = %v, want none", got)
	}
}

func TestObserveGetsItemAndRecordsObservation(t *testing.T) {
	o, mg, got, err := observe(t, http.StatusOK, `{"id":"prod_1","name":"Gold","active":true,"unit_amount":9007199254740993}`, "prod_1")
	if err != nil {
		t.Fatal(err)
	}
	if !o.ResourceExists {
		t.Error("ResourceExists = false, want true")
	}
	r := requestAt(t, got, 0)
	if want := []string{http.MethodGet, "/v1/products/prod_1"}; !reflect.DeepEqual([]string{r.Method, r.Path}, want) {
		t.Errorf("request = %v %v, want %v", r.Method, r.Path, want)
	}
	if want := map[string]any{"id": "prod_1", "name": "Gold", "active": true, "unit_amount": json.Number("9007199254740993")}; !reflect.DeepEqual(mg.observation, want) {
		t.Errorf("observation = %#v, want %#v", mg.observation, want)
	}
}

func TestObserveNotFoundReportsAbsent(t *testing.T) {
	o, _, _, err := observe(t, http.StatusNotFound, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such product: 'prod_1'"}}`, "prod_1")
	if err != nil {
		t.Fatal(err)
	}
	if o.ResourceExists {
		t.Error("ResourceExists = true, want false")
	}
}

func TestObserveOtherFailureIsError(t *testing.T) {
	_, _, _, err := observe(t, http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"bad"}}`, "prod_1")
	if err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestObserveExistingObjectIsAvailable(t *testing.T) {
	_, w := observeWidget(t, widgets, inSyncWidget(), remoteWidget)
	if got := w.GetCondition(xpv2.TypeReady); !got.Equal(xpv2.Available()) {
		t.Errorf("Ready condition = %v, want Available", got)
	}
}

func TestObserveUpToDate(t *testing.T) {
	cases := map[string]struct {
		descriptor Descriptor
		change     func(*widgetParams)
		want       bool
	}{
		"matching spec ignores unset fields set remotely": {widgets, func(*widgetParams) {}, true},
		"amount differs beyond float precision":           {widgets, func(p *widgetParams) { p.Amount = ptr(int64(9007199254740992)) }, false},
		"nested field differs":                            {widgets, func(p *widgetParams) { p.Shipping.Days = ptr(int64(5)) }, false},
		"array element differs":                           {widgets, func(p *widgetParams) { p.Tags = []string{"a", "c"} }, false},
		"array shorter in spec":                           {widgets, func(p *widgetParams) { p.Tags = []string{"a"} }, false},
		"array longer in spec":                            {widgets, func(p *widgetParams) { p.Tags = []string{"a", "b", "c"} }, false},
		"metadata value differs":                          {widgets, func(p *widgetParams) { p.Metadata["team"] = "edge" }, false},
		"metadata key missing remotely":                   {widgets, func(p *widgetParams) { p.Metadata["tier"] = "gold" }, false},
		"remote metadata key absent from spec":            {widgets, func(p *widgetParams) { p.Metadata = nil }, false},
		"update-only field differs":                       {widgets, func(p *widgetParams) { p.Archived = ptr(true) }, false},
		"create-only field differs":                       {widgets, func(p *widgetParams) { p.Currency = "eur" }, true},
		"kind without update endpoint":                    {Descriptor{ItemPath: "/v1/widgets/%s"}, func(p *widgetParams) { p.Amount = ptr(int64(1)) }, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := inSyncWidget()
			tc.change(&p)
			o, _ := observeWidget(t, tc.descriptor, p, remoteWidget)
			if o.ResourceUpToDate != tc.want {
				t.Errorf("ResourceUpToDate = %v, want %v", o.ResourceUpToDate, tc.want)
			}
		})
	}
}

func TestObserveLateInitializesUnsetListedFields(t *testing.T) {
	p := inSyncWidget()
	p.Active = nil
	o, w := observeWidget(t, widgets, p, remoteWidget)
	want := inSyncWidget()
	if diff := cmp.Diff(want, w.params); diff != "" {
		t.Errorf("params (-want +got):\n%s", diff)
	}
	if !o.ResourceLateInitialized {
		t.Error("ResourceLateInitialized = false, want true")
	}
}

func TestObserveKeepsSetFieldsWhenLateInitializing(t *testing.T) {
	p := inSyncWidget()
	p.Active = ptr(false)
	o, w := observeWidget(t, widgets, p, remoteWidget)
	if diff := cmp.Diff(ptr(false), w.params.Active); diff != "" {
		t.Errorf("active (-want +got):\n%s", diff)
	}
	if o.ResourceLateInitialized {
		t.Error("ResourceLateInitialized = true, want false")
	}
}

func TestObserveSkipsLateInitFieldsAbsentRemotely(t *testing.T) {
	p := inSyncWidget()
	p.Active = nil
	o, w := observeWidget(t, widgets, p, `{"id":"wid_1"}`)
	if w.params.Active != nil {
		t.Errorf("active = %v, want nil", *w.params.Active)
	}
	if o.ResourceLateInitialized {
		t.Error("ResourceLateInitialized = true, want false")
	}
}

func TestObserveLateInitTypeMismatchIsError(t *testing.T) {
	e, _ := widgetExternal(t, widgets, http.StatusOK, `{"id":"wid_1","active":"yes"}`)
	_, err := e.Observe(t.Context(), namedWidget(widgetParams{}))
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("err = %v, want an error naming field active", err)
	}
}

func TestObserveSkipsLateInitFieldsNullRemotely(t *testing.T) {
	o, w := observeWidget(t, widgets, widgetParams{}, `{"id":"wid_1","active":null}`)
	if w.params.Active != nil || o.ResourceLateInitialized {
		t.Errorf("active = %v, ResourceLateInitialized = %v, want nil and false", w.params.Active, o.ResourceLateInitialized)
	}
}

const remoteItems = `{"id":"wid_1","tags":[],"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item","price":{"id":"price_1","object":"price","unit_amount":500},"quantity":2}],"has_more":false,"url":"/v1/subscription_items"}}`

func TestObserveUpToDateWithListsAndEmptyArrays(t *testing.T) {
	cases := map[string]struct {
		remote string
		change func(*widgetParams)
		want   bool
	}{
		"list object matches spec by expanded id":           {remoteItems, func(*widgetParams) {}, true},
		"empty array in spec matches absent remote":         {`{"id":"wid_1","items":{"object":"list","data":[{"price":"price_1","quantity":2}]}}`, func(*widgetParams) {}, true},
		"empty array in spec differs from non-empty remote": {remoteItems, func(p *widgetParams) { p.Items = []widgetItem{} }, false},
		"list item expanded id differs":                     {remoteItems, func(p *widgetParams) { p.Items[0].Price = ptr("price_2") }, false},
		"list item field differs":                           {remoteItems, func(p *widgetParams) { p.Items[0].Quantity = ptr(int64(3)) }, false},
		"spec has more items than remote list": {remoteItems, func(p *widgetParams) {
			p.Items = append(p.Items, widgetItem{Price: ptr("price_9")})
		}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := widgetParams{Tags: []string{}, Items: []widgetItem{{Price: ptr("price_1"), Quantity: ptr(int64(2))}}}
			tc.change(&p)
			o, _ := observeWidget(t, widgets, p, tc.remote)
			if o.ResourceUpToDate != tc.want {
				t.Errorf("ResourceUpToDate = %v, want %v", o.ResourceUpToDate, tc.want)
			}
		})
	}
}

func TestObserveUpToDateEquivalences(t *testing.T) {
	cases := map[string]struct {
		remote string
		spec   widgetParams
		want   bool
	}{
		"empty string in spec matches remote null": {`{"id":"wid_1","description":null}`, widgetParams{Description: ptr("")}, true},
		"empty array in spec matches remote null":  {`{"id":"wid_1","tags":null}`, widgetParams{Tags: []string{}}, true},
		"value in spec differs from remote null":   {`{"id":"wid_1","description":null}`, widgetParams{Description: ptr("x")}, false},
		"numeric field compares by value":          {`{"id":"wid_1","rate":"12.50"}`, widgetParams{Rate: ptr(12.5)}, true},
		"string field compares exactly":            {`{"id":"wid_1","description":"007"}`, widgetParams{Description: ptr("7")}, false},
		"numeric field in nested list item":        {`{"id":"wid_1","items":[{"price":"p","quantity":"2.0"}]}`, widgetParams{Items: []widgetItem{{Price: ptr("p"), Quantity: ptr(int64(2))}}}, true},
		"string field in nested list item":         {`{"id":"wid_1","items":[{"price":"02","quantity":2}]}`, widgetParams{Items: []widgetItem{{Price: ptr("2"), Quantity: ptr(int64(2))}}}, false},
		"exponent is equal":                        {`{"id":"wid_1","amount":1e2}`, widgetParams{Amount: ptr(int64(100))}, true},
		"different decimal differs":                {`{"id":"wid_1","rate":"12.51"}`, widgetParams{Rate: ptr(12.5)}, false},
		"integers beyond 2^53 stay exact":          {`{"id":"wid_1","amount":9007199254740993}`, widgetParams{Amount: ptr(int64(9007199254740992))}, false},
		"nested metadata compares only spec keys":  {`{"id":"wid_1","shipping":{"carrier":"ups","metadata":{"x":"y"}}}`, widgetParams{Shipping: &widgetShipping{Carrier: ptr("ups")}}, true},
		"json object number compares by value":     {`{"id":"wid_1","config":{"limit":"10.0","name":"x"}}`, widgetParams{Config: rawJSON(`{"limit":10,"name":"x"}`)}, true},
		"json object string compares exactly":      {`{"id":"wid_1","config":{"limit":10,"name":"x"}}`, widgetParams{Config: rawJSON(`{"limit":10,"name":"y"}`)}, false},
		"json object number differs":               {`{"id":"wid_1","config":{"limit":11}}`, widgetParams{Config: rawJSON(`{"limit":10}`)}, false},
		"json scalar compares by value":            {`{"id":"wid_1","config":100}`, widgetParams{Config: rawJSON(`1e2`)}, true},
		"metadata key with brackets":               {`{"id":"wid_1","metadata":{"a[b]":"1"}}`, widgetParams{Metadata: map[string]string{"a[b]": "1"}}, true},
		"numeric map value compares by value":      {`{"id":"wid_1","currency_options":{"usd":{"unit_amount":1e2}}}`, widgetParams{CurrencyOptions: map[string]widgetCurrencyOption{"usd": {UnitAmount: ptr(int64(100))}}}, true},
		"typed hex is not a number":                {`{"id":"wid_1","amount":"0x64"}`, widgetParams{Amount: ptr(int64(100))}, false},
		"typed fraction is not a number":           {`{"id":"wid_1","rate":"1/2"}`, widgetParams{Rate: ptr(0.5)}, false},
		"typed plus sign is not a number":          {`{"id":"wid_1","amount":"+100"}`, widgetParams{Amount: ptr(int64(100))}, false},
		"typed leading zero is not a number":       {`{"id":"wid_1","amount":"0100"}`, widgetParams{Amount: ptr(int64(100))}, false},
		"json hex is not a number":                 {`{"id":"wid_1","config":{"n":16}}`, widgetParams{Config: rawJSON(`{"n":"0x10"}`)}, false},
		"json fraction is not a number":            {`{"id":"wid_1","config":{"n":0.5}}`, widgetParams{Config: rawJSON(`{"n":"1/2"}`)}, false},
		"json plus sign is not a number":           {`{"id":"wid_1","config":{"n":5}}`, widgetParams{Config: rawJSON(`{"n":"+5"}`)}, false},
		"json leading zero is not a number":        {`{"id":"wid_1","config":{"n":"7"}}`, widgetParams{Config: rawJSON(`{"n":"007"}`)}, false},
		"json negative decimal is a number":        {`{"id":"wid_1","config":{"n":-1.50}}`, widgetParams{Config: rawJSON(`{"n":"-1.5"}`)}, true},
		"unparseable exponent compares exactly":    {`{"id":"wid_1","amount":"1e1000001"}`, widgetParams{Amount: ptr(int64(100))}, false},
		"unparseable exponent equal as strings":    {`{"id":"wid_1","config":{"n":"1e1000001"}}`, widgetParams{Config: rawJSON(`{"n":"1e1000001"}`)}, true},
		"bracketed key in map of struct":           {`{"id":"wid_1","currency_options":{"us[d]":{"unit_amount":100}}}`, widgetParams{CurrencyOptions: map[string]widgetCurrencyOption{"us[d]": {UnitAmount: ptr(int64(100))}}}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, _ := observeWidget(t, widgets, tc.spec, tc.remote)
			if o.ResourceUpToDate != tc.want {
				t.Errorf("ResourceUpToDate = %v, want %v", o.ResourceUpToDate, tc.want)
			}
		})
	}
}

func pagedWidgetServer(t *testing.T, listStatus int) (*External[*widget], *[]string) {
	t.Helper()
	pages := map[string]string{
		"subscription=sub_1&limit=100&starting_after=si_1": `{"object":"list","data":[{"id":"si_2","price":"price_2","quantity":1}],"has_more":true,"url":"/v1/subscription_items?subscription=sub_1"}`,
		"subscription=sub_1&limit=100&starting_after=si_2": `{"object":"list","data":[{"id":"si_3","price":"price_3","quantity":1}],"has_more":false,"url":"/v1/subscription_items?subscription=sub_1"}`,
	}
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == widgetItemPath {
			_, _ = io.WriteString(w, `{"id":"wid_1","items":{"object":"list","data":[{"id":"si_1","price":"price_1","quantity":1}],"has_more":true,"url":"/v1/subscription_items?subscription=sub_1"}}`)
			return
		}
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(listStatus)
		_, _ = io.WriteString(w, pages[r.URL.RawQuery])
	}))
	t.Cleanup(srv.Close)
	return &External[*widget]{Client: NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()), Descriptor: widgets}, &queries
}

func TestObservePagesListsUntilExhaustedBeforeComparing(t *testing.T) {
	e, queries := pagedWidgetServer(t, http.StatusOK)
	items := make([]widgetItem, 0, 3)
	for _, p := range []string{"price_1", "price_2", "price_3"} {
		items = append(items, widgetItem{Price: ptr(p), Quantity: ptr(int64(1))})
	}

	o, err := e.Observe(t.Context(), namedWidget(widgetParams{Items: items}))
	if err != nil {
		t.Fatal(err)
	}

	if !o.ResourceUpToDate {
		t.Error("ResourceUpToDate = false, want true")
	}
	want := []string{
		"/v1/subscription_items?subscription=sub_1&limit=100&starting_after=si_1",
		"/v1/subscription_items?subscription=sub_1&limit=100&starting_after=si_2",
	}
	if diff := cmp.Diff(want, *queries); diff != "" {
		t.Errorf("list requests (-want +got):\n%s", diff)
	}
}

func TestObserveListPageFailureIsError(t *testing.T) {
	e, _ := pagedWidgetServer(t, http.StatusBadRequest)
	if _, err := e.Observe(t.Context(), namedWidget(widgetParams{})); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func deleting(w *widget) *widget {
	w.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	return w
}

func observeDeletedWidget(t *testing.T, d Deletion, status int, body string) (managed.ExternalObservation, []request) {
	t.Helper()
	e, got := widgetExternal(t, withDeletion(d), status, body)
	o, err := e.Observe(t.Context(), deleting(namedWidget(widgetParams{})))
	if err != nil {
		t.Fatal(err)
	}
	return o, *got
}

func TestObserveDeletedNoOpReportsAbsentWithoutCallingStripe(t *testing.T) {
	o, got := observeDeletedWidget(t, Deletion{Mode: NoOp}, http.StatusOK, remoteWidget)

	if o.ResourceExists {
		t.Error("ResourceExists = true, want false")
	}
	if len(got) != 0 {
		t.Errorf("requests = %d, want 0", len(got))
	}
}

const gone = `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"gone"}}`

func unixFromNow(d time.Duration) string {
	return strconv.FormatInt(time.Now().Add(d).Unix(), 10)
}

var creditGrant = Deletion{Mode: Action, Action: voidAction, EndTimestamps: []string{"voided_at", "expires_at"}}

func TestObserveDeletedReportsAbsentOnlyInEndState(t *testing.T) {
	cases := map[string]struct {
		deletion Deletion
		status   int
		body     string
		exists   bool
	}{
		"DeactivateActive":           {deletion: everyDeletion["Deactivate"], body: `{"id":"wid_1","active":true}`, exists: true},
		"DeactivateInactive":         {deletion: everyDeletion["Deactivate"], body: `{"id":"wid_1","active":false}`},
		"SetFieldOther":              {deletion: everyDeletion["SetField"], body: `{"id":"wid_1","status":"active"}`, exists: true},
		"SetFieldValue":              {deletion: everyDeletion["SetField"], body: `{"id":"wid_1","status":"canceled"}`},
		"ActionOpen":                 {deletion: everyDeletion["Action"], body: `{"id":"wid_1","status":"open"}`, exists: true},
		"ActionEndStatus":            {deletion: everyDeletion["Action"], body: `{"id":"wid_1","status":"canceled"}`},
		"ActionUnlistedStatus":       {deletion: everyDeletion["Action"], body: `{"id":"wid_1","status":"succeeded"}`, exists: true},
		"EndTimestampsNullAndFuture": {deletion: creditGrant, body: `{"id":"wid_1","voided_at":null,"expires_at":` + unixFromNow(time.Hour) + `}`, exists: true},
		"EndTimestampsAbsent":        {deletion: creditGrant, body: `{"id":"wid_1"}`, exists: true},
		"EndTimestampsVoidedPast":    {deletion: creditGrant, body: `{"id":"wid_1","voided_at":` + unixFromNow(-time.Minute) + `,"expires_at":null}`},
		"EndTimestampsExpiredPast":   {deletion: creditGrant, body: `{"id":"wid_1","voided_at":null,"expires_at":` + unixFromNow(-time.Minute) + `}`},
		"DeleteDraftElseActionDraft": {deletion: everyDeletion["DeleteDraftElseAction"], body: `{"id":"wid_1","status":"draft"}`, exists: true},
		"DeleteDraftElseActionVoid":  {deletion: everyDeletion["DeleteDraftElseAction"], body: `{"id":"wid_1","status":"void"}`},
		"DeleteDraftElseActionGone":  {deletion: everyDeletion["DeleteDraftElseAction"], status: http.StatusNotFound, body: gone},
		"DeleteExisting":             {deletion: everyDeletion["Delete"], body: `{"id":"wid_1","status":"canceled","active":false}`, exists: true},
		"DeleteGone":                 {deletion: everyDeletion["Delete"], status: http.StatusNotFound, body: gone},
		"UnsetModeReachesDelete":     {body: `{"id":"wid_1","status":"canceled","active":false}`, exists: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, got := observeDeletedWidget(t, tc.deletion, max(tc.status, http.StatusOK), tc.body)

			if o.ResourceExists != tc.exists {
				t.Errorf("ResourceExists = %v, want %v", o.ResourceExists, tc.exists)
			}
			if r := requestAt(t, got, 0); r.Method != http.MethodGet || r.Path != widgetItemPath {
				t.Errorf("request = %v %v, want GET /v1/widgets/wid_1", r.Method, r.Path)
			}
		})
	}
}

func TestObserveDeletingBeforeEndStateOnlyReportsExistence(t *testing.T) {
	e, queries := pagedWidgetServer(t, http.StatusOK)
	e.Descriptor.Deletion = everyDeletion["Action"]
	w := deleting(namedWidget(widgetParams{}))

	o, err := e.Observe(t.Context(), w)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(managed.ExternalObservation{ResourceExists: true}, o); diff != "" {
		t.Errorf("observation (-want +got):\n%s", diff)
	}
	if len(*queries) != 0 {
		t.Errorf("list requests = %v, want none", *queries)
	}
	if w.observation != nil {
		t.Errorf("observation recorded = %v, want none", w.observation)
	}
	if got := w.GetCondition(xpv2.TypeReady); got.Equal(xpv2.Available()) {
		t.Error("Ready condition = Available, want unset")
	}
}

func TestObserveDeletedObjectIsAbsent(t *testing.T) {
	for name, w := range map[string]*widget{"live": namedWidget(widgetParams{}), "deleting": deleting(namedWidget(widgetParams{}))} {
		t.Run(name, func(t *testing.T) {
			e, _ := widgetExternal(t, withDeletion(everyDeletion["Delete"]), http.StatusOK, `{"id":"wid_1","deleted":true}`)

			o, err := e.Observe(t.Context(), w)
			if err != nil {
				t.Fatal(err)
			}
			if o.ResourceExists {
				t.Error("ResourceExists = true, want false")
			}
		})
	}
}

func TestDeletionLifecycleObserveDeleteObserve(t *testing.T) {
	cases := map[string]struct {
		deletion      Deletion
		before, after string
		afterStatus   int
	}{
		"Delete":                     {deletion: everyDeletion["Delete"], before: `{"id":"wid_1"}`, after: gone, afterStatus: http.StatusNotFound},
		"Deactivate":                 {deletion: everyDeletion["Deactivate"], before: `{"id":"wid_1","active":true}`, after: `{"id":"wid_1","active":false}`},
		"Action":                     {deletion: everyDeletion["Action"], before: `{"id":"wid_1","status":"open"}`, after: `{"id":"wid_1","status":"canceled"}`},
		"SetField":                   {deletion: everyDeletion["SetField"], before: `{"id":"wid_1","status":"active"}`, after: `{"id":"wid_1","status":"canceled"}`},
		"DeleteDraftElseActionDraft": {deletion: everyDeletion["DeleteDraftElseAction"], before: `{"id":"wid_1","status":"draft"}`, after: gone, afterStatus: http.StatusNotFound},
		"EndTimestamps":              {deletion: creditGrant, before: `{"id":"wid_1","voided_at":null,"expires_at":null}`, after: `{"id":"wid_1","voided_at":` + unixFromNow(-time.Second) + `,"expires_at":null}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body, status := tc.before, http.StatusOK
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					body, status = tc.after, max(tc.afterStatus, http.StatusOK)
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(srv.Close)
			e := External[*widget]{
				Client:     NewClient(clients.Config{APIKey: testAPIKey, BaseURL: srv.URL}, logging.NewNopLogger()),
				Descriptor: withDeletion(tc.deletion),
			}
			w := deleting(namedWidget(widgetParams{}))

			exists := make([]bool, 0, 2)
			o, err := e.Observe(t.Context(), w)
			if err != nil {
				t.Fatal(err)
			}
			exists = append(exists, o.ResourceExists)
			if _, err := e.Delete(t.Context(), w); err != nil {
				t.Fatal(err)
			}
			o, err = e.Observe(t.Context(), w)
			if err != nil {
				t.Fatal(err)
			}
			exists = append(exists, o.ResourceExists)

			if diff := cmp.Diff([]bool{true, false}, exists); diff != "" {
				t.Errorf("ResourceExists across Observe, Delete, Observe (-want +got):\n%s", diff)
			}
		})
	}
}
