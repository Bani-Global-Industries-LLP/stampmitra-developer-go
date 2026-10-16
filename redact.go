package stampmitra

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const redacted = "[REDACTED]"

// secretHeaders are never logged.
var secretHeaders = map[string]struct{}{
	"authorization": {}, "x-api-key": {}, "cookie": {}, "set-cookie": {}, "proxy-authorization": {},
}

// identifierKeys hold identifiers; only the last four characters stay visible.
var identifierKeys = toSet("pan", "gstin", "accountnumber", "ifsc", "cin", "udyamnumber", "vehiclenumber", "regno",
	"dlnumber", "filenumber", "voterid", "phone", "din", "chassis", "engine", "micr", "utr", "email",
	"customermobile", "customeremail", "partypan")

// personalKeys hold personal data and are removed entirely.
var personalKeys = toSet("name", "businessname", "registeredname", "fathername", "ownername", "ownerfathername",
	"nameatbank", "fatherorhusbandname", "companyname", "tradename", "legalname", "dob", "address",
	"principalplaceofbusiness", "enterprisename", "customername", "partyname", "partyaddress", "downloadurl", "certificateurl")

func toSet(keys ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}

// redactHeaders returns a copy of h that is safe to log.
func redactHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if _, secret := secretHeaders[strings.ToLower(k)]; secret {
			out[k] = redacted
			continue
		}
		out[k] = strings.Join(v, ", ")
	}
	return out
}

// RedactBody masks personal data and identifiers in a JSON body so it can be logged. Non-JSON
// bodies are replaced by a length marker.
func RedactBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return "[non-JSON body, " + strconv.Itoa(len(body)) + " bytes]"
	}
	out, err := json.Marshal(maskValue("", v))
	if err != nil {
		return "[unserializable body]"
	}
	return string(out)
}

func maskValue(key string, v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = maskValue(k, val)
		}
		return m
	case []any:
		a := make([]any, len(t))
		for i, val := range t {
			a[i] = maskValue(key, val)
		}
		return a
	}
	lk := strings.ToLower(key)
	if _, ok := personalKeys[lk]; ok {
		return redacted
	}
	if _, ok := identifierKeys[lk]; ok {
		s, isString := v.(string)
		if !isString {
			return redacted
		}
		r := []rune(s)
		if len(r) <= 4 {
			return "****"
		}
		return strings.Repeat("*", len(r)-4) + string(r[len(r)-4:])
	}
	return v
}
