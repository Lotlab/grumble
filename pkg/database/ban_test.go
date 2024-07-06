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

	// todo: ban list should be adjust, e.g. add some primary key to reference
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

	list, count, err := tx.BanRead(sid, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list length %d is not match", len(list))
	}
	if count != 1 {
		t.Errorf("total length %d is not match", count)
	}
}
