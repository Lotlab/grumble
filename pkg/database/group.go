package database

type Group struct {
	ServerID uint64  `gorm:"not null;uniqueIndex:groups_name_channels"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	GroupID     uint64   `gorm:"primaryKey;autoIncrement"`
	Name        string   `gorm:"uniqueIndex:groups_name_channels"`
	ChannelID   uint64   `gorm:"not null;uniqueIndex:groups_name_channels"`
	Channel     *Channel `gorm:"foreignKey:ServerID,ChannelID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE;"`
	Inherit     bool
	Inheritable bool
}

func (s Group) TableName() string {
	return "groups"
}

func (d *DbTx) groupInit(sid uint64, name string) (*Group, error) {
	var count int64
	err := d.db.Model(&Group{}).Where("server_id = ? AND channel_id = ?", sid, 0).Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}
	return d.GroupAdd(sid, 0, name, true, true)
}

func (d *DbTx) GroupAdd(sid, cid uint64, name string, inherit, inheritable bool) (*Group, error) {
	group := Group{
		ServerID:    sid,
		ChannelID:   cid,
		Name:        name,
		Inherit:     inherit,
		Inheritable: inheritable,
	}
	err := d.db.Create(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (d *DbTx) GroupGetByGID(sid, gid uint64) (*Group, error) {
	group := Group{}
	err := d.db.Find(&group, "server_id = ? AND group_id = ?", sid, gid).Error
	if err != nil {
		return nil, err
	}
	if group.ServerID != sid || group.GroupID != gid {
		return nil, nil
	}
	return &group, nil
}

func (d *DbTx) GroupGetByChannel(sid, cid uint64) ([]Group, error) {
	var groups []Group
	err := d.db.Find(&groups, "server_id = ? AND channel_id = ?", sid, cid).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (d *DbTx) GroupDeleteByChannel(sid, cid uint64) error {
	return d.db.Delete(&Group{}, "server_id = ? AND channel_id = ?", sid, cid).Error
}

func (d *DbTx) GroupDelete(sid, gid uint64) error {
	return d.db.Delete(&Group{}, "server_id = ? AND group_id = ?", sid, gid).Error
}

func (d *DbTx) GroupModify(group *Group) error {
	return d.db.Save(group).Error
}
