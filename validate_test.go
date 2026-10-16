package stampmitra

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

type validatorFile struct {
	Validators map[string]struct {
		Pattern string   `json:"pattern"`
		Valid   []string `json:"valid"`
		Invalid []string `json:"invalid"`
	} `json:"validators"`
}

func TestValidatorVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/validators.json")
	if err != nil {
		t.Fatal(err)
	}
	var f validatorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	fns := map[string]func(string) (string, error){
		"pan": ValidatePAN, "gstin": ValidateGSTIN, "ifsc": ValidateIFSC, "cin": ValidateCIN, "udyam": ValidateUdyam,
		"vehicleNumber": ValidateVehicleNumber, "bankAccount": ValidateBankAccount, "passport": ValidatePassportFileNumber,
		"drivingLicense": ValidateDrivingLicense, "voterId": ValidateVoterID,
	}
	if len(fns) != len(f.Validators) {
		t.Fatalf("vector file has %d validators, SDK tests %d", len(f.Validators), len(fns))
	}
	n := 0
	for name, v := range f.Validators {
		fn, ok := fns[name]
		if !ok {
			t.Fatalf("no SDK validator for %q", name)
		}
		for _, in := range v.Valid {
			n++
			out, err := fn(in)
			if err != nil {
				t.Errorf("%s: %q should be valid: %v", name, in, err)
				continue
			}
			want := strings.TrimSpace(in)
			if name != "bankAccount" {
				want = strings.ToUpper(want)
			}
			if out != want {
				t.Errorf("%s: %q normalized to %q, want %q", name, in, out, want)
			}
		}
		for _, in := range v.Invalid {
			n++
			out, err := fn(in)
			if err == nil {
				t.Errorf("%s: %q should be invalid (got %q)", name, in, out)
				continue
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || !errors.Is(err, ErrValidation) {
				t.Errorf("%s: want *ValidationError, got %T", name, err)
			}
			if in != "" && strings.Contains(err.Error(), in) {
				t.Errorf("%s: error message echoes the input: %v", name, err)
			}
		}
	}
	if n < 80 {
		t.Fatalf("only %d vectors ran", n)
	}
}

func TestValidatorsDoNotStripSeparators(t *testing.T) {
	for _, c := range []struct {
		fn func(string) (string, error)
		in string
	}{
		{ValidateVehicleNumber, "MH 12 AB 1234"}, {ValidateVehicleNumber, "MH-12-AB-1234"},
		{ValidateDrivingLicense, "MH-14-2011-0012345"}, {ValidatePassportFileNumber, "PA-1079341954215"},
		{ValidatePAN, "ABCDE-1234F"}, {ValidateBankAccount, "1234 5678 9012"},
	} {
		if _, err := c.fn(c.in); err == nil {
			t.Errorf("%q must be rejected, not silently cleaned", c.in)
		}
	}
}

func TestASCIIUppercaseOnly(t *testing.T) {
	// U+0131 (dotless i) and U+017F (long s) upper-case to ASCII 'I' and 'S' with strings.ToUpper.
	for _, in := range []string{"ABCDE1234ı", "ſBCDE1234F"} {
		if _, err := ValidatePAN(in); err == nil {
			t.Errorf("%q must not validate", in)
		}
	}
}

func TestValidationHappensBeforeNetwork(t *testing.T) {
	env := newEnv(t, func(int, http.ResponseWriter, *http.Request) { t.Error("network reached") })
	ctx := context.Background()
	long := strings.Repeat("a", 257)
	checks := map[string]func() error{
		"pan":      func() error { _, e := env.c.Verify.PAN(ctx, &PANVerifyParams{PAN: "bad"}); return e },
		"pan name": func() error { _, e := env.c.Verify.PAN(ctx, &PANVerifyParams{PAN: "ABCDE1234F", Name: long}); return e },
		"gstin":    func() error { _, e := env.c.Verify.GSTIN(ctx, &GSTINVerifyParams{GSTIN: "x"}); return e },
		"rc":       func() error { _, e := env.c.Verify.RC(ctx, &RCVerifyParams{VehicleNumber: "MH 12 AB 1234"}); return e },
		"voter":    func() error { _, e := env.c.Verify.VoterID(ctx, &VoterIDVerifyParams{VoterID: "1"}); return e },
		"dl dob": func() error {
			_, e := env.c.Verify.DrivingLicense(ctx, &DrivingLicenseVerifyParams{DLNumber: "MH1420110012345", DOB: "31-01-1990"})
			return e
		},
		"dl bad date": func() error {
			_, e := env.c.Verify.DrivingLicense(ctx, &DrivingLicenseVerifyParams{DLNumber: "MH1420110012345", DOB: "1990-02-31"})
			return e
		},
		"passport": func() error {
			_, e := env.c.Verify.Passport(ctx, &PassportVerifyParams{FileNumber: "PA1079341954215", DOB: ""})
			return e
		},
		"cin":     func() error { _, e := env.c.Verify.CIN(ctx, &CINVerifyParams{CIN: "x"}); return e },
		"udyam":   func() error { _, e := env.c.Verify.Udyam(ctx, &UdyamVerifyParams{UdyamNumber: "x"}); return e },
		"pan2gst": func() error { _, e := env.c.Verify.PANToGSTIN(ctx, &PANToGSTINParams{PAN: "x"}); return e },
		"pan2udy": func() error { _, e := env.c.Verify.PANToUdyam(ctx, &PANToUdyamParams{PAN: "x"}); return e },
		"bank": func() error {
			_, e := env.c.Verify.BankAccount(ctx, &BankAccountVerifyParams{AccountNumber: "1", IFSC: "ICIC0001234"})
			return e
		},
		"bank ifsc": func() error {
			_, e := env.c.Verify.BankAccount(ctx, &BankAccountVerifyParams{AccountNumber: "123456789", IFSC: "bad"})
			return e
		},
		"bank phone": func() error {
			_, e := env.c.Verify.BankAccount(ctx, &BankAccountVerifyParams{AccountNumber: "123456789", IFSC: "ICIC0001234", Phone: "12345"})
			return e
		},
		"duty": func() error {
			_, e := env.c.Estamp.CalculateDuty(ctx, &EstampDutyParams{StateCode: "", ArticleCode: "5"})
			return e
		},
		"duty nan": func() error {
			_, e := env.c.Estamp.CalculateDuty(ctx, &EstampDutyParams{StateCode: "MH", ArticleCode: "5", StampDutyAmount: -1})
			return e
		},
		"parties": func() error { _, e := env.c.Estamp.AddParties(ctx, "o", &EstampPartiesParams{PartyPAN: "x"}); return e },
		"order":   func() error { _, e := env.c.Orders.Create(ctx, &CreateOrderParams{}); return e },
		"order id": func() error {
			_, e := env.c.Orders.Create(ctx, &CreateOrderParams{ExternalOrderID: strings.Repeat("x", 129), ExternalCustomerID: "c", ServiceCode: "s"})
			return e
		},
		"intent": func() error {
			_, e := env.c.Payments.Intents.Create(ctx, &CreatePaymentIntentParams{ExternalOrderID: "o"})
			return e
		},
		"intent amount": func() error {
			_, e := env.c.Payments.Intents.Create(ctx, &CreatePaymentIntentParams{ExternalOrderID: "o", Currency: "INR", PaymentConnectionID: "p"})
			return e
		},
		"logs status": func() error { _, e := env.c.Logs.List(ctx, &LogsListParams{Status: "weird"}); return e },
		"logs limit":  func() error { _, e := env.c.Logs.List(ctx, &LogsListParams{Limit: 500}); return e },
		"events limit": func() error {
			_, e := env.c.Webhooks.Events.List(ctx, &WebhookEventsListParams{Limit: -1})
			return e
		},
	}
	for name, fn := range checks {
		err := fn()
		var ve *ValidationError
		if !errors.As(err, &ve) || !errors.Is(err, ErrValidation) || !IsValidation(err) {
			t.Errorf("%s: want local *ValidationError, got %v", name, err)
		}
	}
	if n := len(env.rec.all()); n != 0 {
		t.Fatalf("%d requests reached the network", n)
	}
}

func TestInputIsNormalizedBeforeSending(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"referenceId":"r","valid":true}`)
	})
	p := &PANVerifyParams{PAN: " abcde1234f ", Name: "Rahul"}
	if _, err := env.c.Verify.PAN(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if got := env.rec.all()[0].Body; got != `{"pan":"ABCDE1234F","name":"Rahul"}` {
		t.Errorf("body = %s", got)
	}
	if p.PAN != " abcde1234f " {
		t.Error("caller's params must not be mutated")
	}
}

func TestValidateInputOptOut(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"referenceId":"r","valid":false}`)
	}, WithValidateInput(false))
	if _, err := env.c.Verify.PAN(context.Background(), &PANVerifyParams{PAN: "not-a-pan"}); err != nil {
		t.Fatal(err)
	}
	if got := env.rec.all()[0].Body; got != `{"pan":"not-a-pan"}` {
		t.Errorf("body = %s", got)
	}
}

func TestBankAccountPhoneSent(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"referenceId":"r","valid":true}`)
	})
	_, err := env.c.Verify.BankAccount(context.Background(), &BankAccountVerifyParams{AccountNumber: "000123456789", IFSC: "icic0001234", Name: "A", Phone: "9876543210"})
	if err != nil {
		t.Fatal(err)
	}
	if got := env.rec.all()[0].Body; got != `{"accountNumber":"000123456789","ifsc":"ICIC0001234","name":"A","phone":"9876543210"}` {
		t.Errorf("body = %s", got)
	}
}
