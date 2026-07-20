package mqtttls

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfig_ValidCACert_ReturnsTrustingConfig(t *testing.T) {
	cfg, err := LoadClientConfig(filepath.Join("..", "..", "tests", "mosquitto-certs", "ca.pem"))
	if err != nil {
		t.Fatalf("LoadClientConfig: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be set")
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to stay false (CLAUDE.MD §0)")
	}
}

func TestLoadClientConfig_MissingFile_ReturnsError(t *testing.T) {
	if _, err := LoadClientConfig(filepath.Join(t.TempDir(), "does-not-exist.pem")); err == nil {
		t.Fatal("expected an error for a missing CA file")
	}
}

func TestLoadClientConfig_NotAValidCert_ReturnsError(t *testing.T) {
	garbage := filepath.Join(t.TempDir(), "garbage.pem")
	if err := os.WriteFile(garbage, []byte("not a certificate"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadClientConfig(garbage); err == nil {
		t.Fatal("expected an error for a file that contains no valid PEM certificate")
	}
}
