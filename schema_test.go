package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	srvConfig "github.com/CHESSComputing/golib/config"
)

// TestSchemaYaml tests schema yaml file
func TestSchemaYaml(t *testing.T) {
	config := os.Getenv("FOXDEN_CONFIG")
	if cobj, err := srvConfig.ParseConfig(config); err == nil {
		srvConfig.Config = &cobj
	}
	tmpFile, err := os.CreateTemp(os.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	yamlData := `
- key: Pi
  optional: true
  type: string
- key: BeamEnergy
  optional: false
  type: int
`
	tmpFile.Write([]byte(yamlData))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// load json data
	fname := tmpFile.Name()
	s := &Schema{FileName: fname}
	err = s.Load()
	if err != nil {
		t.Fatal(err)
	}

	keys, err := s.Keys()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Schema keys", keys)
	okeys, err := s.OptionalKeys()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Schema optional keys", okeys)

	rec := make(map[string]any)
	rec["Pi"] = "person"
	rec["BeamEnergy"] = 123
	err = s.Validate(rec)
	if err != nil {
		t.Fatal(err)
	}
}

// TestSchemaJson tests schema json file
func TestSchemaJson(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "*.json")
	if err != nil {
		t.Fatal(err)
	}
	jsonData := `[
    {"key": "Pi", "type": "string", "optional": true},
    {"key": "BeamEnergy", "type": "int", "optional": false}
]`
	tmpFile.Write([]byte(jsonData))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// load json data
	fname := tmpFile.Name()
	s := &Schema{FileName: fname}
	err = s.Load()
	if err != nil {
		t.Fatal(err)
	}

	keys, err := s.Keys()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Schema keys", keys)
	okeys, err := s.OptionalKeys()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Schema optional keys", okeys)

	rec := make(map[string]any)
	rec["Pi"] = "person"
	rec["BeamEnergy"] = 123
	err = s.Validate(rec)
	if err != nil {
		t.Fatal(err)
	}
}

// TestLoadSchemaWithInclude tests loading a schema that includes another file
func TestLoadSchemaWithInclude(t *testing.T) {
	// Create temporary directory for schema files
	tempDir := t.TempDir()
	firstPath := filepath.Join(tempDir, "first_schema.json")
	secondPath := filepath.Join(tempDir, "second_schema.json")

	// Content of first_schema.json
	firstSchema := `[
		{
			"key": "did",
			"type": "string",
			"optional": true,
			"multiple": false,
			"section": "User",
			"description": "Dataset IDentifier",
			"units": "",
			"placeholder": "/beamline=demo/btr=user-1234-a/cycle=2025-2/sample_name=testsample"
		}
	]`

	// Content of second_schema.json (includes first)
	secondSchema := fmt.Sprintf("[{ \"file\": \"%s\" },", firstPath)
	secondSchema += `
		{
			"key": "new",
			"type": "string",
			"optional": true,
			"multiple": false,
			"section": "User",
			"description": "New field",
			"units": "",
			"placeholder": "/new"
		}
	]`

	// Write first schema
	if err := os.WriteFile(firstPath, []byte(firstSchema), 0644); err != nil {
		t.Fatalf("Failed to write first schema: %v", err)
	}

	// Write second schema
	if err := os.WriteFile(secondPath, []byte(secondSchema), 0644); err != nil {
		t.Fatalf("Failed to write second schema: %v", err)
	}

	// Load second schema (which includes the first)
	schema := &Schema{FileName: secondPath}
	err := schema.Load()
	if err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	// Expect 2 fields: "did" from first file, "new" from second
	keys, err := schema.Keys()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Schema keys", keys)
	if len(keys) != 2 {
		t.Errorf("Expected 2 schema fields, got %d", len(keys))
	}

	rec := make(map[string]any)
	rec["did"] = "did key"
	rec["new"] = "new key"

	found := map[string]bool{
		"did": false,
		"new": false,
	}

	for _, key := range keys {
		if _, ok := found[key]; ok {
			found[key] = true
		}
	}

	for key, wasFound := range found {
		if !wasFound {
			t.Errorf("Expected key %q not found in parsed schema", key)
		}
	}
}
