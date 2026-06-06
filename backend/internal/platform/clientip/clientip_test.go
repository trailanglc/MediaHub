package clientip

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"192.168.1.10", "192.168.1.10"},
		{" 203.0.113.5 , 10.0.0.1", "203.0.113.5"},
		{"::ffff:192.168.1.10", "192.168.1.10"},
		{"::1", "::1"},
		{"[2001:db8::1]", "2001:db8::1"},
		{"2001:db8::dead:beef%eth0", "2001:db8::dead:beef"},
		{"not-an-ip", "not-an-ip"},
	}
	for _, tc := range tests {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
