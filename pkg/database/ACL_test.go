package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func TestACL(t *testing.T) {
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

	err = tx.ACLAdd(database.NewACL(sid, 0, 1).Apply(true, true).Grant(0x01).WithGroup("admin"))
	if err != nil {
		t.Fatal(err)
	}

	acls, err := tx.ACLGet(sid, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(acls) != 1 {
		t.Error("acl length is not match")
	}

	err = tx.ACLRemove(sid, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	acls, err = tx.ACLGet(sid, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(acls) != 0 {
		t.Error("acl length is not match")
	}
}
