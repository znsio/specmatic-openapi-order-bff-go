package models // Assuming your models reside in a package named "models"

type ProductMessage struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Inventory  int               `json:"inventory"`
	Categories []ProductCategory `json:"categories,omitempty"` // omitempty tag ensures empty slices are omitted during marshalling
}
