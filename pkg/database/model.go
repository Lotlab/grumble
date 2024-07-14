package database

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const (
	DBStructureVersion = 9
)

type GrumbleDb struct {
	db *gorm.DB
}

type DbConfig struct {
	Type string
	Conn string
}

func NewDB(cfg *DbConfig) (*GrumbleDb, error) {
	var dialet gorm.Dialector

	switch cfg.Type {
	case "sqlite":
		dialet = sqlite.Open(cfg.Conn)
	default:
		return nil, fmt.Errorf("invalid sql database type %s", cfg.Conn)
	}

	db, err := gorm.Open(dialet, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "",   // table name prefix, table for `User` would be `t_users`
			SingularTable: true, // use singular table name, table for `User` would be `user` with this option enabled
			NameReplacer:  nil,
		},
	})
	if err != nil {
		return nil, err
	}

	return &GrumbleDb{
		db: db,
	}, nil
}

func (d *GrumbleDb) Init() error {
	err := d.db.AutoMigrate(&Meta{}, &Server{}, &ServerLog{}, &Config{}, &Channel{}, &ChannelInfo{}, &User{}, &UserInfo{}, &Group{}, &GroupMember{}, &ACL{}, &ChannelLink{}, &Ban{}, &ChannelListener{})
	if err != nil {
		return err
	}

	tx := d.Tx()
	defer tx.Commit()
	err = tx.MetaSetVersion(DBStructureVersion)
	if err != nil {
		return err
	}
	return nil
}

type DbTx struct {
	db *gorm.DB
}

func (d *GrumbleDb) Tx() DbTx {
	return DbTx{
		db: d.db.Begin(),
	}
}

func (d *DbTx) Commit() error {
	return d.db.Commit().Error
}

func (d *DbTx) Rollback() error {
	return d.db.Rollback().Error
}

func (d *GrumbleDb) Transaction(fc func(tx *DbTx) error) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		return fc(&DbTx{db: tx})
	})
}
