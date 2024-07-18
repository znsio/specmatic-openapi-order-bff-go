package models

type Order struct {
	ID        int    `json:"id"`
	ProductID int    `json:"productid" binding:"required"`
	Count     int    `json:"count" binding:"required,min=1"`
	Status    string `json:"status"`
}
