package database_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSlogAdd(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()

	sid, err := NewTestServer(tx)
	if err != nil {
		t.Fatal(err)
	}
	sid2, err := NewTestServer(tx)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	for i := 0; i < 20; i++ {
		err = tx.ServerLogAdd(sid, fmt.Sprintf("test log %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		err = tx.ServerLogAdd(sid2, fmt.Sprintf("test log %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	logs, count, err := tx.ServerLogGet(sid, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 10 {
		t.Errorf("logs length is not match, len %d", len(logs))
	}
	if count != 20 {
		t.Errorf("logs total length is not match, count: %d", count)
	}

	for _, v := range logs {
		if !strings.HasPrefix(v.Msg, "test log") {
			t.Errorf("log text is not match")
		}
		if v.MsgTime.Sub(now) > time.Second {
			t.Errorf("log time is not match")
		}
	}

	err = tx.ServerLogWipe(sid)
	if err != nil {
		t.Fatal(err)
	}

	logs, count, err = tx.ServerLogGet(sid, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 {
		t.Errorf("logs length is not match, len %d", len(logs))
	}
	if count != 0 {
		t.Errorf("logs total length is not match, count: %d", count)
	}
}
