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

func (d *DbTx) BanRead(sid uint64) ([]Ban, error) {
	var bans []Ban
	err := d.db.Find(&bans, "server_id = ?", sid).Error
	if err != nil {
		return nil, err
	}
	return bans, nil
}

func (d *DbTx) BanWrite(bans []Ban) error {
	err := d.db.Delete(&Ban{}, "TRUE").Error
	if err != nil {
		return err
	}

	return d.db.Create(bans).Error
}
