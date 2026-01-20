// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConvertToAttrValue_Nil(t *testing.T) {
	result, err := convertToAttrValue(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsNull() {
		t.Error("expected null value for nil input")
	}
}

func TestConvertToAttrValue_String(t *testing.T) {
	result, err := convertToAttrValue("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	strVal, ok := result.(types.String)
	if !ok {
		t.Fatalf("expected types.String, got %T", result)
	}

	if strVal.ValueString() != "hello" {
		t.Errorf("expected 'hello', got %q", strVal.ValueString())
	}
}

func TestConvertToAttrValue_Float64(t *testing.T) {
	result, err := convertToAttrValue(float64(3.14))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	floatVal, ok := result.(types.Float64)
	if !ok {
		t.Fatalf("expected types.Float64, got %T", result)
	}

	if floatVal.ValueFloat64() != 3.14 {
		t.Errorf("expected 3.14, got %f", floatVal.ValueFloat64())
	}
}

func TestConvertToAttrValue_Int(t *testing.T) {
	result, err := convertToAttrValue(int(42))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	intVal, ok := result.(types.Int64)
	if !ok {
		t.Fatalf("expected types.Int64, got %T", result)
	}

	if intVal.ValueInt64() != 42 {
		t.Errorf("expected 42, got %d", intVal.ValueInt64())
	}
}

func TestConvertToAttrValue_Int64(t *testing.T) {
	result, err := convertToAttrValue(int64(9999999999))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	intVal, ok := result.(types.Int64)
	if !ok {
		t.Fatalf("expected types.Int64, got %T", result)
	}

	if intVal.ValueInt64() != 9999999999 {
		t.Errorf("expected 9999999999, got %d", intVal.ValueInt64())
	}
}

func TestConvertToAttrValue_Bool(t *testing.T) {
	tests := []struct {
		input    bool
		expected bool
	}{
		{true, true},
		{false, false},
	}

	for _, tt := range tests {
		result, err := convertToAttrValue(tt.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		boolVal, ok := result.(types.Bool)
		if !ok {
			t.Fatalf("expected types.Bool, got %T", result)
		}

		if boolVal.ValueBool() != tt.expected {
			t.Errorf("expected %v, got %v", tt.expected, boolVal.ValueBool())
		}
	}
}

func TestConvertToAttrValue_EmptySlice(t *testing.T) {
	result, err := convertToAttrValue([]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listVal, ok := result.(types.List)
	if !ok {
		t.Fatalf("expected types.List, got %T", result)
	}

	if !listVal.IsNull() {
		t.Error("expected null list for empty slice")
	}
}

func TestConvertToAttrValue_StringSlice(t *testing.T) {
	result, err := convertToAttrValue([]any{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listVal, ok := result.(types.List)
	if !ok {
		t.Fatalf("expected types.List, got %T", result)
	}

	elements := listVal.Elements()
	if len(elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elements))
	}

	for i, expected := range []string{"a", "b", "c"} {
		strVal, ok := elements[i].(types.String)
		if !ok {
			t.Fatalf("element %d: expected types.String, got %T", i, elements[i])
		}
		if strVal.ValueString() != expected {
			t.Errorf("element %d: expected %q, got %q", i, expected, strVal.ValueString())
		}
	}
}

func TestConvertToAttrValue_EmptyMap(t *testing.T) {
	result, err := convertToAttrValue(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapVal, ok := result.(types.Map)
	if !ok {
		t.Fatalf("expected types.Map, got %T", result)
	}

	if !mapVal.IsNull() {
		t.Error("expected null map for empty input")
	}
}

func TestConvertToAttrValue_MapStringAny(t *testing.T) {
	input := map[string]any{
		"name":  "test",
		"count": float64(42),
	}

	result, err := convertToAttrValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	objVal, ok := result.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", result)
	}

	attrs := objVal.Attributes()
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}

	nameVal, ok := attrs["name"].(types.String)
	if !ok {
		t.Fatalf("expected name to be types.String, got %T", attrs["name"])
	}
	if nameVal.ValueString() != "test" {
		t.Errorf("expected name='test', got %q", nameVal.ValueString())
	}

	countVal, ok := attrs["count"].(types.Float64)
	if !ok {
		t.Fatalf("expected count to be types.Float64, got %T", attrs["count"])
	}
	if countVal.ValueFloat64() != 42 {
		t.Errorf("expected count=42, got %f", countVal.ValueFloat64())
	}
}

func TestConvertToAttrValue_EmptyMapStringString(t *testing.T) {
	result, err := convertToAttrValue(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapVal, ok := result.(types.Map)
	if !ok {
		t.Fatalf("expected types.Map, got %T", result)
	}

	if !mapVal.IsNull() {
		t.Error("expected null map for empty input")
	}
}

func TestConvertToAttrValue_MapStringString(t *testing.T) {
	input := map[string]string{
		"Environment": "test",
		"Owner":       "probe",
	}

	result, err := convertToAttrValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapVal, ok := result.(types.Map)
	if !ok {
		t.Fatalf("expected types.Map, got %T", result)
	}

	elements := mapVal.Elements()
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	envVal, ok := elements["Environment"].(types.String)
	if !ok {
		t.Fatalf("expected Environment to be types.String, got %T", elements["Environment"])
	}
	if envVal.ValueString() != "test" {
		t.Errorf("expected Environment='test', got %q", envVal.ValueString())
	}
}

func TestConvertToAttrValue_UnknownType(t *testing.T) {
	// Custom struct should fall back to string representation
	type customType struct {
		Value int
	}

	result, err := convertToAttrValue(customType{Value: 123})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	strVal, ok := result.(types.String)
	if !ok {
		t.Fatalf("expected types.String for fallback, got %T", result)
	}

	// The exact format depends on fmt.Sprintf("%v", val)
	if strVal.ValueString() != "{123}" {
		t.Errorf("unexpected string representation: %q", strVal.ValueString())
	}
}

func TestConvertMapToDynamic_Empty(t *testing.T) {
	result, diags := convertMapToDynamic(map[string]any{})

	if diags.HasError() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if !result.IsNull() {
		t.Error("expected null dynamic for empty map")
	}
}

func TestConvertMapToDynamic_WithData(t *testing.T) {
	input := map[string]any{
		"TableName": "test-table",
		"Arn":       "arn:aws:dynamodb:us-east-1:123456789012:table/test-table",
	}

	result, diags := convertMapToDynamic(input)

	if diags.HasError() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if result.IsNull() {
		t.Error("expected non-null dynamic value")
	}

	// Verify the underlying value
	underlyingVal := result.UnderlyingValue()
	objVal, ok := underlyingVal.(types.Object)
	if !ok {
		t.Fatalf("expected underlying value to be types.Object, got %T", underlyingVal)
	}

	attrs := objVal.Attributes()
	tableNameVal, ok := attrs["TableName"].(types.String)
	if !ok {
		t.Fatalf("expected TableName to be types.String, got %T", attrs["TableName"])
	}
	if tableNameVal.ValueString() != "test-table" {
		t.Errorf("expected TableName='test-table', got %q", tableNameVal.ValueString())
	}
}

func TestConvertToAttrValue_NestedMap(t *testing.T) {
	input := map[string]any{
		"outer": map[string]any{
			"inner": "value",
		},
	}

	result, err := convertToAttrValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	objVal, ok := result.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", result)
	}

	attrs := objVal.Attributes()
	outerVal, ok := attrs["outer"].(types.Object)
	if !ok {
		t.Fatalf("expected outer to be types.Object, got %T", attrs["outer"])
	}

	outerAttrs := outerVal.Attributes()
	innerVal, ok := outerAttrs["inner"].(types.String)
	if !ok {
		t.Fatalf("expected inner to be types.String, got %T", outerAttrs["inner"])
	}
	if innerVal.ValueString() != "value" {
		t.Errorf("expected inner='value', got %q", innerVal.ValueString())
	}
}

func TestConvertToAttrValue_SliceOfMaps(t *testing.T) {
	input := []any{
		map[string]any{"key": "value1"},
		map[string]any{"key": "value2"},
	}

	result, err := convertToAttrValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listVal, ok := result.(types.List)
	if !ok {
		t.Fatalf("expected types.List, got %T", result)
	}

	elements := listVal.Elements()
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	// Verify first element
	elem0, ok := elements[0].(types.Object)
	if !ok {
		t.Fatalf("expected element 0 to be types.Object, got %T", elements[0])
	}
	elem0Attrs := elem0.Attributes()
	keyVal, ok := elem0Attrs["key"].(types.String)
	if !ok {
		t.Fatalf("expected key to be types.String, got %T", elem0Attrs["key"])
	}
	if keyVal.ValueString() != "value1" {
		t.Errorf("expected key='value1', got %q", keyVal.ValueString())
	}
}

func TestConvertToAttrValue_MixedSlice(t *testing.T) {
	// This tests the case where list elements have different types
	// The function uses the first element's type for the list type
	input := []any{"string", float64(42)}

	result, err := convertToAttrValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listVal, ok := result.(types.List)
	if !ok {
		t.Fatalf("expected types.List, got %T", result)
	}

	// Verify it has the expected element type (string from first element)
	elemType := listVal.ElementType(context.Background())
	if elemType != types.StringType {
		t.Errorf("expected element type to be StringType, got %v", elemType)
	}
}
