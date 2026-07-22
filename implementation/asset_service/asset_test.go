package assetservice

import "testing"

func TestWebRenditionName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"video.mp4", "video-web.mp4"},
		{"photo.jpeg", "photo-web.jpeg"},
		{"song.mp3", "song-web.mp3"},
		{"noext", "noext-web"},
		{"clip.tar.gz", "clip.tar-web.gz"},
		{"archive/a.mp3", "a-web.mp3"},
		{"UPPER.MP4", "UPPER-web.MP4"},
	}
	for _, c := range cases {
		if got := WebRenditionName(c.in); got != c.want {
			t.Errorf("WebRenditionName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAssetHasRawAndRenditionNames(t *testing.T) {
	a := Asset{
		ID:           "asset-1",
		SHA256:       sha256hex([]byte("raw")),
		OriginalName: "video.mp4",
		Renditions: map[string]Rendition{
			WebRenditionName("video.mp4"): {Name: WebRenditionName("video.mp4"), ContentID: sha256hex([]byte("web"))},
		},
	}
	if !a.HasRaw() {
		t.Fatalf("HasRaw = false, want true")
	}
	names := a.RenditionNames()
	if len(names) != 1 || names[0] != "video-web.mp4" {
		t.Fatalf("RenditionNames = %v, want [video-web.mp4]", names)
	}

	empty := Asset{ID: "asset-2"}
	if empty.HasRaw() {
		t.Fatalf("empty asset HasRaw = true, want false")
	}
}
