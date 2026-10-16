package stampmitra

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestUnknownFieldsPreserved(t *testing.T) {
	in := `{"referenceId":"SMV-1","valid":true,"registeredName":"ACME","brandNewField":{"x":[1,2,3]},"another":null,"PanStatus":"E"}`
	var v PANVerification
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatal(err)
	}
	if v.ReferenceID != "SMV-1" || !v.Valid || v.RegisteredName != "ACME" {
		t.Fatalf("known fields: %+v", v)
	}
	if string(v.Extra["brandNewField"]) != `{"x":[1,2,3]}` || string(v.Extra["another"]) != "null" {
		t.Fatalf("extra = %v", v.Extra)
	}
	if _, leaked := v.Extra["referenceId"]; leaked {
		t.Fatal("known field leaked into Extra")
	}
	// encoding/json matches keys case-insensitively: "PanStatus" fills PANStatus and is not duplicated in Extra
	if v.PANStatus != "E" {
		t.Errorf("PANStatus = %q", v.PANStatus)
	}
	if _, dup := v.Extra["PanStatus"]; dup {
		t.Error("case-variant of a known field must not be duplicated in Extra")
	}

	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal(out, &a)
	if a["brandNewField"] == nil || a["referenceId"] != "SMV-1" {
		t.Fatalf("marshal lost data: %s", out)
	}
	// round trip is stable
	var v2 PANVerification
	if err := json.Unmarshal(out, &v2); err != nil {
		t.Fatal(err)
	}
	out2, _ := json.Marshal(v2)
	_ = json.Unmarshal(out2, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("round trip differs:\n%s\n%s", out, out2)
	}
}

func TestExtraNilWhenNoUnknownFields(t *testing.T) {
	var v PANVerification
	if err := json.Unmarshal([]byte(`{"referenceId":"x","valid":false}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.Extra != nil {
		t.Fatalf("Extra = %v", v.Extra)
	}
}

func TestDeclaredFieldWinsOverExtraOnMarshal(t *testing.T) {
	v := PANVerification{ReferenceID: "real", Extra: map[string]json.RawMessage{"referenceId": json.RawMessage(`"fake"`), "x": json.RawMessage(`1`)}}
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["referenceId"] != "real" || m["x"] != float64(1) {
		t.Fatalf("%s", b)
	}
}

func TestNestedAndNullableModels(t *testing.T) {
	in := `[{"serviceCode":"s","name":"n","category":null,"basePrice":"500.00","currency":"INR","isActive":true,"gstRatePercent":"18.00","defaultGovernmentFee":null,"processingTimeHours":24,"guestCheckoutEnabled":false,"requiresDocuments":true,"uiSchema":null,"entitled":true,"entitlementUpdatedAt":"2026-09-01T10:00:00.000Z","future":1}]`
	var list []Service
	if err := json.Unmarshal([]byte(in), &list); err != nil {
		t.Fatal(err)
	}
	s := list[0]
	if s.Category != nil || s.GstRatePercent == nil || *s.GstRatePercent != "18.00" || s.ProcessingTimeHours == nil || *s.ProcessingTimeHours != 24 {
		t.Fatalf("%+v", s)
	}
	if string(s.Extra["future"]) != "1" || string(s.UISchema) != "null" {
		t.Fatalf("extra/uiSchema: %v %s", s.Extra, s.UISchema)
	}

	var u UsageSummary
	body := `{"currentPeriod":{"start":"a","end":"b","z":1},"totalApiRequests":5,"callsByService":{"pan.verify":3},"callsByPeriod":[{"period":"2026-10","count":5}],"callsByDay":[{"day":"2026-10-15","count":2}],"extraTop":true}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatal(err)
	}
	if u.CurrentPeriod.Start != "a" || u.CallsByService["pan.verify"] != 3 || u.CallsByPeriod[0].Count != 5 || u.CallsByDay[0].Day != "2026-10-15" {
		t.Fatalf("%+v", u)
	}
	if string(u.CurrentPeriod.Extra["z"]) != "1" || string(u.Extra["extraTop"]) != "true" {
		t.Fatalf("nested extra lost: %+v", u)
	}
}

func TestGenericResponsePreservesEverything(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"items":[{"id":"e1","status":"SUCCEEDED","attempts":1},{"id":"e2"}],"nextCursor":"abc","weird":{"deep":[1]}}`)
	})
	g, err := env.c.Webhooks.Events.List(context.Background(), &WebhookEventsListParams{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(g.Items()); got != 2 {
		t.Errorf("items = %d", got)
	}
	if g.NextCursor() != "abc" {
		t.Errorf("cursor = %q", g.NextCursor())
	}
	if v, ok := g.Get("weird"); !ok || string(v) != `{"deep":[1]}` {
		t.Errorf("weird = %s", v)
	}
	var shaped struct {
		Items []struct{ ID string }
	}
	if err := g.Decode(&shaped); err != nil || shaped.Items[1].ID != "e2" {
		t.Errorf("decode: %v %+v", err, shaped)
	}
	b, _ := json.Marshal(g)
	if string(b) != string(g.Raw()) {
		t.Errorf("marshal differs from raw")
	}
	var nilG *GenericResponse
	if nilG.Items() != nil || nilG.Raw() != nil || nilG.NextCursor() != "" {
		t.Error("nil GenericResponse must be safe")
	}
}

func TestGenericBareArray(t *testing.T) {
	var g GenericResponse
	_ = json.Unmarshal([]byte(`[1,2,3]`), &g)
	if len(g.Items()) != 3 {
		t.Fatal("bare array items")
	}
	if _, err := g.Map(); err == nil {
		t.Fatal("Map on array must fail")
	}
}

func TestServicesListBareArrayAndEmpty(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, `null`) })
	got, err := env.c.Services.List(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestOrderModelsUseExternalOrderID(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, `{"externalOrderId":"ord_ext_123","createdAt":"2026-10-15T09:41:07.000Z","serviceCode":"estamp.instrument.rent_agreement"}`)
	})
	o, err := env.c.Orders.Create(context.Background(), &CreateOrderParams{ExternalOrderID: "ord_ext_123", ExternalCustomerID: "c", ServiceCode: "s", Details: map[string]any{"state": "MH", "stampValue": 500}})
	if err != nil || o.ExternalOrderID != "ord_ext_123" {
		t.Fatalf("%v %+v", err, o)
	}
	if got := env.rec.all()[0].Body; got != `{"externalOrderId":"ord_ext_123","externalCustomerId":"c","serviceCode":"s","details":{"stampValue":500,"state":"MH"}}` {
		t.Errorf("body = %s", got)
	}
}
