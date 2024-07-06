package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func TestMetaVersionGet(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()

	ver, err := tx.MetaGetVersion()
	if err != nil {
		t.Fatal(err)
	}

	if ver != database.DBStructureVersion {
		t.Errorf("db version is not match! ver: %d", ver)
	}
}

func TestMetaKDFIter(t *testing.T) {
	db, err := NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	tx := db.Tx()
	defer tx.Rollback()

	err = tx.MetaSetKDFIter(12)
	if err != nil {
		t.Fatal(err)
	}

	iter, err := tx.MetaGetKDFIter()
	if err != nil {
		t.Fatal(err)
	}

	if iter != 12 {
		t.Errorf("iter is not match! %d", iter)
	}

	err = tx.MetaSetKDFIter(14)
	if err != nil {
		t.Fatal(err)
	}

	iter, err = tx.MetaGetKDFIter()
	if err != nil {
		t.Fatal(err)
	}

	if iter != 14 {
		t.Errorf("iter is not match! %d", iter)
	}
}
