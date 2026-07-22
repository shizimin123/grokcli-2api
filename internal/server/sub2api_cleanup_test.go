package server

import (
	"reflect"
	"testing"
)

func TestSuccessfulSub2APIAccountIDsOnlyReturnsRequestedSuccesses(t *testing.T) {
	result := map[string]any{
		"results": []map[string]any{
			{"account_id": "acc-1", "ok": true},
			{"account_id": "acc-2", "ok": false},
			{"account_id": "outside", "ok": true},
			{"account_id": "acc-1", "ok": true},
		},
	}

	got := successfulSub2APIAccountIDs(result, []string{"acc-1", "acc-2"})
	if want := []string{"acc-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("successful IDs = %#v, want %#v", got, want)
	}
}

func TestMarkSub2APILocalCleanup(t *testing.T) {
	rows := []map[string]any{
		{"account_id": "acc-1", "ok": true},
		{"account_id": "acc-2", "ok": false},
	}
	result := map[string]any{"results": rows}

	markSub2APILocalCleanup(result, []string{"acc-1"})

	if rows[0]["local_deleted"] != true {
		t.Fatalf("successful row not marked: %#v", rows[0])
	}
	if _, exists := rows[1]["local_deleted"]; exists {
		t.Fatalf("failed row was marked deleted: %#v", rows[1])
	}
}
