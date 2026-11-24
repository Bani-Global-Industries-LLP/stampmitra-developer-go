package stampmitra

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Values for EstampParty.Gender and EstampStampLine.PayerGender.
const (
	GenderMale   = "MALE"
	GenderFemale = "FEMALE"
	GenderOther  = "OTHER"
)

// Values for EstampPropertyArea.Unit.
const (
	AreaSquareFeet    = "Square Feet"
	AreaSquareYard    = "Square Yard"
	AreaSquareMeters  = "Square Meters"
	AreaNotApplicable = "Not Applicable"
)

// Values for EstampStampLine.Category.
const (
	CategoryEStamp      = "e_stamp"
	CategoryTraditional = "traditional_stamp"
)

// DocumentTypePANCard is the only value the document upload accepts for its optional type part.
const DocumentTypePANCard = "pan_card"

// Party is a party to an instrument. The API accepts either a bare name string or an object with
// details; Party marshals to whichever form you built it with and decodes both. Use PartyName for
// a bare name and PartyDetails when the state wants an address or a father's or husband's name.
type Party struct {
	name   string
	detail *EstampParty
}

// PartyName returns a Party that is sent as a bare name string (the API treats it as { name }).
func PartyName(name string) Party { return Party{name: name} }

// PartyDetails returns a Party that is sent as an object.
func PartyDetails(p EstampParty) Party { return Party{name: p.Name, detail: &p} }

// Name returns the party's name in either form.
func (p Party) Name() string { return p.name }

// Details returns the object form, and false when the party is a bare name.
func (p Party) Details() (EstampParty, bool) {
	if p.detail == nil {
		return EstampParty{}, false
	}
	return *p.detail, true
}

// IsZero reports a Party with no name and no details.
func (p Party) IsZero() bool { return p.detail == nil && p.name == "" }

// MarshalJSON emits a string for PartyName values, an object for PartyDetails values and null for the zero value.
func (p Party) MarshalJSON() ([]byte, error) {
	switch {
	case p.detail != nil:
		return json.Marshal(*p.detail)
	case p.name != "":
		return json.Marshal(p.name)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts a string or an object.
func (p *Party) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*p = Party{}
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*p = Party{name: s}
	case len(b) > 0 && b[0] == '{':
		var d EstampParty
		if err := json.Unmarshal(b, &d); err != nil {
			return err
		}
		*p = Party{name: d.Name, detail: &d}
	default:
		return errors.New("stampmitra: a party must be a string or an object")
	}
	return nil
}
