package filesystem

import "testing"

func TestIsMediaExt(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Movie.mkv", true},
		{"Movie.MKV", true},
		{"Movie.mp4", true},
		{"Episode.m2ts", true},
		{"notes.nfo", false},
		{"subs.srt", false},
		{"poster.jpg", false},
		{"readme.txt", false},
		{"download.partial", false},
		{"noextension", false},
	}
	for _, tt := range tests {
		if got := isMediaExt(tt.name); got != tt.want {
			t.Errorf("isMediaExt(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
