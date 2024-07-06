package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func TestUserInfo(t *testing.T) {
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

	err = tx.UserInfoSet(sid, 0, map[database.UserInfoKey]string{
		database.UserComment: "SuperUser",
		database.UserEmail:   "test@example.org",
	})
	if err != nil {
		t.Fatal(err)
	}

	uid, err := tx.UserInfoGetUID(sid, database.UserEmail, "test@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if !uid.Valid || uid.Int64 != 0 {
		t.Fail()
	}

	hash, err := tx.UserInfoGet(sid, 0, database.UserHash)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "" {
		t.Errorf("user hash is not empty: %s", hash)
	}

	info, err := tx.UserInfoGets(sid, 0, []database.UserInfoKey{database.UserComment, database.UserHash})
	if err != nil {
		t.Fatal(err)
	}

	if info[database.UserHash] != "" {
		t.Errorf("user hash is not empty")
	}
	if info[database.UserComment] != "SuperUser" {
		t.Errorf("user comment is not match: %s", info[database.UserComment])
	}

	err = tx.UserInfoSet(sid, 0, map[database.UserInfoKey]string{
		database.UserComment: "RootUser",
	})
	if err != nil {
		t.Fatal(err)
	}

	comment, err := tx.UserInfoGet(sid, 0, database.UserComment)
	if err != nil {
		t.Fatal(err)
	}
	if comment != "RootUser" {
		t.Errorf("user comment is not match: %s", comment)
	}
}
