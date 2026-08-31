package config

import (
	"os"
	"testing"
)

func TestMustLoad(t *testing.T) {
	tempFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
env: "test"
ws:
  port: 8080
http:
  api_key: "test-key"
auth:
  address: "localhost:9999"
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	os.Setenv("CONFIG_PATH", tempFile.Name())
	defer os.Unsetenv("CONFIG_PATH")

	// Avoid command-line arguments interfering with the flag parsing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd"}

	cfg := MustLoad()

	if cfg.Env != "test" {
		t.Errorf("expected env test, got %s", cfg.Env)
	}
	if cfg.Ws.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Ws.Port)
	}
	if cfg.Http.ApiKey != "test-key" {
		t.Errorf("expected api_key test-key, got %s", cfg.Http.ApiKey)
	}
	if cfg.Auth.Address != "localhost:9999" {
		t.Errorf("expected auth address localhost:9999, got %s", cfg.Auth.Address)
	}
}
