package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func TestChannelInfo(t *testing.T) {
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

	err = tx.ChannelInfoSet(sid, 0, map[database.ChannelInfoKey]string{
		database.ChannelPosition: "1",
		database.ChannelMaxUsers: "12",
	})
	if err != nil {
		t.Fatal(err)
	}

	desc, err := tx.ChannelInfoGet(sid, 0, database.ChannelDescription)
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("channel desc is not empty: %s", desc)
	}

	info, err := tx.ChannelInfoGets(sid, 0, []database.ChannelInfoKey{database.ChannelDescription, database.ChannelMaxUsers})
	if err != nil {
		t.Fatal(err)
	}

	if info[database.ChannelDescription] != "" {
		t.Errorf("channel desc is not empty")
	}
	if info[database.ChannelMaxUsers] != "12" {
		t.Errorf("channel max users is not match: %s", info[database.ChannelMaxUsers])
	}

	err = tx.ChannelInfoSet(sid, 0, map[database.ChannelInfoKey]string{
		database.ChannelMaxUsers: "10",
	})
	if err != nil {
		t.Fatal(err)
	}

	maxUsers, err := tx.ChannelInfoGet(sid, 0, database.ChannelMaxUsers)
	if err != nil {
		t.Fatal(err)
	}
	if maxUsers != "10" {
		t.Errorf("channel max user is not match: %s", maxUsers)
	}
}
