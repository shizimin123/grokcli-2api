package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSaveRenewFailureThresholdIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	accountID := fmt.Sprintf("test-renew-threshold-%d", time.Now().UnixNano())
	if _, err := store.Pool.Exec(ctx, `
		INSERT INTO accounts (id, payload, expires_at)
		VALUES ($1, '{}'::jsonb, now() + interval '1 hour')
	`, accountID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = store.Pool.Exec(context.Background(), `DELETE FROM account_pool WHERE account_id = $1`, accountID)
		_, _ = store.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, accountID)
	})

	assertState := func(wantStatus string, wantFailures int) {
		t.Helper()
		var status string
		var failures int
		if err := store.Pool.QueryRow(ctx, `
			SELECT pool_status, COALESCE((extra->>'renew_fail_count')::int, 0)
			FROM account_pool WHERE account_id = $1
		`, accountID).Scan(&status, &failures); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || failures != wantFailures {
			t.Fatalf("status/failures = %s/%d, want %s/%d", status, failures, wantStatus, wantFailures)
		}
	}

	if err := store.SaveRenewFailure(ctx, accountID, "fail", "temporary network error", "test", false); err != nil {
		t.Fatal(err)
	}
	assertState("normal", 1)

	if err := store.SaveRenewFailure(ctx, accountID, "fail", "temporary network error", "test", false); err != nil {
		t.Fatal(err)
	}
	assertState("expired", 2)
}
