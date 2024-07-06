package database

type ChannelLink struct {
	ServerID uint64
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	ChannelID uint64   `gorm:"not null"`
	Channel   *Channel `gorm:"belongsTo;foreignKey:ServerID,ChannelID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE;"`

	LinkID uint64 `gorm:"not null"`
}

func (s ChannelLink) TableName() string {
	return "channel_links"
}

func (d *DbTx) ChannelLinkAdd(sid, cid, lid uint64) error {
	return d.db.Create(&ChannelLink{ServerID: sid, ChannelID: cid, LinkID: lid}).Error
}

func (d *DbTx) ChannelLinkRemove(sid, cid, lid uint64) error {
	return d.db.Delete(&ChannelLink{}, "server_id = ? AND channel_id = ? AND link_id = ?", sid, cid, lid).Error
}

func (d *DbTx) ChannelLinkList(sid uint64, limit, offset int) ([]ChannelLink, int64, error) {
	var links []ChannelLink
	var count int64
	err := d.db.Limit(limit).Offset(offset).Find(&links, "server_id = ?", sid).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}
	return links, count, nil
}
