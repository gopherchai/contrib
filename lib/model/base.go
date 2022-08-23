package model

import "time"

type BaseModel interface {
	TableName() string
	SetID(id int64)
	GetID() int64
}

type OrmCommon struct {
	CreatorUserId    int64     `json:"creatorUserId"`
	MaintainerUserId int64     `json:"maintainerUserId"`
	CreatedAt        time.Time `orm:"auto_now_add;type(datetime)" json:"createdAt"`
	UpdatedAt        time.Time `orm:"auto_now;type(datetime)" json:"updatedAt"`
	IsDeleted        bool      `json:"isDeleted"`
}
