package database

import (
	"strconv"
)

type Meta struct {
	KeyString string `gorm:"primaryKey;column:keystring"`
	Value     string
}

func (s Meta) TableName() string {
	return "meta"
}

func (p *DbTx) metaGetInt(key string) (int, error) {
	meta := Meta{
		KeyString: key,
	}
	err := p.db.First(&meta).Error
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseInt(meta.Value, 10, 32)
	return int(i), err
}

func (p *DbTx) metaSetInt(key string, val int) error {
	meta := Meta{
		KeyString: key,
		Value:     strconv.FormatInt(int64(val), 10),
	}
	return p.db.Save(&meta).Error
}

func (p *DbTx) MetaSetVersion(ver int) error {
	return p.metaSetInt("version", ver)
}

func (p *DbTx) MetaGetVersion() (int, error) {
	return p.metaGetInt("version")
}

func (p *DbTx) MetaSetKDFIter(iter int) error {
	return p.metaSetInt("pbkdf2_iterations", iter)
}

func (p *DbTx) MetaGetKDFIter() (int, error) {
	return p.metaGetInt("pbkdf2_iterations")
}
