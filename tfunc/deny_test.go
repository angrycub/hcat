package tfunc

import "testing"

func TestDeny(t *testing.T) {
	v, err := DenyFunc()
	if v != "" {
		t.Errorf("bad return string: '%v'", v)
	}
	if err != errDisabled {
		t.Errorf("bad error: %v", err)
	}
}
