package db

type Hall struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
	Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`
}
