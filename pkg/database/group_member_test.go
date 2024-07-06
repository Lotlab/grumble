package database_test

import "testing"

func TestGroupMember(t *testing.T) {
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

	user1, err := tx.UserRegister(sid, "TestUser")
	if err != nil || user1 == nil {
		t.Fatal(err)
	}

	group1, err := tx.GroupAdd(sid, 0, "Group1", true, true)
	if err != nil || group1 == nil {
		t.Fatal(err)
	}

	err = tx.GroupMemberAdd(sid, group1.GroupID, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.GroupMemberAdd(sid, group1.GroupID, user1.UserID, false)
	if err != nil {
		t.Fatal(err)
	}

	list, err := tx.GroupMemberList(sid, group1.GroupID)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Fatal("list length is not match")
	}

	err = tx.GroupMemberRemove(sid, group1.GroupID, 0)
	if err != nil {
		t.Fatal(err)
	}

	list, err = tx.GroupMemberList(sid, group1.GroupID)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Fatal("list length is not match")
	}

	err = tx.GroupMemberRemoveAll(sid, group1.GroupID)
	if err != nil {
		t.Fatal(err)
	}

	list, err = tx.GroupMemberList(sid, group1.GroupID)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Fatal("list length is not match")
	}
}
