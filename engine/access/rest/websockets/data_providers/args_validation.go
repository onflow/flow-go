package data_providers

import (
	"fmt"
)

func ensureAllowedFields(data map[string]interface{}, allowedFields []string) error {
	fieldsMap := make(map[string]bool, len(allowedFields))
	for _, field := range allowedFields {
		fieldsMap[field] = true
	}

	for key := range data {
		if !fieldsMap[key] {
			return fmt.Errorf("unexpected field: '%s'", key)
		}
	}
	return nil
}
