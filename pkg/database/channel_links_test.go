package database_test

import "testing"

func TestChannelLink(t *testing.T) {
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

	ch1, err := tx.ChannelAdd(sid, "ch1", 0)
	if err != nil {
		t.Fatal(err)
	}
	ch2, err := tx.ChannelAdd(sid, "ch2", 0)
	if err != nil {
		t.Fatal(err)
	}

	err = tx.ChannelLinkAdd(sid, ch1.ChannelID, ch2.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.ChannelLinkAdd(sid, ch2.ChannelID, ch1.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.ChannelLinkAdd(sid, ch1.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.ChannelLinkAdd(sid, 0, ch1.ChannelID)
	if err != nil {
		t.Fatal(err)
	}

	links, count, err := tx.ChannelLinkList(sid, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("total length %d is not match", count)
	}
	if len(links) != 2 {
		t.Errorf("sub length %d is not match", len(links))
	}

	err = tx.ChannelLinkRemove(sid, 0, ch1.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.ChannelLinkRemove(sid, ch1.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, count, err = tx.ChannelLinkList(sid, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("total length %d is not match", count)
	}
}
