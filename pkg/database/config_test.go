package database_test

import "testing"

func TestConfig(t *testing.T) {
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

	err = tx.ConfigSet(sid, "password", "123456")
	if err != nil {
		t.Fatal(err)
	}

	val, err := tx.ConfigGet(sid, "password")
	if err != nil {
		t.Fatal(err)
	}
	if val != "123456" {
		t.Errorf("value not match")
	}

	vals, err := tx.ConfigList(sid)
	if err != nil {
		t.Fatal(err)
	}
	if vals["password"] != "123456" {
		t.Errorf("value not match")
	}
}
