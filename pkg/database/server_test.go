package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func TestServerAdd(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()
	serv, err := tx.ServerAdd()
	if err != nil {
		t.Fatal(err)
	}

	if serv == nil {
		t.Fatal("server is nil")
	}
	if serv.ServerID != 1 {
		t.Fatalf("first server id %d is not 1", serv.ServerID)
	}

	exists, err := tx.ServerExists(serv.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("server %d is not exists", serv.ServerID)
	}
}

func TestServerList(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()
	serv1, err := tx.ServerAdd()
	if err != nil {
		t.Fatal(err)
	}

	if serv1 == nil {
		t.Fatal("server is nil")
	}

	serv2, err := tx.ServerAdd()
	if err != nil {
		t.Fatal(err)
	}
	if serv2 == nil {
		t.Fatal("server is nil")
	}

	list, err := tx.ServerList(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("server list count %d is not match", len(list))
	}
	if list[0].ServerID != serv1.ServerID {
		t.Fatalf("server 1 id is not match")
	}
	if list[1].ServerID != serv2.ServerID {
		t.Fatalf("server 2 id is not match")
	}
}

func TestServerDelete(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()
	serv, err := tx.ServerAdd()
	if err != nil {
		t.Fatal(err)
	}

	if serv == nil {
		t.Fatal("server is nil")
	}

	err = tx.ServerDelete(serv.ServerID)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := tx.ServerExists(serv.ServerID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("server %d is still exists after delete", serv.ServerID)
	}
}

func NewTestServer(tx database.DbTx) (uint64, error) {
	serv, err := tx.ServerAdd()
	if err != nil {
		return 0, err
	}

	_, err = tx.ServerInit(serv.ServerID, database.UserPasswordHash{
		Hash:          "sha1:1:2",
		KDFIterations: 0,
	})
	if err != nil {
		return 0, err
	}

	return serv.ServerID, nil
}
