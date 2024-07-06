package database

type Server struct {
	ServerID uint64 `gorm:"primaryKey;autoIncrement"`
}

func (s Server) TableName() string {
	return "servers"
}

func (d *DbTx) ServerList(offset, limit int) ([]Server, error) {
	servers := make([]Server, 0)
	err := d.db.Limit(limit).Offset(offset).Find(&servers).Error
	return servers, err
}

func (d *DbTx) ServerAdd() (*Server, error) {
	var server Server
	err := d.db.Create(&server).Error
	return &server, err
}

func (d *DbTx) ServerExists(sid uint64) (bool, error) {
	var count int64
	err := d.db.Model(&Server{}).Where(&Server{ServerID: sid}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DbTx) ServerDelete(sid uint64) error {
	return d.db.Delete(&Server{ServerID: sid}).Error
}

func (d *DbTx) ServerInit(sid uint64, suPassword UserPasswordHash) (bool, error) {
	err := d.userInitSuperuser(sid, "SuperUser", suPassword)
	if err != nil {
		return false, err
	}

	channel, err := d.channelInit(sid, "Root")
	if err != nil {
		return false, err
	}

	if channel == nil {
		return false, nil
	}

	group, err := d.groupInit(sid, "admin")
	if err != nil {
		return false, err
	}

	if group == nil {
		return false, nil
	}

	err = d.GroupMemberAdd(sid, group.GroupID, 0, true)
	if err != nil {
		return false, err
	}

	return true, nil
}
