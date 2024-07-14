package main

import (
	"strconv"

	"mumble.info/grumble/pkg/database"
	"mumble.info/grumble/pkg/serverconf"
)

type ConfigWrapper struct {
	db       *database.GrumbleDb
	serverID uint64
}

func NewConfigWrapper(db *database.GrumbleDb, serverID uint64) serverconf.ConfigRepo {
	return &ConfigWrapper{
		db:       db,
		serverID: serverID,
	}
}

func (c *ConfigWrapper) Set(key, value string) {
	tx := c.db.Tx()
	defer tx.Commit()

	tx.ConfigSet(c.serverID, key, value)
}

func (c *ConfigWrapper) StringValue(key string) string {
	tx := c.db.Tx()
	defer tx.Rollback()

	val, err := tx.ConfigGet(c.serverID, key)
	if err != nil {
		return ""
	}

	if val != "" {
		return val
	}

	val, ok := serverconf.DefaultCfg[key]
	if ok {
		return val
	}

	return ""
}

// IntValue gets the value of a speific config key as an int
func (cfg *ConfigWrapper) IntValue(key string) (intval int) {
	str := cfg.StringValue(key)
	intval, _ = strconv.Atoi(str)
	return
}

// Uint32Value gets the value of a specific config key as a uint32
func (cfg *ConfigWrapper) Uint32Value(key string) (uint32val uint32) {
	str := cfg.StringValue(key)
	uintval, _ := strconv.ParseUint(str, 10, 0)
	return uint32(uintval)
}

// BoolValue gets the value fo a sepcific config key as a bool
func (cfg *ConfigWrapper) BoolValue(key string) (boolval bool) {
	str := cfg.StringValue(key)
	boolval, _ = strconv.ParseBool(str)
	return
}
