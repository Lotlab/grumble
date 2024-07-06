package database_test

import "testing"

func TestChannelAdd(t *testing.T) {
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

	ch, err := tx.ChannelAdd(sid, "Test Channel", 0)
	if ch == nil || err != nil {
		t.Fatal(err)
	}

	if ch.ChannelID != 1 {
		t.Errorf("channel id %d is not match", ch.ChannelID)
	}
	if ch.Name != "Test Channel" {
		t.Errorf("channel name %s is not match", ch.Name)
	}
	if ch.ServerID != sid {
		t.Errorf("server id %d is not match", ch.ServerID)
	}
	if !ch.ParentID.Valid || ch.ParentID.Int64 != 0 {
		t.Errorf("parent id %d is not match", ch.ParentID.Int64)
	}

	chs, err := tx.ChannelListByParent(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 1 {
		t.Fatalf("channel is is incorrect")
	}
	if chs[0].ChannelID != ch.ChannelID {
		t.Fatalf("channel id not match")
	}
}

func TestChannelList(t *testing.T) {
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

	chs, err := tx.ChannelListNoParent(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 1 {
		t.Fatalf("channel is is incorrect")
	}
	if chs[0].ChannelID != 0 {
		t.Fatalf("root channel not found")
	}
}

func TestChannelUpdate(t *testing.T) {
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

	err = tx.ChannelUpdate(sid, 0, "RootCh", -1, true)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := tx.ChannelGet(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name != "RootCh" {
		t.Errorf("channel name %s is not match", ch.Name)
	}
	if ch.InheritACL != true {
		t.Error("channel inherit acl is not modified")
	}
}

func TestChannelRemove(t *testing.T) {
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

	ch, err := tx.ChannelAdd(sid, "Test Channel", 0)
	if ch == nil || err != nil {
		t.Fatal(err)
	}

	chMod, err := tx.ChannelGet(sid, ch.ChannelID)
	if chMod == nil || err != nil {
		t.Fatal(err)
	}

	err = tx.ChannelRemove(sid, ch.ChannelID)
	if err != nil {
		t.Fatal(err)
	}

	chMod, err = tx.ChannelGet(sid, ch.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if chMod != nil {
		t.Fatalf("ch is not deleted, %d", chMod.ChannelID)
	}
}
