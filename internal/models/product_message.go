package models

type ProductMessage struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Inventory int    `json:"inventory"`
}
