package maintainer

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewDefaultsToOneHourRefreshWindow(t *testing.T) {
	t.Setenv("GROK2API_TOKEN_REFRESH_SKEW", "")

	service := New(nil, nil, nil)
	if service.Skew != time.Hour {
		t.Fatalf("Skew = %s, want %s", service.Skew, time.Hour)
	}
}

func TestConfigureFromSettings(t *testing.T) {
	service := New(nil, nil, nil)
	service.ConfigureFromSettings(map[string]any{
		"token_maintain_interval_sec": json.Number("90"),
		"token_refresh_skew_sec":      float64(3600),
	})

	interval, _, _, skew, _ := service.configSnapshot()
	if interval != 90*time.Second {
		t.Fatalf("Interval = %s, want %s", interval, 90*time.Second)
	}
	if skew != time.Hour {
		t.Fatalf("Skew = %s, want %s", skew, time.Hour)
	}
}

func TestConfigureClampsRefreshWindow(t *testing.T) {
	service := New(nil, nil, nil)
	service.Configure(1, 10_000)

	interval, _, _, skew, _ := service.configSnapshot()
	if interval != 30*time.Second {
		t.Fatalf("Interval = %s, want %s", interval, 30*time.Second)
	}
	if skew != 2*time.Hour {
		t.Fatalf("Skew = %s, want %s", skew, 2*time.Hour)
	}
}

func TestSSOReauthCooldown(t *testing.T) {
	now := time.Unix(10_000, 0)
	if !ssoReauthCoolingDown(map[string]any{"sso_reauth_failed_at": float64(now.Add(-14 * time.Minute).Unix())}, now) {
		t.Fatal("recent SSO failure should be cooling down")
	}
	if ssoReauthCoolingDown(map[string]any{"sso_reauth_failed_at": float64(now.Add(-16 * time.Minute).Unix())}, now) {
		t.Fatal("old SSO failure should be eligible for retry")
	}
}
