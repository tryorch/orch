package adaptersupport

import "testing"

func TestStringifyOutputValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{name: "string", value: "hello", want: "hello"},
		{name: "bool", value: true, want: "true"},
		{name: "number", value: 42.5, want: "42.5"},
		{name: "nil", value: nil, want: ""},
		{name: "list", value: []interface{}{"a", "b"}, want: `["a","b"]`},
		{name: "map", value: map[string]interface{}{"key": "value"}, want: `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StringifyOutputValue(tt.value)
			if err != nil {
				t.Fatalf("StringifyOutputValue returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("StringifyOutputValue = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringifyOutputValueRejectsUnsupportedType(t *testing.T) {
	if _, err := StringifyOutputValue(1); err == nil {
		t.Fatal("expected unsupported int value to fail")
	}
}
