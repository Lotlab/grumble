package database_test

import (
	"testing"

	"mumble.info/grumble/pkg/database"
)

func NewTestDB() (*database.GrumbleDb, error) {
	db, err := database.NewDB(&database.DbConfig{
		Type: "sqlite",
		Conn: ":memory:",
	})
	if err != nil {
		return nil, err
	}

	err = db.Init()
	if err != nil {
		return nil, err
	}

	return db, err
}

func TestModelInit(t *testing.T) {
	db, err := database.NewDB(&database.DbConfig{
		Type: "sqlite",
		Conn: ":memory:",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Init()
	if err != nil {
		t.Fatal(err)
	}
}
