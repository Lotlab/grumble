package database_test

import "testing"

func TestGroupCreate(t *testing.T) {
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

	group1, err := tx.GroupAdd(sid, 0, "Group1", true, true)
	if err != nil || group1 == nil {
		t.Fatal(err)
	}
	group2, err := tx.GroupAdd(sid, 0, "Group2", true, true)
	if err != nil || group2 == nil {
		t.Fatal(err)
	}
	group3, err := tx.GroupAdd(sid, 0, "Group1", true, true)
	if err == nil || group3 != nil {
		t.Fatal(err)
	}

	group, err := tx.GroupGetByGID(sid, group1.GroupID)
	if err != nil || group == nil {
		t.Fatal(err)
	}
	if group.Name != group1.Name {
		t.Error("group name not match")
	}

	group, err = tx.GroupGetByGID(sid, 14)
	if err != nil || group != nil {
		t.Fatal(err)
	}

	groups, err := tx.GroupGetByChannel(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Errorf("groups count is not match")
	}
}

func TestGroupModify(t *testing.T) {
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

	groups, err := tx.GroupGetByChannel(sid, 0)
	if err != nil || len(groups) == 0 {
		t.Fatal(err)
	}

	groups[0].Inherit = false
	err = tx.GroupModify(&groups[0])
	if err != nil {
		t.Fatal(err)
	}

	group, err := tx.GroupGetByGID(sid, groups[0].GroupID)
	if err != nil || group == nil {
		t.Fatal(err)
	}

	if group.Inherit != groups[0].Inherit {
		t.Fatal("inherit is not update")
	}
}

func TestGroupDelete(t *testing.T) {
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

	group1, err := tx.GroupAdd(sid, 0, "Group1", true, true)
	if err != nil || group1 == nil {
		t.Fatal(err)
	}
	group2, err := tx.GroupAdd(sid, 0, "Group2", true, true)
	if err != nil || group2 == nil {
		t.Fatal(err)
	}
	groups, err := tx.GroupGetByChannel(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Errorf("groups count is not match")
	}

	err = tx.GroupDelete(sid, group1.GroupID)
	if err != nil {
		t.Fatal(err)
	}

	groups, err = tx.GroupGetByChannel(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Errorf("groups count is not match")
	}

	err = tx.GroupDeleteByChannel(sid, 0)
	if err != nil {
		t.Fatal(err)
	}

	groups, err = tx.GroupGetByChannel(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("groups count is not match")
	}
}
