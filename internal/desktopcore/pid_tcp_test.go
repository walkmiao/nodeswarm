package desktopcore

import "testing"

func TestPIDTCPSettingsAcceptButNeverAutoEnable(t *testing.T) {
	def := Defaults()
	if def.Engine != "observe" {
		t.Fatal("experimental engine must not be default")
	}
	cfg := def
	cfg.Controller = "https://example.com"
	cfg.Name = "Windows-PC"
	cfg.Engine = "pidtcp"
	if err := ValidateSettings(cfg); err != nil {
		t.Fatalf("pidtcp settings rejected: %v", err)
	}
	cfg.Engine = "unsupported"
	if err := ValidateSettings(cfg); err == nil {
		t.Fatal("unknown engine accepted")
	}
}
