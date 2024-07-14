package database

import "time"

type User struct {
	ServerID uint64  `gorm:"not null;index:users_channel;uniqueIndex:users_name;uniqueIndex:users_id"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	UserID      uint64           `gorm:"not null;index:users_id"`
	Name        string           `gorm:"not null;index:users_name"`
	Password    UserPasswordHash `gorm:"embedded"`
	LastChannel uint64           `gorm:"column:lastchannel;index:users_channel"`

	Texture        []byte
	LastActive     time.Time
	LastDisconnect time.Time
}

type UserPasswordHash struct {
	Hash string `gorm:"column:pw"`
	Salt []byte `gorm:"column:salt"`

	KDFIterations int `gorm:"column:kdfiterations"`
}

func (s User) TableName() string {
	return "users"
}

func (d *DbTx) userInitSuperuser(sid uint64, name string, pass UserPasswordHash) error {
	exist, err := d.UserExists(sid, 0)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	return d.db.Create(&User{
		ServerID: sid,
		UserID:   0,
		Name:     name,
		Password: pass,
	}).Error
}

func (d *DbTx) UserRegister(sid uint64, name string) (*User, error) {
	userID := 0
	err := d.db.Model(&User{}).Where("server_id = ?", sid).Select("MAX(`user_id`)+1").Scan(&userID).Error
	if err != nil {
		return nil, err
	}

	user := User{
		ServerID: sid,
		UserID:   uint64(userID),
		Name:     name,
	}
	err = d.db.Create(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *DbTx) UserUnregister(sid uint64, uid uint64) error {
	err := d.db.Where("server_id = ? AND user_id = ?", sid, uid).Delete(&User{}).Error
	if err != nil {
		return err
	}

	err = d.db.Where("server_id = ? AND user_id = ?", sid, uid).Delete(&UserInfo{}).Error
	return err
}

func (d *DbTx) UserList(sid uint64, limit, offset int) ([]User, int64, error) {
	var users []User
	var count int64
	err := d.db.Limit(limit).Offset(offset).Select("user_id", "name", "lastchannel", "last_active").Find(&users, "server_id = ?", sid).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}
	return users, count, err
}

func (d *DbTx) UserFind(sid uint64, query string) ([]User, error) {
	var users []User
	err := d.db.Select("server_id", "user_id", "name").Find(&users, "server_id = ? AND name LIKE ?", sid, query).Error
	return users, err
}

func (d *DbTx) UserExists(sid uint64, uid uint64) (bool, error) {
	count := int64(0)
	err := d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Count(&count).Error
	return count > 0, err
}

func (d *DbTx) UserGetInfo(sid uint64, uid uint64) (*User, error) {
	user := User{}
	err := d.db.Select("server_id", "user_id", "name", "last_active").Find(&user, "server_id = ? AND user_id = ?", sid, uid).Error
	if err != nil {
		return nil, err
	}
	if user.ServerID != sid || user.UserID != uid {
		return nil, nil
	}
	return &user, err
}

func (d *DbTx) UserGetID(sid uint64, name string) (*User, error) {
	user := User{}
	err := d.db.Select("server_id", "user_id").Find(&user, "server_id = ? AND name = ?", sid, name).Error
	if err != nil {
		return nil, err
	}
	if user.ServerID != sid {
		return nil, nil
	}
	return &user, err
}

func (d *DbTx) UserGetAuth(sid uint64, name string) (*User, error) {
	user := User{}
	err := d.db.Select("user_id", "name", "pw", "salt", "kdfiterations").Find(&user, "server_id = ? AND name = ?", sid, name).Error
	if err != nil {
		return nil, err
	}
	if user.Name != name {
		return nil, nil
	}
	return &user, err
}

func (d *DbTx) UserGetLastChannel(sid, uid uint64) (*User, error) {
	user := User{}
	err := d.db.Select("server_id", "user_id", "lastchannel", "last_active", "last_disconnect").Find(&user, "server_id = ? AND user_id = ?", sid, uid).Error
	if err != nil {
		return nil, err
	}
	if user.ServerID != sid || user.UserID != uid {
		return nil, nil
	}
	return &user, err
}
func (d *DbTx) UserSetTexture(sid, uid uint64, texture []byte) error {
	return d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Updates(User{Texture: texture}).Error
}

func (d *DbTx) UserGetTexture(sid uint64, uid uint64) (*User, error) {
	user := User{}
	err := d.db.Select("server_id", "user_id", "texture").Find(&user, "server_id = ? AND user_id = ?", sid, uid).Error
	if err != nil {
		return nil, err
	}
	if user.ServerID != sid || user.UserID != uid {
		return nil, nil
	}
	return &user, err
}

func (d *DbTx) UserSetAuth(sid, uid uint64, pw UserPasswordHash) error {
	return d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Updates(User{Password: pw}).Error
}

func (d *DbTx) UserSetLastChannel(sid, uid, cid uint64) error {
	return d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Updates(User{LastChannel: cid, LastActive: time.Now()}).Error
}

func (d *DbTx) UserSetLastDisconnect(sid, uid uint64) error {
	return d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Updates(User{LastDisconnect: time.Now()}).Error
}

func (d *DbTx) UserRename(sid, uid uint64, name string) error {
	return d.db.Model(&User{}).Where("server_id = ? AND user_id = ?", sid, uid).Updates(User{Name: name}).Error
}
