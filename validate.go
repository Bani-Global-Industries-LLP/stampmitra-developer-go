package stampmitra

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Patterns are copied from the backend DTOs and the OpenAPI spec (see testdata/validators.json).
var (
	rePAN      = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)
	reGSTIN    = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
	reIFSC     = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
	reCIN      = regexp.MustCompile(`^[LU][0-9]{5}[A-Z]{2}[0-9]{4}[A-Z]{3}[0-9]{6}$`)
	reUdyam    = regexp.MustCompile(`^UDYAM-[A-Z]{2}-[0-9]{2}-[0-9]{7}$`)
	reVehicle  = regexp.MustCompile(`^[A-Z]{2}[0-9]{1,2}[A-Z]{0,3}[0-9]{4}$`)
	reAccount  = regexp.MustCompile(`^[0-9]{9,18}$`)
	rePassport = regexp.MustCompile(`^[A-Z]{2}[0-9]{13}$`)
	reDL       = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[0-9]{11}$`)
	reVoter    = regexp.MustCompile(`^[A-Z]{3}[0-9]{7}$`)
	rePhone    = regexp.MustCompile(`^(?:\+?91|0)?[6-9][0-9]{9}$`)
)

// asciiUpper upper-cases ASCII letters only. strings.ToUpper would also fold characters such as
// U+0131 (dotless i) and U+017F (long s) into ASCII letters, letting look-alikes pass validation.
func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

func matchUpper(field string, re *regexp.Regexp, human, v string) (string, error) {
	n := asciiUpper(strings.TrimSpace(v))
	if !re.MatchString(n) {
		return "", &ValidationError{Field: field, Message: "must be " + human}
	}
	return n, nil
}

// ValidatePAN trims and upper-cases s and checks the PAN format (5 letters, 4 digits, 1 letter).
// It returns the normalized value or a *ValidationError. Spaces and hyphens inside the value are rejected, not removed.
func ValidatePAN(s string) (string, error) {
	return matchUpper("pan", rePAN, "5 letters, 4 digits, 1 letter", s)
}

// ValidateGSTIN trims and upper-cases s and checks the 15-character GSTIN format.
func ValidateGSTIN(s string) (string, error) {
	return matchUpper("gstin", reGSTIN, "a valid 15-character GSTIN", s)
}

// ValidateIFSC trims and upper-cases s and checks the IFSC format (4 letters, 0, 6 alphanumerics).
func ValidateIFSC(s string) (string, error) {
	return matchUpper("ifsc", reIFSC, "4 letters, the digit 0, then 6 letters or digits", s)
}

// ValidateCIN trims and upper-cases s and checks the 21-character CIN format.
func ValidateCIN(s string) (string, error) {
	return matchUpper("cin", reCIN, "a valid 21-character CIN", s)
}

// ValidateUdyam trims and upper-cases s and checks the UDYAM-XX-00-0000000 format.
func ValidateUdyam(s string) (string, error) {
	return matchUpper("udyamNumber", reUdyam, "in the form UDYAM-XX-00-0000000", s)
}

// ValidateVehicleNumber trims and upper-cases s and checks the registration number format (for example MH12AB1234). Spaces and hyphens are rejected, not removed.
func ValidateVehicleNumber(s string) (string, error) {
	return matchUpper("vehicleNumber", reVehicle, "a registration number without spaces or hyphens", s)
}

// ValidateBankAccount trims s and checks that it is 9 to 18 digits.
func ValidateBankAccount(s string) (string, error) {
	n := strings.TrimSpace(s)
	if !reAccount.MatchString(n) {
		return "", &ValidationError{Field: "accountNumber", Message: "must be 9 to 18 digits"}
	}
	return n, nil
}

// ValidatePassportFileNumber trims and upper-cases s and checks 2 letters followed by 13 digits.
func ValidatePassportFileNumber(s string) (string, error) {
	return matchUpper("fileNumber", rePassport, "2 letters followed by 13 digits", s)
}

// ValidateDrivingLicense trims and upper-cases s and checks 2 letters followed by 13 digits (no separators).
func ValidateDrivingLicense(s string) (string, error) {
	return matchUpper("dlNumber", reDL, "2 letters followed by 13 digits (no spaces or hyphens)", s)
}

// ValidateVoterID trims and upper-cases s and checks 3 letters followed by 7 digits.
func ValidateVoterID(s string) (string, error) {
	return matchUpper("voterId", reVoter, "3 letters followed by 7 digits", s)
}

func validatePhone(s string) (string, error) {
	n := strings.TrimSpace(s)
	if !rePhone.MatchString(n) {
		return "", &ValidationError{Field: "phone", Message: "must be an Indian mobile number"}
	}
	return n, nil
}

func validateDate(field, s string) (string, error) {
	n := strings.TrimSpace(s)
	if _, err := time.Parse("2006-01-02", n); err != nil {
		return "", &ValidationError{Field: field, Message: "must be an ISO date (YYYY-MM-DD)"}
	}
	return n, nil
}

func validateMaxLen(field, s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return &ValidationError{Field: field, Message: fmt.Sprintf("must be at most %d characters", max)}
	}
	return nil
}

func validateRequired(field, s string, max int) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", &ValidationError{Field: field, Message: "is required"}
	}
	if max > 0 {
		if err := validateMaxLen(field, s, max); err != nil {
			return "", err
		}
	}
	return s, nil
}

// validatePathID rejects values that could change the request path and returns the escaped segment.
func validatePathID(field, v string) (string, error) {
	if v == "" || strings.TrimSpace(v) == "" {
		return "", &ValidationError{Field: field, Message: "is required"}
	}
	if v == "." || v == ".." {
		return "", &ValidationError{Field: field, Message: "must not be \".\" or \"..\""}
	}
	for _, r := range v {
		if r == '/' || r == '\\' || r == '?' || r == '#' || r == '%' || r < 0x20 || r == 0x7f {
			return "", &ValidationError{Field: field, Message: "contains a character that is not allowed in an identifier"}
		}
	}
	return pathEscape(v), nil
}

func finite(field string, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return &ValidationError{Field: field, Message: "must be a non-negative finite number"}
	}
	return nil
}

// normalized returns a validated, normalized copy of the receiver.

func (p *PANVerifyParams) normalized() (*PANVerifyParams, error) {
	c := *p
	var err error
	if c.PAN, err = ValidatePAN(c.PAN); err != nil {
		return nil, err
	}
	if err = validateMaxLen("name", c.Name, 256); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *GSTINVerifyParams) normalized() (*GSTINVerifyParams, error) {
	c := *p
	var err error
	if c.GSTIN, err = ValidateGSTIN(c.GSTIN); err != nil {
		return nil, err
	}
	if err = validateMaxLen("businessName", c.BusinessName, 256); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *RCVerifyParams) normalized() (*RCVerifyParams, error) {
	c := *p
	var err error
	if c.VehicleNumber, err = ValidateVehicleNumber(c.VehicleNumber); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *VoterIDVerifyParams) normalized() (*VoterIDVerifyParams, error) {
	c := *p
	var err error
	if c.VoterID, err = ValidateVoterID(c.VoterID); err != nil {
		return nil, err
	}
	if err = validateMaxLen("name", c.Name, 256); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *DrivingLicenseVerifyParams) normalized() (*DrivingLicenseVerifyParams, error) {
	c := *p
	var err error
	if c.DLNumber, err = ValidateDrivingLicense(c.DLNumber); err != nil {
		return nil, err
	}
	if c.DOB, err = validateDate("dob", c.DOB); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *PassportVerifyParams) normalized() (*PassportVerifyParams, error) {
	c := *p
	var err error
	if c.FileNumber, err = ValidatePassportFileNumber(c.FileNumber); err != nil {
		return nil, err
	}
	if c.DOB, err = validateDate("dob", c.DOB); err != nil {
		return nil, err
	}
	if err = validateMaxLen("name", c.Name, 256); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *CINVerifyParams) normalized() (*CINVerifyParams, error) {
	c := *p
	var err error
	if c.CIN, err = ValidateCIN(c.CIN); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *UdyamVerifyParams) normalized() (*UdyamVerifyParams, error) {
	c := *p
	var err error
	if c.UdyamNumber, err = ValidateUdyam(c.UdyamNumber); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *PANToGSTINParams) normalized() (*PANToGSTINParams, error) {
	c := *p
	var err error
	if c.PAN, err = ValidatePAN(c.PAN); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *PANToUdyamParams) normalized() (*PANToUdyamParams, error) {
	c := *p
	var err error
	if c.PAN, err = ValidatePAN(c.PAN); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *BankAccountVerifyParams) normalized() (*BankAccountVerifyParams, error) {
	c := *p
	var err error
	if c.AccountNumber, err = ValidateBankAccount(c.AccountNumber); err != nil {
		return nil, err
	}
	if c.IFSC, err = ValidateIFSC(c.IFSC); err != nil {
		return nil, err
	}
	if c.Phone != "" {
		if c.Phone, err = validatePhone(c.Phone); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

func (p *EstampDutyParams) normalized() (*EstampDutyParams, error) {
	c := *p
	var err error
	if c.StateCode, err = validateRequired("stateCode", c.StateCode, 0); err != nil {
		return nil, err
	}
	if c.ArticleCode, err = validateRequired("articleCode", c.ArticleCode, 0); err != nil {
		return nil, err
	}
	if err = finite("stampDutyAmount", c.StampDutyAmount); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *EstampPartiesParams) normalized() (*EstampPartiesParams, error) {
	c := *p
	if c.PartyPAN != "" {
		var err error
		if c.PartyPAN, err = ValidatePAN(c.PartyPAN); err != nil {
			return nil, &ValidationError{Field: "partyPan", Message: "must be a valid PAN"}
		}
	}
	return &c, nil
}

func (p *CreateOrderParams) normalized() (*CreateOrderParams, error) {
	c := *p
	var err error
	if c.ExternalOrderID, err = validateRequired("externalOrderId", c.ExternalOrderID, 128); err != nil {
		return nil, err
	}
	if c.ExternalCustomerID, err = validateRequired("externalCustomerId", c.ExternalCustomerID, 128); err != nil {
		return nil, err
	}
	if c.ServiceCode, err = validateRequired("serviceCode", c.ServiceCode, 64); err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *CreatePaymentIntentParams) normalized() (*CreatePaymentIntentParams, error) {
	c := *p
	var err error
	if c.ExternalOrderID, err = validateRequired("externalOrderId", c.ExternalOrderID, 0); err != nil {
		return nil, err
	}
	if c.Currency, err = validateRequired("currency", c.Currency, 0); err != nil {
		return nil, err
	}
	if len(c.Currency) < 3 {
		return nil, &ValidationError{Field: "currency", Message: "must be at least 3 characters"}
	}
	if c.PaymentConnectionID, err = validateRequired("paymentConnectionId", c.PaymentConnectionID, 0); err != nil {
		return nil, err
	}
	if c.AmountMinor <= 0 {
		return nil, &ValidationError{Field: "amountMinor", Message: "must be a positive integer (minor currency units)"}
	}
	return &c, nil
}
