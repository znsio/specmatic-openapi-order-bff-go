package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type NewProduct struct {
	Name      string    `json:"name" binding:"required"`
	Type      string    `json:"type" binding:"required,oneof=book food gadget other"`
	Inventory Inventory `json:"inventory" binding:"required,min=1,max=101"`
}

/**
* Custom type for Inventory as it can be provide as a string (int as string) in json or as a string, and both should be
* accepted.
 */
type Inventory int

/**
* Custom unmarshalling logic for custom type Inventory (as it can be a string representation of int or a int in the incoming json)
 */
func (i *Inventory) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as integer
	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		*i = Inventory(intVal)
		return nil
	}

	// Try to unmarshal as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			return fmt.Errorf("invalid inventory value: %v", strVal)
		}
		*i = Inventory(intVal)
		return nil
	}

	return fmt.Errorf("invalid inventory value: %v", string(data))
}
