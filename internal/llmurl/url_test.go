package llmurl

import "testing"

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{
			name: "provider root",
			base: "https://api.deepseek.com",
			path: "/v1/chat/completions",
			want: "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name: "versioned root",
			base: "https://api.deepseek.com/v1/",
			path: "/v1/chat/completions",
			want: "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name: "anthropic endpoint",
			base: "https://api.anthropic.com",
			path: "/v1/messages",
			want: "https://api.anthropic.com/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Endpoint(tt.base, tt.path); got != tt.want {
				t.Fatalf("Endpoint(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
			}
		})
	}
}
