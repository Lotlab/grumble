package database

import (
	"time"
)

type ServerLog struct {
	ServerID uint64  `gorm:"not null"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`
	Msg      string
	MsgTime  time.Time `gorm:"autoCreateTime;column:msgtime;index:slog_time"`
}

func (s ServerLog) TableName() string {
	return "slog"
}

func (d *DbTx) ServerLogAdd(sid uint64, log string) error {
	logItem := ServerLog{
		ServerID: sid,
		Msg:      log,
	}
	return d.db.Create(&logItem).Error
}

func (d *DbTx) ServerLogWipe(sid uint64) error {
	return d.db.Delete(&ServerLog{}, "server_id = ?", sid).Error
}

func (d *DbTx) ServerLogGet(sid uint64, limit, offset int) ([]ServerLog, int64, error) {
	var logs []ServerLog
	var count int64
	err := d.db.Limit(limit).Offset(offset).Find(&logs, "server_id = ?", sid).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, count, nil
}
