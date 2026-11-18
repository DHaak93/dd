package dd

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	in := `{"bsid":"B1","phoneId":"P1","phonePassword":"pp","userPassword":"up","sessionSecret":"ss","PhoneSecret" : "x","messages":"[]"}`
	out := redactSecrets(in)
	for _, secret := range []string{`"pp"`, `"up"`, `"ss"`, `"x"`} {
		if strings.Contains(out, secret) {
			t.Errorf("secret %s not redacted: %s", secret, out)
		}
	}
	for _, keep := range []string{`"bsid":"B1"`, `"phoneId":"P1"`, `"messages":"[]"`} {
		if !strings.Contains(out, keep) {
			t.Errorf("non-secret %s was changed: %s", keep, out)
		}
	}
}
