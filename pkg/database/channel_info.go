package database

import "gorm.io/gorm/clause"

type ChannelInfoKey int

const (
	ChannelDescription ChannelInfoKey = iota
	ChannelPosition
	ChannelMaxUsers
)

type ChannelInfo struct {
	ServerID uint64  `gorm:"not null;uniqueIndex:channel_info_id"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	ChannelID uint64         `gorm:"not null;uniqueIndex:channel_info_id"`
	Channel   *Channel       `gorm:"belongsTo;foreignKey:ServerID,ChannelID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE;"`
	Key       ChannelInfoKey `gorm:"uniqueIndex:channel_info_id"`
	Value     string
}

func (s ChannelInfo) TableName() string {
	return "channel_info"
}

func (d *DbTx) ChannelInfoSet(sid, cid uint64, info map[ChannelInfoKey]string) error {
	for k, v := range info {
		err := d.db.Clauses(&clause.OnConflict{
			UpdateAll: true,
		}).Create(&ChannelInfo{
			ServerID:  sid,
			ChannelID: cid,
			Key:       k,
			Value:     v,
		}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DbTx) ChannelInfoGet(sid, cid uint64, key ChannelInfoKey) (string, error) {
	info := ChannelInfo{}
	err := d.db.Where("server_id = ? AND channel_id = ? AND key = ?", sid, cid, key).Find(&info).Error
	return info.Value, err
}

func (d *DbTx) ChannelInfoGets(sid, cid uint64, keys []ChannelInfoKey) (map[ChannelInfoKey]string, error) {
	var info []ChannelInfo
	err := d.db.Find(&info, "server_id = ? AND channel_id = ? AND key IN ?", sid, cid, keys).Error
	if err != nil {
		return nil, err
	}

	values := make(map[ChannelInfoKey]string)
	for _, item := range info {
		values[item.Key] = item.Value
	}
	return values, nil
}
