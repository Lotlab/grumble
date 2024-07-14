package database

import (
	"database/sql"

	"gorm.io/gorm/clause"
)

type ACL struct {
	ServerID uint64  `gorm:"not null;uniqueIndex:acl_channel_pri"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	ChannelID uint64   `gorm:"not null;uniqueIndex:acl_channel_pri"`
	Channel   *Channel `gorm:"belongsTo;foreignKey:ServerID,ChannelID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE;"`

	Priority int `gorm:"uniqueIndex:acl_channel_pri"`

	UserID sql.NullInt64
	User   *User `gorm:"belongsTo;foreignKey:ServerID,UserID;references:ServerID,UserID;constraint:OnDelete:CASCADE;"`

	GroupName  string
	ApplyHere  bool
	ApplySub   bool
	GrantPriv  int
	RevokePriv int
}

func (s ACL) TableName() string {
	return "acl"
}

func NewACL(sid, cid uint64, priority int) ACL {
	return ACL{
		ServerID:  sid,
		ChannelID: cid,
		Priority:  priority,
	}
}

func (s ACL) WithGroup(name string) ACL {
	acl := s
	acl.GroupName = name
	return acl
}

func (s ACL) WithUser(user uint64) ACL {
	acl := s
	acl.UserID.Int64 = int64(user)
	acl.UserID.Valid = true
	return acl
}

func (s ACL) Apply(here, sub bool) ACL {
	acl := s
	acl.ApplyHere = here
	acl.ApplySub = sub
	return acl
}

func (s ACL) Grant(priv int) ACL {
	acl := s
	acl.GrantPriv = priv
	return acl
}

func (s ACL) Revoke(priv int) ACL {
	acl := s
	acl.RevokePriv = priv
	return acl
}

func (d *DbTx) ACLGet(sid, cid uint64) ([]ACL, error) {
	var acls []ACL
	err := d.db.Find(&acls, "server_id = ? AND channel_id = ?", sid, cid).Order("priority").Error
	return acls, err
}

func (d *DbTx) ACLAdd(acl ACL) error {
	return d.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&acl).Error
}

func (d *DbTx) ACLRemove(sid, cid uint64, priority int) error {
	return d.db.Delete(&ACL{}, "server_id = ? AND channel_id = ? AND priority = ?", sid, cid, priority).Error
}

func (d *DbTx) ACLRemoveByChannel(sid, cid uint64) error {
	return d.db.Delete(&ACL{}, "server_id = ? AND channel_id = ?", sid, cid).Error
}
