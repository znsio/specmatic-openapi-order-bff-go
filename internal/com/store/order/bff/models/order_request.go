package models

type OrderRequest struct {
	ProductID int `json:"productid" binding:"required"`
	Count     int `json:"count" binding:"required"`
}
