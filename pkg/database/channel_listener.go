package database

type ChannelListener struct {
	ServerID uint64  `gorm:"not null;"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	ChannelID uint64   `gorm:"not null"`
	Channel   *Channel `gorm:"belongsTo;foreignKey:ServerID,ChannelID;references:ServerID,ChannelID;constraint:OnDelete:CASCADE;"`

	UserID uint64 `gorm:"not null"`
	User   *User  `gorm:"belongsTo;foreignKey:ServerID,UserID;references:ServerID,UserID;constraint:OnDelete:CASCADE;"`

	VolumeAdjustment float32
	Enabled          bool
}

func (s ChannelListener) TableName() string {
	return "channel_listeners"
}

func (d *DbTx) ChannelListenerLoadByUser(sid, uid uint64) ([]ChannelListener, error) {
	var chList []ChannelListener
	err := d.db.Find(&chList, "server_id = ? AND user_id = ?", sid, uid).Error
	if err != nil {
		return nil, err
	}
	return chList, err
}

func (d *DbTx) ChannelListenerAdd(sid, uid, cid uint64) (*ChannelListener, error) {
	chl := ChannelListener{
		ServerID:         sid,
		ChannelID:        cid,
		UserID:           uid,
		VolumeAdjustment: 1,
		Enabled:          true,
	}
	var count int64
	err := d.db.Find(&chl, "server_id = ? AND user_id = ? AND channel_id = ?", sid, uid, cid).Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		chl.Enabled = true
		err = d.db.Model(&chl).Where("server_id = ? AND user_id = ? AND channel_id = ?", sid, uid, cid).Update("enabled", true).Error
	} else {
		err = d.db.Create(&chl).Error
	}

	if err != nil {
		return nil, err
	}
	return &chl, err
}

func (d *DbTx) ChannelListenerSetEnabled(sid, uid, cid uint64, enabled bool) error {
	return d.db.Model(&ChannelListener{}).Where("server_id = ? AND user_id = ? AND channel_id = ?", sid, uid, cid).Update("enabled", enabled).Error
}

func (d *DbTx) ChannelListenerSetVolume(sid, uid, cid uint64, volume float32) error {
	return d.db.Model(&ChannelListener{}).Where("server_id = ? AND user_id = ? AND channel_id = ?", sid, uid, cid).Update("volume_adjustment", volume).Error
}

func (d *DbTx) ChannelListenerDelete(sid, uid, cid uint64) error {
	return d.db.Delete(&ChannelListener{}, "server_id = ? AND user_id = ? AND channel_id = ?", sid, uid, cid).Error
}
