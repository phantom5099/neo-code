package gateway

import "testing"

func TestStrictACLAllowlist(t *testing.T) {
	acl := NewStrictControlPlaneACL()
	cases := []struct {
		source RequestSource
		method string
		want   bool
	}{
		{source: RequestSourceIPC, method: "gateway.authenticate", want: true},
		{source: RequestSourceIPC, method: "gateway.ping", want: true},
		{source: RequestSourceIPC, method: "wake.openUrl", want: true},
		{source: RequestSourceHTTP, method: "gateway.bindStream", want: true},
		{source: RequestSourceWS, method: "wake.openUrl", want: true},
		{source: RequestSourceSSE, method: "gateway.ping", want: true},
		{source: RequestSourceSSE, method: "wake.openUrl", want: false},
		{source: RequestSourceHTTP, method: "gateway.run", want: false},
		{source: RequestSourceUnknown, method: "gateway.ping", want: false},
	}
	for _, tc := range cases {
		got := acl.IsAllowed(tc.source, tc.method)
		if got != tc.want {
			t.Fatalf("acl allowed(%s,%s) = %v, want %v", tc.source, tc.method, got, tc.want)
		}
	}
}

func TestNormalizeRequestSource(t *testing.T) {
	if got := NormalizeRequestSource(" WS "); got != RequestSourceWS {
		t.Fatalf("normalized source = %q, want %q", got, RequestSourceWS)
	}
	if got := NormalizeRequestSource("custom"); got != RequestSourceUnknown {
		t.Fatalf("normalized source = %q, want %q", got, RequestSourceUnknown)
	}
}
