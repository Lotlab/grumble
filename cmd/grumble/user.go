// Copyright (c) 2011 The Grumble Authors
// The use of this source code is goverened by a BSD-style
// license that can be found in the LICENSE-file.

package main

import (
	"encoding/hex"
	"errors"

	"mumble.info/grumble/pkg/database"
)

// This file implements Server's handling of Users.
//
// Users are registered clients on the server.

type User struct {
	Id            uint32
	Name          string
	TextureBlob   string
	CommentBlob   string
	LastChannelId int
}

// Create a new User
func NewUser(id uint32, name string) (user *User, err error) {
	if len(name) == 0 {
		return nil, errors.New("invalid username")
	}

	return &User{
		Id:   id,
		Name: name,
	}, nil
}

func UserFromDatabase(dbu *database.User, tx *database.DbTx) *User {
	user := User{
		Id:            uint32(dbu.UserID),
		Name:          dbu.Name,
		TextureBlob:   hex.EncodeToString(dbu.Texture),
		LastChannelId: int(dbu.LastChannel),
	}

	if tx != nil {
		cmt, _ := tx.UserInfoGet(dbu.ServerID, dbu.UserID, database.UserComment)
		user.CommentBlob = cmt
	}

	return &user
}

// HasComment Does the channel have comment?
func (user *User) HasComment() bool {
	return len(user.CommentBlob) > 0
}

// CommentBlobHashBytes gets the hash of the user's comment blob as a byte slice for transmitting via a protobuf message.
// Returns nil if there is no such blob.
func (user *User) CommentBlobHashBytes() (buf []byte) {
	buf, err := hex.DecodeString(user.CommentBlob)
	if err != nil {
		return nil
	}
	return buf
}

// HasTexture Does the user have a texture?
func (user *User) HasTexture() bool {
	return len(user.TextureBlob) > 0
}

// TextureBlobHashBytes gets the hash of the user's texture blob as a byte slice for transmitting via a protobuf message.
// Returns nil if there is no such blob.
func (user *User) TextureBlobHashBytes() (buf []byte) {
	buf, err := hex.DecodeString(user.TextureBlob)
	if err != nil {
		return nil
	}
	return buf
}

// UserSetTexture updates user texture in database
func (server *Server) UserSetTexture(user *User, key string) error {
	return server.DB.Transaction(func(tx *database.DbTx) error {
		hash, err := hex.DecodeString(key)
		if err != nil {
			return err
		}

		if err := tx.UserSetTexture(uint64(server.Id), uint64(user.Id), hash); err != nil {
			return err
		}

		user.TextureBlob = key
		return nil
	})
}

// UserSetComment updates user texture in database
func (server *Server) UserSetComment(user *User, key string) error {
	return server.DB.Transaction(func(tx *database.DbTx) error {
		if err := tx.UserInfoSet(uint64(server.Id), uint64(user.Id), map[database.UserInfoKey]string{
			database.UserComment: key,
		}); err != nil {
			return err
		}

		user.CommentBlob = key
		return nil
	})
}

// UserSetLastChannel updates user last channel in database
func (server *Server) UserSetLastChannel(user *User, ch *Channel) error {
	return server.DB.Transaction(func(tx *database.DbTx) error {
		if err := tx.UserSetLastChannel(uint64(server.Id), uint64(user.Id), uint64(ch.Id)); err != nil {
			return err
		}

		user.LastChannelId = ch.Id
		return nil
	})
}
