package ui

import "testing"

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		selected  int
		height    int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "empty list",
			length:    0,
			selected:  0,
			height:    5,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "non-positive height",
			length:    10,
			selected:  3,
			height:    0,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "selection within initial viewport",
			length:    10,
			selected:  2,
			height:    5,
			wantStart: 0,
			wantEnd:   5,
		},
		{
			name:      "selection scrolls down at bottom edge",
			length:    10,
			selected:  5,
			height:    5,
			wantStart: 1,
			wantEnd:   6,
		},
		{
			name:      "selection near list end",
			length:    10,
			selected:  9,
			height:    5,
			wantStart: 5,
			wantEnd:   10,
		},
		{
			name:      "viewport taller than list",
			length:    3,
			selected:  2,
			height:    8,
			wantStart: 0,
			wantEnd:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := visibleRange(tt.length, tt.selected, tt.height)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Fatalf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)", tt.length, tt.selected, tt.height, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestCommitHash(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   string
		wantOK bool
	}{
		{
			name:   "short git hash",
			line:   "a83f21c Add project filtering",
			want:   "a83f21c",
			wantOK: true,
		},
		{
			name:   "full git hash",
			line:   "0123456789abcdef0123456789abcdef01234567 Commit message",
			want:   "0123456789abcdef0123456789abcdef01234567",
			wantOK: true,
		},
		{
			name:   "non-hash prefix",
			line:   "HEAD -> main",
			want:   "",
			wantOK: false,
		},
		{
			name:   "too short",
			line:   "abc123 Fix",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := commitHash(tt.line)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("commitHash(%q) = (%q, %t), want (%q, %t)", tt.line, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestColorizeCommitLine(t *testing.T) {
	got := colorizeCommitLine("a83f21c Add project filtering", "a83f21c")
	want := "\x1b[1;35ma83f21c\x1b[0m Add project filtering"
	if got != want {
		t.Fatalf("colorizeCommitLine() = %q, want %q", got, want)
	}
}
