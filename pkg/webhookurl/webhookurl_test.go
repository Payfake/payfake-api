package webhookurl

import "testing"

func TestValidateRejectsPrivateDestinationsInProductionMode(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1:8080/webhook",
		"http://169.254.169.254/latest/meta-data",
		"http://localhost:3000/webhook",
	} {
		if err := Validate(rawURL, false); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}

func TestValidateAllowsLocalDevelopmentWhenExplicitlyEnabled(t *testing.T) {
	if err := Validate("http://localhost:3000/webhook", true); err != nil {
		t.Fatalf("expected local development URL to pass: %v", err)
	}
}

func TestValidateRejectsUnsupportedSchemes(t *testing.T) {
	if err := Validate("file:///etc/passwd", true); err == nil {
		t.Fatal("expected non-HTTP webhook URL to be rejected")
	}
}
