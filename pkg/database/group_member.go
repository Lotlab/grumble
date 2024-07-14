package database

import "gorm.io/gorm/clause"

type GroupMember struct {
	ServerID uint64  `gorm:"not null;index:group_members_users"`
	Server   *Server `gorm:"constraint:OnDelete:CASCADE;"`

	GroupID uint64 `gorm:"not null"`
	Group   *Group `gorm:"foreignKey:ServerID,GroupID;references:ServerID,GroupID;constraint:OnDelete:CASCADE;"`
	UserID  uint64 `gorm:"not null;index:group_members_users"`
	User    *User  `gorm:"belongsTo;foreignKey:ServerID,UserID;references:ServerID,UserID;constraint:OnDelete:CASCADE;"`

	Addit bool // Member should be include or exclude for group
}

func (s GroupMember) TableName() string {
	return "group_members"
}

func (d *DbTx) GroupMemberList(sid, gid uint64) ([]GroupMember, error) {
	var members []GroupMember
	err := d.db.Find(&members, "server_id = ? AND group_id = ?", sid, gid).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (d *DbTx) GroupMemberAdd(sid, gid, uid uint64, addit bool) error {
	return d.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&GroupMember{ServerID: sid, GroupID: gid, UserID: uid, Addit: addit}).Error
}

func (d *DbTx) GroupMemberRemove(sid, gid, uid uint64) error {
	return d.db.Delete(&GroupMember{}, "server_id = ? AND group_id = ? AND user_id = ?", sid, gid, uid).Error
}

func (d *DbTx) GroupMemberRemoveAll(sid, gid uint64) error {
	return d.db.Delete(&GroupMember{}, "server_id = ? AND group_id = ?", sid, gid).Error
}
