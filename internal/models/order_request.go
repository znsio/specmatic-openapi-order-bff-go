package models

type OrderRequest struct {
	ProductID int `json:"productid"`
	Count     int `json:"count"`
}
