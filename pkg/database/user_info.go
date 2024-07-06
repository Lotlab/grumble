package database

import (
	"database/sql"

	"gorm.io/gorm/clause"
)

type UserInfoKey int

const (
	UserName UserInfoKey = iota
	UserEmail
	UserComment
	UserHash
	UserPassword
	UserLastActive
	UserKDFIterations
)

type UserInfo struct {
	ServerID uint64  `gorm:"not null;uniqueIndex:user_info_id"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	UserID uint64      `gorm:"not null;uniqueIndex:user_info_id"`
	User   *User       `gorm:"belongsTo;ForeignKey:ServerID,UserID;references:ServerID,UserID;constraint:OnDelete:CASCADE;"`
	Key    UserInfoKey `gorm:"uniqueIndex:user_info_id"`
	Value  string
}

func (s UserInfo) UserInfo() string {
	return "user_info"
}

func (d *DbTx) UserInfoSet(serverID uint64, uid uint64, info map[UserInfoKey]string) error {
	for k, v := range info {
		err := d.db.Clauses(&clause.OnConflict{
			UpdateAll: true,
		}).Create(&UserInfo{
			ServerID: serverID,
			UserID:   uid,
			Key:      k,
			Value:    v,
		}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DbTx) UserInfoGet(sid, uid uint64, key UserInfoKey) (string, error) {
	info := UserInfo{}
	err := d.db.Find(&info, "server_id = ? AND user_id = ? AND key = ?", sid, uid, key).Error
	return info.Value, err
}

func (d *DbTx) UserInfoGetUID(sid uint64, key UserInfoKey, value string) (sql.NullInt64, error) {
	info := UserInfo{}
	err := d.db.Find(&info, "server_id = ? AND key = ? AND value = ?", sid, key, value).Error
	if info.ServerID != sid {
		return sql.NullInt64{}, err
	}
	return sql.NullInt64{
		Valid: true,
		Int64: int64(info.UserID),
	}, err
}

func (d *DbTx) UserInfoGets(sid, uid uint64, keys []UserInfoKey) (map[UserInfoKey]string, error) {
	var info []UserInfo
	err := d.db.Find(&info, "server_id = ? AND user_id = ? AND key IN ?", sid, uid, keys).Error
	if err != nil {
		return nil, err
	}

	values := make(map[UserInfoKey]string)
	for _, item := range info {
		values[item.Key] = item.Value
	}
	return values, nil
}
