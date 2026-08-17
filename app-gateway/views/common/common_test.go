package common

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestButton(t *testing.T) {
	// 1. Define table-driven test cases
	tests := []struct {
		name       string
		buttonType string
		text       string
		classes    string
		wantType   string
		wantText   string
		wantClass  string
	}{
		{
			name:       "renders with default classes when empty",
			buttonType: "button",
			text:       "Click Me",
			classes:    "",
			wantType:   `type="button"`,
			wantText:   "Click Me",
			wantClass:  `class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded focus:outline-none focus:shadow-outline mb-1"`,
		},
		{
			name:       "overrides default classes when provided",
			buttonType: "submit",
			text:       "Submit Form",
			classes:    "custom-btn secondary",
			wantType:   `type="submit"`,
			wantText:   "Submit Form",
			wantClass:  `class="custom-btn secondary"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 2. Initialize a buffer to capture the HTML output
			var buf bytes.Buffer

			// 3. Instantiate the component and call Render
			component := Button(tt.buttonType, tt.text, tt.classes)
			err := component.Render(context.Background(), &buf)
			if err != nil {
				t.Fatalf("failed to render component: %v", err)
			}

			// 4. Extract the raw string output
			gotHtml := buf.String()

			// 5. Make assertions on the output
			if !strings.Contains(gotHtml, tt.wantType) {
				t.Errorf("expected HTML to contain %q, got:\n%s", tt.wantType, gotHtml)
			}
			if !strings.Contains(gotHtml, tt.wantText) {
				t.Errorf("expected HTML to contain text %q, got:\n%s", tt.wantText, gotHtml)
			}
			if !strings.Contains(gotHtml, tt.wantClass) {
				t.Errorf("expected HTML to contain %q, got:\n%s", tt.wantClass, gotHtml)
			}
		})
	}
}
