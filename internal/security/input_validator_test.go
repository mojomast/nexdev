package security

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewInputValidator(t *testing.T) {
	validator := NewInputValidator()
	if validator == nil {
		t.Fatal("NewInputValidator returned nil")
	}
	if validator.projectNamePattern == nil {
		t.Error("projectNamePattern not initialized")
	}
}

func TestValidateProjectName(t *testing.T) {
	validator := NewInputValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid names
		{
			name:      "simple alphanumeric",
			input:     "myproject",
			wantError: false,
		},
		{
			name:      "with hyphens",
			input:     "my-project",
			wantError: false,
		},
		{
			name:      "with underscores",
			input:     "my_project",
			wantError: false,
		},
		{
			name:      "mixed case",
			input:     "MyProject",
			wantError: false,
		},
		{
			name:      "with numbers",
			input:     "project123",
			wantError: false,
		},
		{
			name:      "complex valid name",
			input:     "my-awesome_project-2024",
			wantError: false,
		},
		{
			name:      "single character",
			input:     "a",
			wantError: false,
		},
		{
			name:      "single digit",
			input:     "1",
			wantError: false,
		},

		// Invalid names
		{
			name:      "empty string",
			input:     "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "starts with hyphen",
			input:     "-project",
			wantError: true,
			errorMsg:  "must start and end with an alphanumeric character",
		},
		{
			name:      "ends with hyphen",
			input:     "project-",
			wantError: true,
			errorMsg:  "must start and end with an alphanumeric character",
		},
		{
			name:      "starts with underscore",
			input:     "_project",
			wantError: true,
			errorMsg:  "must start and end with an alphanumeric character",
		},
		{
			name:      "ends with underscore",
			input:     "project_",
			wantError: true,
			errorMsg:  "must start and end with an alphanumeric character",
		},
		{
			name:      "contains spaces",
			input:     "my project",
			wantError: true,
			errorMsg:  "only alphanumeric characters, hyphens, and underscores are allowed",
		},
		{
			name:      "contains special characters",
			input:     "my-project!",
			wantError: true,
			errorMsg:  "only alphanumeric characters, hyphens, and underscores are allowed",
		},
		{
			name:      "contains dots",
			input:     "my.project",
			wantError: true,
			errorMsg:  "only alphanumeric characters, hyphens, and underscores are allowed",
		},
		{
			name:      "contains slashes",
			input:     "my/project",
			wantError: true,
			errorMsg:  "only alphanumeric characters, hyphens, and underscores are allowed",
		},
		{
			name:      "consecutive hyphens",
			input:     "my--project",
			wantError: true,
			errorMsg:  "consecutive special characters are not allowed",
		},
		{
			name:      "consecutive underscores",
			input:     "my__project",
			wantError: true,
			errorMsg:  "consecutive special characters are not allowed",
		},
		{
			name:      "mixed consecutive special chars",
			input:     "my-_project",
			wantError: true,
			errorMsg:  "consecutive special characters are not allowed",
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 256),
			wantError: true,
			errorMsg:  "exceeds maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateProjectName(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateProjectName(%q) expected error, got nil", tt.input)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateProjectName(%q) error = %v, want error containing %q", tt.input, err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateProjectName(%q) unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	validator := NewInputValidator()

	type TestSchema struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name      string
		data      []byte
		schema    interface{}
		wantError bool
		errorMsg  string
	}{
		// Valid JSON
		{
			name:      "valid JSON without schema",
			data:      []byte(`{"name": "test", "value": 123}`),
			schema:    nil,
			wantError: false,
		},
		{
			name:      "valid JSON with matching schema",
			data:      []byte(`{"name": "test", "value": 123}`),
			schema:    &TestSchema{},
			wantError: false,
		},
		{
			name:      "valid JSON array",
			data:      []byte(`[1, 2, 3]`),
			schema:    nil,
			wantError: false,
		},
		{
			name:      "valid JSON string",
			data:      []byte(`"hello"`),
			schema:    nil,
			wantError: false,
		},
		{
			name:      "valid JSON number",
			data:      []byte(`123`),
			schema:    nil,
			wantError: false,
		},
		{
			name:      "valid JSON boolean",
			data:      []byte(`true`),
			schema:    nil,
			wantError: false,
		},
		{
			name:      "valid JSON null",
			data:      []byte(`null`),
			schema:    nil,
			wantError: false,
		},

		// Invalid JSON
		{
			name:      "empty data",
			data:      []byte{},
			schema:    nil,
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "invalid JSON syntax",
			data:      []byte(`{name: "test"}`),
			schema:    nil,
			wantError: true,
			errorMsg:  "invalid JSON",
		},
		{
			name:      "unclosed brace",
			data:      []byte(`{"name": "test"`),
			schema:    nil,
			wantError: true,
			errorMsg:  "invalid JSON",
		},
		{
			name:      "trailing comma",
			data:      []byte(`{"name": "test",}`),
			schema:    nil,
			wantError: true,
			errorMsg:  "invalid JSON",
		},
		{
			name:      "not JSON",
			data:      []byte(`this is not json`),
			schema:    nil,
			wantError: true,
			errorMsg:  "invalid JSON",
		},

		// Schema validation
		{
			name:      "JSON doesn't match schema type",
			data:      []byte(`{"name": "test", "value": "not a number"}`),
			schema:    &TestSchema{},
			wantError: true,
			errorMsg:  "does not match expected schema",
		},
		{
			name:      "JSON missing required field",
			data:      []byte(`{"name": "test"}`),
			schema:    &TestSchema{},
			wantError: false, // Go's json.Unmarshal allows missing fields (zero values)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateJSON(tt.data, tt.schema)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateJSON() expected error, got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateJSON() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateJSON() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateJSON_SchemaPopulation(t *testing.T) {
	validator := NewInputValidator()

	type TestSchema struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := []byte(`{"name": "test", "value": 123}`)
	schema := &TestSchema{}

	err := validator.ValidateJSON(data, schema)
	if err != nil {
		t.Fatalf("ValidateJSON() unexpected error: %v", err)
	}

	// Verify the schema was populated
	if schema.Name != "test" {
		t.Errorf("schema.Name = %q, want %q", schema.Name, "test")
	}
	if schema.Value != 123 {
		t.Errorf("schema.Value = %d, want %d", schema.Value, 123)
	}
}

func TestValidateFileContent(t *testing.T) {
	validator := NewInputValidator()

	tests := []struct {
		name      string
		content   string
		maxSize   int
		wantError bool
		errorMsg  string
	}{
		// Valid content
		{
			name:      "small content no limit",
			content:   "Hello, World!",
			maxSize:   0,
			wantError: false,
		},
		{
			name:      "content within limit",
			content:   "Hello, World!",
			maxSize:   100,
			wantError: false,
		},
		{
			name:      "content at exact limit",
			content:   "Hello!",
			maxSize:   6,
			wantError: false,
		},
		{
			name:      "empty content",
			content:   "",
			maxSize:   100,
			wantError: false,
		},
		{
			name:      "multiline content",
			content:   "Line 1\nLine 2\nLine 3",
			maxSize:   100,
			wantError: false,
		},
		{
			name:      "content with unicode",
			content:   "Hello 世界 🌍",
			maxSize:   100,
			wantError: false,
		},

		// Invalid content
		{
			name:      "content exceeds limit",
			content:   "Hello, World!",
			maxSize:   5,
			wantError: true,
			errorMsg:  "exceeds maximum size",
		},
		{
			name:      "large content exceeds limit",
			content:   strings.Repeat("a", 1000),
			maxSize:   500,
			wantError: true,
			errorMsg:  "exceeds maximum size",
		},
		{
			name:      "invalid UTF-8",
			content:   "Hello\xff\xfeWorld",
			maxSize:   100,
			wantError: true,
			errorMsg:  "invalid UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFileContent(tt.content, tt.maxSize)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateFileContent() expected error, got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateFileContent() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFileContent() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	validator := NewInputValidator()

	tests := []struct {
		name      string
		path      string
		wantError bool
		errorMsg  string
	}{
		// Valid paths
		{
			name:      "simple path",
			path:      "file.txt",
			wantError: false,
		},
		{
			name:      "path with directory",
			path:      "dir/file.txt",
			wantError: false,
		},
		{
			name:      "absolute path",
			path:      "/home/user/file.txt",
			wantError: false,
		},
		{
			name:      "windows path",
			path:      "C:\\Users\\file.txt",
			wantError: false,
		},
		{
			name:      "path with spaces",
			path:      "my file.txt",
			wantError: false,
		},
		{
			name:      "path with unicode",
			path:      "文件.txt",
			wantError: false,
		},

		// Invalid paths
		{
			name:      "empty path",
			path:      "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "path with null byte",
			path:      "file\x00.txt",
			wantError: true,
			errorMsg:  "null bytes",
		},
		{
			name:      "path with control characters",
			path:      "file\x01.txt",
			wantError: true,
			errorMsg:  "control characters",
		},
		{
			name:      "path with invalid UTF-8",
			path:      "file\xff\xfe.txt",
			wantError: true,
			errorMsg:  "invalid UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFilePath(tt.path)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateFilePath(%q) expected error, got nil", tt.path)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateFilePath(%q) error = %v, want error containing %q", tt.path, err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFilePath(%q) unexpected error: %v", tt.path, err)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateProjectName(b *testing.B) {
	validator := NewInputValidator()
	name := "my-awesome-project-2024"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateProjectName(name)
	}
}

func BenchmarkValidateJSON(b *testing.B) {
	validator := NewInputValidator()
	data := []byte(`{"name": "test", "value": 123}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateJSON(data, nil)
	}
}

func BenchmarkValidateFileContent(b *testing.B) {
	validator := NewInputValidator()
	content := strings.Repeat("Hello, World!\n", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateFileContent(content, 10000)
	}
}

// Test edge cases for JSON validation with complex schemas
func TestValidateJSON_ComplexSchema(t *testing.T) {
	validator := NewInputValidator()

	type NestedSchema struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type ComplexSchema struct {
		Title   string         `json:"title"`
		Items   []NestedSchema `json:"items"`
		Enabled bool           `json:"enabled"`
	}

	tests := []struct {
		name      string
		data      string
		wantError bool
	}{
		{
			name: "valid complex JSON",
			data: `{
				"title": "Test",
				"items": [
					{"id": 1, "name": "Item 1"},
					{"id": 2, "name": "Item 2"}
				],
				"enabled": true
			}`,
			wantError: false,
		},
		{
			name: "invalid nested type",
			data: `{
				"title": "Test",
				"items": [
					{"id": "not a number", "name": "Item 1"}
				],
				"enabled": true
			}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &ComplexSchema{}
			err := validator.ValidateJSON([]byte(tt.data), schema)
			if tt.wantError && err == nil {
				t.Error("ValidateJSON() expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("ValidateJSON() unexpected error: %v", err)
			}
		})
	}
}

// Test that ValidateJSON properly unmarshals into the schema
func TestValidateJSON_Unmarshal(t *testing.T) {
	validator := NewInputValidator()

	type Schema struct {
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Count int      `json:"count"`
	}

	data := []byte(`{"name": "test", "tags": ["a", "b", "c"], "count": 42}`)
	schema := &Schema{}

	err := validator.ValidateJSON(data, schema)
	if err != nil {
		t.Fatalf("ValidateJSON() unexpected error: %v", err)
	}

	// Verify all fields were unmarshaled correctly
	if schema.Name != "test" {
		t.Errorf("Name = %q, want %q", schema.Name, "test")
	}
	if len(schema.Tags) != 3 {
		t.Errorf("len(Tags) = %d, want 3", len(schema.Tags))
	}
	if schema.Count != 42 {
		t.Errorf("Count = %d, want 42", schema.Count)
	}
}

// Test ValidateJSON with raw json.RawMessage
func TestValidateJSON_RawMessage(t *testing.T) {
	validator := NewInputValidator()

	data := []byte(`{"key": "value"}`)
	var raw json.RawMessage

	err := validator.ValidateJSON(data, &raw)
	if err != nil {
		t.Fatalf("ValidateJSON() unexpected error: %v", err)
	}

	if string(raw) != string(data) {
		t.Errorf("RawMessage = %s, want %s", raw, data)
	}
}
