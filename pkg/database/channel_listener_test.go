package database_test

import "testing"

func TestChannelListener(t *testing.T) {
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

	chl, err := tx.ChannelListenerAdd(sid, 0, 0)
	if err != nil || chl == nil {
		t.Fatal(err)
	}

	err = tx.ChannelListenerSetEnabled(sid, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.ChannelListenerSetVolume(sid, 0, 0, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	list, err := tx.ChannelListenerLoadByUser(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatal("list length not match")
	}

	if list[0].Enabled != false {
		t.Error("channel listener enabled is not match")
	}
	if list[0].VolumeAdjustment != 0.5 {
		t.Error("channel listener volume adjustment is not match")
	}

	chl, err = tx.ChannelListenerAdd(sid, 0, 0)
	if err != nil || chl == nil {
		t.Fatal(err)
	}

	list, err = tx.ChannelListenerLoadByUser(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatal("list length not match")
	}

	if list[0].Enabled != true {
		t.Error("channel listener enabled is not match")
	}

	err = tx.ChannelListenerDelete(sid, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	list, err = tx.ChannelListenerLoadByUser(sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatal("list length not match")
	}
}
