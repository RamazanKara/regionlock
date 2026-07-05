package regmap

import (
	"bytes"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileSchema loads a JSON Schema file and compiles it.
func compileSchema(t *testing.T, path, id string) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse schema %s: %v", path, err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	sch, err := c.Compile(id)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return sch
}

// TestBundledRulesetsMatchSchema validates every embedded ruleset JSON against
// the published ruleset schema, so the schema and the data can never drift.
func TestBundledRulesetsMatchSchema(t *testing.T) {
	sch := compileSchema(t, "../../schemas/ruleset.schema.json", "ruleset.schema.json")
	for id, raw := range rulesets {
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: parse ruleset: %v", id, err)
		}
		if err := sch.Validate(inst); err != nil {
			t.Errorf("%s: does not match schemas/ruleset.schema.json:\n%v", id, err)
		}
	}
}
