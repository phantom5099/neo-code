package gateway

import "testing"

func TestGatewayMetricsSnapshot(t *testing.T) {
	metrics := NewGatewayMetrics()
	metrics.IncRequests("ipc", "gateway.ping", "ok")
	metrics.IncAuthFailures("ws", "unauthorized")
	metrics.IncACLDenied("http", "wake.openUrl")
	metrics.SetConnectionsActive("ws", 2)
	metrics.IncStreamDropped("queue_full")

	snapshot := metrics.Snapshot()
	if snapshot["gateway_requests_total"]["ipc|gateway.ping|ok"] != 1 {
		t.Fatalf("requests snapshot mismatch: %#v", snapshot["gateway_requests_total"])
	}
	if snapshot["gateway_auth_failures_total"]["ws|unauthorized"] != 1 {
		t.Fatalf("auth failures snapshot mismatch: %#v", snapshot["gateway_auth_failures_total"])
	}
	if snapshot["gateway_acl_denied_total"]["http|wake.openurl"] != 1 {
		t.Fatalf("acl denied snapshot mismatch: %#v", snapshot["gateway_acl_denied_total"])
	}
	if snapshot["gateway_connections_active"]["ws"] != 2 {
		t.Fatalf("connections gauge snapshot mismatch: %#v", snapshot["gateway_connections_active"])
	}
	if snapshot["gateway_stream_dropped_total"]["queue_full"] != 1 {
		t.Fatalf("stream dropped snapshot mismatch: %#v", snapshot["gateway_stream_dropped_total"])
	}
}
