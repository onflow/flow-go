package data_providers

import (
	"fmt"
)

func ensureAllowedFields(fields map[string]interface{}, allowedFields map[string]struct{}) error {
	// Ensure only allowed fields are present
	for key := range fields {
		if _, exists := allowedFields[key]; !exists {
			return fmt.Errorf("unexpected field: '%s'", key)
		}
	}

	return nil
}
