package database_test

import (
	"testing"
	"time"

	"mumble.info/grumble/pkg/database"
)

func TestBanList(t *testing.T) {
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

	err = tx.BanWrite([]database.Ban{
		{
			ServerID: sid,
			Start:    time.Now(),
			Duraion:  120,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := tx.BanRead(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list length %d is not match", len(list))
	}
}
