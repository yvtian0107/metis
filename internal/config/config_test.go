package config

import "testing"

func TestDefaultDevConfigMatchesDevComposePorts(t *testing.T) {
	cfg := DefaultDevConfig()

	if cfg.ClickHouse == nil {
		t.Fatal("expected clickhouse config")
	}
	if got, want := cfg.ClickHouse.DSN, "clickhouse://default:@localhost:19000/otel"; got != want {
		t.Fatalf("clickhouse dsn = %q, want %q", got, want)
	}
}
