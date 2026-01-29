package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLanguageFromExtension(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		wantLang  Language
		wantFound bool
	}{
		// Java
		{"java file", ".java", "java", true},
		{"java file no dot", "java", "java", true},

		// C#
		{"csharp file", ".cs", "csharp", true},
		{"cshtml file", ".cshtml", "csharp", true},

		// Node/JavaScript/TypeScript
		{"javascript file", ".js", "node", true},
		{"jsx file", ".jsx", "node", true},
		{"typescript file", ".ts", "node", true},
		{"tsx file", ".tsx", "node", true},

		// Python
		{"python file", ".py", "python", true},
		{"python3 file", ".py3", "python", true},

		// Go
		{"go file", ".go", "go", true},

		// Rust
		{"rust file", ".rs", "rust", true},

		// C/C++
		{"c file", ".c", "c", true},
		{"h file", ".h", "c", true},
		{"cpp file", ".cpp", "cpp", true},
		{"hpp file", ".hpp", "cpp", true},

		// Unknown extensions
		{"txt file", ".txt", "", false},
		{"md file", ".md", "", false},
		{"json file", ".json", "", false},
		{"empty extension", "", "", false},
		{"yaml file", ".yaml", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLang, gotFound := GetLanguageFromExtension(tt.extension)
			assert.Equal(t, tt.wantLang, gotLang, "language mismatch for %s", tt.extension)
			assert.Equal(t, tt.wantFound, gotFound, "found mismatch for %s", tt.extension)
		})
	}
}

