package database

import "database/sql"

type Channel struct {
	ServerID uint64  `gorm:"not null;uniqueIndex:channel_id"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	ChannelID uint64 `gorm:"not null;uniqueIndex:channel_id"`
	ParentID  sql.NullInt64
	Parent    *Channel `gorm:"foreignKey:ServerID,ParentID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE"`

	Name       string
	InheritACL bool `gorm:"column:inheritacl"`
}

func (s Channel) TableName() string {
	return "channels"
}

func nullInt64(val int64) sql.NullInt64 {
	if val >= 0 {
		return sql.NullInt64{
			Valid: true,
			Int64: val,
		}
	} else {
		return sql.NullInt64{
			Valid: false,
		}
	}
}

func (d *DbTx) channelInit(sid uint64, defaultName string) (*Channel, error) {
	ch, err := d.ChannelGet(sid, 0)
	if err != nil {
		return nil, err
	}

	if ch != nil {
		return nil, nil
	}

	channel := Channel{
		ServerID:  sid,
		ChannelID: 0,
		ParentID:  sql.NullInt64{},
		Name:      defaultName,
	}

	err = d.db.Create(&channel).Error
	if err != nil {
		return nil, err
	}

	return &channel, nil
}

func (d *DbTx) ChannelAdd(sid uint64, name string, parentID int64) (*Channel, error) {
	channelID := uint64(0)
	err := d.db.Model(&Channel{}).Where("server_id = ?", sid).Select("MAX(`channel_id`)+1").Scan(&channelID).Error
	if err != nil {
		return nil, err
	}

	channel := Channel{ServerID: sid, ChannelID: channelID, ParentID: nullInt64(parentID), Name: name}
	err = d.db.Create(&channel).Error
	if err != nil {
		return nil, err
	}

	return &channel, err
}

func (d *DbTx) ChannelGet(sid, cid uint64) (*Channel, error) {
	ch := Channel{}
	err := d.db.Find(&ch, "server_id = ? AND channel_id = ?", sid, cid).Error
	if err != nil {
		return nil, err
	}
	if ch.ServerID != sid || ch.ChannelID != cid {
		return nil, nil
	}
	return &ch, nil
}

func (d *DbTx) ChannelRemove(sid, cid uint64) error {
	return d.db.Delete(&Channel{}, "server_id = ? AND channel_id = ?", sid, cid).Error
}

func (d *DbTx) ChannelUpdate(sid, cid uint64, name string, parentID int64, inheritACL bool) error {
	return d.db.Where(&Channel{ServerID: sid, ChannelID: cid}).Updates(Channel{Name: name, ParentID: nullInt64(parentID), InheritACL: inheritACL}).Error
}

func (d *DbTx) ChannelListByParent(sid, parentCID uint64) ([]Channel, error) {
	var chs []Channel
	err := d.db.Find(&chs, "server_id = ? AND parent_id = ?", sid, parentCID).Order("name").Error
	if err != nil {
		return nil, err
	}
	return chs, err
}

func (d *DbTx) ChannelListNoParent(sid uint64) ([]Channel, error) {
	var chs []Channel
	err := d.db.Find(&chs, "server_id = ? AND parent_id IS NULL", sid).Order("name").Error
	if err != nil {
		return nil, err
	}
	return chs, err
}

func (d *DbTx) ChannelList(sid uint64) ([]Channel, error) {
	var chs []Channel
	err := d.db.Find(&chs, "server_id = ?", sid).Order("name").Error
	if err != nil {
		return nil, err
	}
	return chs, err
}
