package database_test

import (
	"testing"
	"time"

	"mumble.info/grumble/pkg/database"
)

func TestUserRegister(t *testing.T) {
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

	user, err := tx.UserRegister(sid, "TestUser")
	if err != nil || user == nil {
		t.Fatal(err)
	}

	userDup, err := tx.UserRegister(sid, user.Name)
	if err == nil || userDup != nil {
		t.Fatal("duplicate user is created!")
	}

	user2, err := tx.UserRegister(sid, "DemoUser")
	if err != nil || user2 == nil {
		t.Fatal(err)
	}

	uList, count, err := tx.UserList(sid, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("user list total length %d is not match", count)
	}
	if len(uList) != 2 {
		t.Errorf("user list sub length %d is not match", count)
	}

	uList, err = tx.UserFind(sid, "Test%")
	if err != nil {
		t.Fatal(err)
	}

	if len(uList) != 1 {
		t.Errorf("user find is invalid")
	}

	exists, err := tx.UserExists(sid, user.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("user exists is not match")
	}

	exists, err = tx.UserExists(sid, 999)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("user exists is not match")
	}
}

func TestUserUnregister(t *testing.T) {
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

	user, err := tx.UserRegister(sid, "TestUser")
	if err != nil || user == nil {
		t.Fatal(err)
	}

	err = tx.UserUnregister(sid, user.UserID)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := tx.UserExists(sid, user.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("user unregister is failed")
	}
}

func TestUserGetSet(t *testing.T) {
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

	user, err := tx.UserRegister(sid, "TestUser")
	if err != nil || user == nil {
		t.Fatal(err)
	}

	user2, err := tx.UserGetID(sid, user.Name)
	if err != nil || user2 == nil {
		t.Fatalf("failed to get user id: %v", err)
	}

	user3, err := tx.UserGetID(sid, "NonExists")
	if err != nil || user3 != nil {
		t.Fatalf("get invalid user with non-exists name: %v", err)
	}

	now := time.Now()
	err = tx.UserSetLastChannel(sid, user.UserID, 1)
	if err != nil {
		t.Fatal(err)
	}

	info, err := tx.UserGetInfo(sid, user.UserID)
	if err != nil || info == nil {
		t.Fatal(err)
	}

	if info.LastActive.Sub(now) < 0 {
		t.Fatal("last active is incorrect")
	}

	err = tx.UserSetLastDisconnect(sid, info.UserID)
	if err != nil {
		t.Fatal(err)
	}

	info, err = tx.UserGetLastChannel(sid, user.UserID)
	if err != nil || info == nil {
		t.Fatal(err)
	}

	if info.LastChannel != 1 {
		t.Fatalf("last channel %d is incorrect", info.LastChannel)
	}
	if info.LastDisconnect.Sub(now) < 0 {
		t.Fatal("last disconnect is incorrect")
	}
}

func TestUserAuth(t *testing.T) {
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

	user, err := tx.UserRegister(sid, "TestUser")
	if err != nil || user == nil {
		t.Fatal(err)
	}

	expected := database.UserPasswordHash{
		Hash:          "someHash",
		Salt:          []byte{17, 69, 20},
		KDFIterations: 19,
	}
	err = tx.UserSetAuth(sid, user.UserID, expected)
	if err != nil {
		t.Fatal(err)
	}

	auth, err := tx.UserGetAuth(sid, user.Name)
	if err != nil || auth == nil {
		t.Fatalf("failed to get auth: %v", err)
	}

	if auth.Password.Hash != expected.Hash {
		t.Error("hash is not match")
	}
	if string(auth.Password.Salt) != string(expected.Salt) {
		t.Error("salt is not match")
	}
	if auth.Password.KDFIterations != expected.KDFIterations {
		t.Error("KDFIter is not match")
	}

	auth2, err := tx.UserGetAuth(sid, "NonExist")
	if err != nil || auth2 != nil {
		t.Fatalf("get non-exist user auth: %v", err)
	}
}

func TestUserTexture(t *testing.T) {
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

	err = tx.UserSetTexture(sid, 0, []byte("FakeTexture"))
	if err != nil {
		t.Fatal(err)
	}

	user, err := tx.UserGetTexture(sid, 0)
	if err != nil || user == nil {
		t.Fatalf("failed to get user texture: %v", err)
	}

	if string(user.Texture) != "FakeTexture" {
		t.Error("texture not match")
	}

	user2, err := tx.UserGetTexture(sid, 2)
	if err != nil || user2 != nil {
		t.Errorf("get non-exist user texture: %v", err)
	}
}
