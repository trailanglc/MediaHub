package service

import (
	"encoding/json"
	"testing"
)

func TestValidateSchemaCustom(t *testing.T) {
	if err := validateSchemaCustom(json.RawMessage("[]")); err != nil {
		t.Fatalf("empty array: %v", err)
	}

	valid := json.RawMessage(`[{"@context":"https://schema.org","@type":"Organization","name":"Acme"}]`)
	if err := validateSchemaCustom(valid); err != nil {
		t.Fatalf("valid array: %v", err)
	}

	single := json.RawMessage(`{"@type":"WebSite","name":"Test"}`)
	if err := validateSchemaCustom(single); err != nil {
		t.Fatalf("valid single object: %v", err)
	}

	tooMany := make([]map[string]string, maxSchemaCustomItems+1)
	for i := range tooMany {
		tooMany[i] = map[string]string{"@type": "Thing", "name": "x"}
	}
	raw, _ := json.Marshal(tooMany)
	if err := validateSchemaCustom(raw); err == nil {
		t.Fatal("expected error for too many items")
	}

	if err := validateSchemaCustom(json.RawMessage(`"not-an-object"`)); err == nil {
		t.Fatal("expected error for non-object")
	}
}

func TestNormalizeSchemaCustom(t *testing.T) {
	got := normalizeSchemaCustom(json.RawMessage(`{"@type":"WebSite"}`))
	var arr []json.RawMessage
	if err := json.Unmarshal(got, &arr); err != nil || len(arr) != 1 {
		t.Fatalf("expected single-item array, got %s", got)
	}
}
