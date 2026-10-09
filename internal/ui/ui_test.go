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
