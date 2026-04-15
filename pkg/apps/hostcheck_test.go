package apps

import "testing"

func TestCheckAllowedHost(t *testing.T) {
	hosts := []string{"*.nubedge.com", "localhost:*"}

	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://foo.nubedge.com/api", false},
		{"http://bar.nubedge.com:8080/api", false},
		{"http://nubedge.com/api", true},            // wildcard doesn't match bare domain
		{"http://localhost:9000/api", false},          // port wildcard
		{"http://localhost:1234/api", false},
		{"http://evil.com/api", true},                 // not in list
		{"http://192.168.1.1/api", true},              // IP blocked by default
		{"http://127.0.0.1:9000/api", true},           // IP blocked by default
	}

	for _, tt := range tests {
		err := CheckAllowedHost(tt.url, hosts)
		if (err != nil) != tt.wantErr {
			t.Errorf("CheckAllowedHost(%q): got err=%v, wantErr=%v", tt.url, err, tt.wantErr)
		}
	}

	// Empty allowedHosts blocks everything.
	if err := CheckAllowedHost("http://localhost:9000", nil); err == nil {
		t.Error("expected error for empty allowedHosts")
	}
}
