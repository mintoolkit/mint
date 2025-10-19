package http

import (
	"net/url"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSubstitutePathParams(t *testing.T) {
	apiPath := "/pets/{id}/owners/{ownerId}"

	params := []*openapi3.Parameter{
		{
			Name: "id",
			In:   "path",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: "integer",
			}},
		},
		{
			Name: "ownerId",
			In:   "path",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: "string",
			}},
		},
	}

	got := substitutePathParams(apiPath, params)
	want := "/pets/1/owners/x"
	if got != want {
		t.Fatalf("substitutePathParams() = %q; want %q", got, want)
	}
}

func TestBuildQueryAndHeaders(t *testing.T) {
	params := []*openapi3.Parameter{
		{
			Name: "q",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: "string",
			}},
		},
		{
			Name: "count",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: "integer",
			}},
		},
		{
			Name: "meta",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type:       "object",
				Properties: openapi3.Schemas{"x": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}},
			}},
		},
		{
			Name: "X-Token",
			In:   "header",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: "string",
			}},
		},
	}

	qs, headers := buildQueryAndHeaders(params)
	values, err := url.ParseQuery(qs)
	if err != nil {
		t.Fatalf("failed to parse query string: %v", err)
	}
	if values.Get("q") != "x" {
		t.Fatalf("query param q = %q; want %q", values.Get("q"), "x")
	}
	if values.Get("count") != "1" {
		t.Fatalf("query param count = %q; want %q", values.Get("count"), "1")
	}
	if headers["X-Token"] != "x" {
		t.Fatalf("header X-Token = %q; want %q", headers["X-Token"], "x")
	}

	// object param should be JSON-stringified
	if values.Get("meta") == "" {
		t.Fatalf("expected meta query param to be present")
	}
	if !strings.HasPrefix(values.Get("meta"), "{") {
		t.Fatalf("expected meta to be JSON string, got: %q", values.Get("meta"))
	}
}

func TestCollectParameters_MergeAndOverride(t *testing.T) {
	pathItem := &openapi3.PathItem{
		Parameters: openapi3.Parameters{
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "x", In: "query", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}}},
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "id", In: "path", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}}}},
		},
	}
	op := &openapi3.Operation{
		Parameters: openapi3.Parameters{
			// override x
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "x", In: "query", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}}}},
			// add new y
			&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "y", In: "query", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "boolean"}}}},
		},
	}

	merged := collectParameters(pathItem, op)
	get := func(in, name string) *openapi3.Parameter {
		for _, p := range merged {
			if p != nil && p.In == in && p.Name == name {
				return p
			}
		}
		return nil
	}

	if p := get("query", "x"); p == nil || p.Schema == nil || p.Schema.Value == nil || p.Schema.Value.Type != "integer" {
		t.Fatalf("expected op-level override for x to be integer, got: %#v", p)
	}
	if p := get("query", "y"); p == nil || p.Schema == nil || p.Schema.Value == nil || p.Schema.Value.Type != "boolean" {
		t.Fatalf("expected new param y boolean, got: %#v", p)
	}
	if p := get("path", "id"); p == nil || p.Schema == nil || p.Schema.Value == nil || p.Schema.Value.Type != "integer" {
		t.Fatalf("expected path id integer preserved, got: %#v", p)
	}
}

func TestParamStringForSchema_TypesAndEnum(t *testing.T) {
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Enum: []interface{}{"A", "B"}}}); got != "A" {
		t.Fatalf("enum preferred value = %q; want %q", got, "A")
	}
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}}); got != "1" {
		t.Fatalf("integer -> %q; want %q", got, "1")
	}
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Type: "number"}}); got != "1" {
		t.Fatalf("number -> %q; want %q", got, "1")
	}
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Type: "boolean"}}); got != "true" {
		t.Fatalf("boolean -> %q; want %q", got, "true")
	}
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Type: "array", Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}}}}); got != "1" {
		t.Fatalf("array[integer] -> %q; want %q", got, "1")
	}
	if got := paramStringForSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{Type: "object"}}); got != "x" {
		t.Fatalf("object -> %q; want %q", got, "x")
	}
}

func TestSubstitutePathParams_FallbackStripsUnknown(t *testing.T) {
	apiPath := "/stores/{known}/items/{unknown}"
	params := []*openapi3.Parameter{
		{Name: "known", In: "path", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}},
	}
	got := substitutePathParams(apiPath, params)
	want := "/stores/x/items/unknown"
	if got != want {
		t.Fatalf("fallback strip = %q; want %q", got, want)
	}
}

func TestBuildQueryAndHeaders_ArrayHandling(t *testing.T) {
	params := []*openapi3.Parameter{
		{Name: "ids", In: "query", Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "array", Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}}}}},
	}
	qs, _ := buildQueryAndHeaders(params)
	values, err := url.ParseQuery(qs)
	if err != nil {
		t.Fatalf("failed to parse query string: %v", err)
	}
	if values.Get("ids") != "1" {
		t.Fatalf("array item -> %q; want %q", values.Get("ids"), "1")
	}
}

