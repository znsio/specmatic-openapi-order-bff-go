package models

type ProductType string

const (
	TypeBook   ProductType = "book"
	TypeFood   ProductType = "food"
	TypeGadget ProductType = "gadget"
	TypeOther  ProductType = "other"
)

type Product struct {
	ID        int         `json:"id"`
	Name      string      `json:"name" binding:"required"`
	Type      ProductType `json:"type" binding:"required,oneof=book food gadget other"`
	Inventory int         `json:"inventory" binding:"required,min=1,max=101"`
}
