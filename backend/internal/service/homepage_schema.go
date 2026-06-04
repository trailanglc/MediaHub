package service

import (
	"encoding/json"
	"fmt"
)

const (
	maxSchemaCustomBytes = 16 * 1024
	maxSchemaCustomItems = 10
)

func normalizeSchemaCustom(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) == 0 {
			return json.RawMessage("[]")
		}
		out, err := json.Marshal(arr)
		if err != nil {
			return json.RawMessage("[]")
		}
		return out
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return json.RawMessage("[]")
	}
	out, err := json.Marshal([]json.RawMessage{raw})
	if err != nil {
		return json.RawMessage("[]")
	}
	return out
}

func validateSchemaCustom(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if len(raw) > maxSchemaCustomBytes {
		return fmt.Errorf("%w: homepage.schema_custom", ErrInvalidSettings)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		var obj map[string]json.RawMessage
		if err2 := json.Unmarshal(raw, &obj); err2 != nil || len(obj) == 0 {
			return fmt.Errorf("%w: homepage.schema_custom", ErrInvalidSettings)
		}
		items = []json.RawMessage{raw}
	}
	if len(items) > maxSchemaCustomItems {
		return fmt.Errorf("%w: homepage.schema_custom", ErrInvalidSettings)
	}
	for _, item := range items {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(item, &obj); err != nil || len(obj) == 0 {
			return fmt.Errorf("%w: homepage.schema_custom", ErrInvalidSettings)
		}
	}
	return nil
}
