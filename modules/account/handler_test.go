package account

import "testing"

func TestSanitizeRedirectTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		next     string
		fallback string
		want     string
	}{
		{name: "empty uses fallback", next: "", fallback: "/account/profile", want: "/account/profile"},
		{name: "relative path allowed", next: "/account/settings?tab=profile", fallback: "/account/profile", want: "/account/settings?tab=profile"},
		{name: "scheme absolute blocked", next: "https://evil.example/phish", fallback: "/account/profile", want: "/account/profile"},
		{name: "scheme relative blocked", next: "//evil.example/phish", fallback: "/account/profile", want: "/account/profile"},
		{name: "path traversal style still local", next: "/../admin", fallback: "/account/profile", want: "/../admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeRedirectTarget(tt.next, tt.fallback); got != tt.want {
				t.Errorf("sanitizeRedirectTarget(%q, %q) = %q, want %q", tt.next, tt.fallback, got, tt.want)
			}
		})
	}
}
