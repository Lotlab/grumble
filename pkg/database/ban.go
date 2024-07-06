package database

import "time"

type Ban struct {
	ServerID uint64  `gorm:"not null"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	Base    []byte
	Mask    int
	Name    string
	Hash    []byte
	Reason  string
	Start   time.Time
	Duraion int
}

func (s Ban) TableName() string {
	return "bans"
}

func (d *DbTx) BanRead(sid uint64, limit, offset int) ([]Ban, int64, error) {
	var bans []Ban
	var count int64
	err := d.db.Limit(limit).Offset(offset).Find(&bans, "server_id = ?", sid).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}
	return bans, count, nil
}

func (d *DbTx) BanWrite(bans []Ban) error {
	err := d.db.Delete(&Ban{}, "TRUE").Error
	if err != nil {
		return err
	}

	return d.db.Create(bans).Error
}
