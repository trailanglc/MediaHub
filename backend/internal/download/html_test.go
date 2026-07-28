package download

import "testing"

func TestCleanURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{
			`https://cdn.example.com/a/video.mp4","6843462","/api/history`,
			`https://cdn.example.com/a/video.mp4`,
		},
		{
			`https://cdn.example.com/a/video.mp4&quot;,&quot;type&quot;:&quot;video/mp4`,
			`https://cdn.example.com/a/video.mp4`,
		},
		{
			`https://cdn.example.com/a/stream.m3u8?token=abc`,
			`https://cdn.example.com/a/stream.m3u8?token=abc`,
		},
		{
			`https://cdn.example.com/a/stream.mp4?x=1,y=2`,
			`https://cdn.example.com/a/stream.mp4?x=1`,
		},
		{
			`https://video.example.com/P0/trailer/2160.mp4`,
			`https://video.example.com/P0/trailer/2160.mp4`,
		},
	}
	for _, tc := range tests {
		got := cleanURL(tc.in)
		if got != tc.want {
			t.Errorf("cleanURL(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestIsNoiseMediaURL(t *testing.T) {
	noise := []string{
		"https://thumb-ah.flixcdn.com/xc/ya/yaXEBw/heat-preview/fh_heatmap_preview_v6_a-big-720.mp4",
		"https://thumb-ah.flixcdn.com/xc/uj/ujNmUS/preview/big-720.mp4",
		"https://cdn.example.com/sprites/sheet.mp4",
		"https://thumbs.example.com/v/clip.mp4",
	}
	for _, u := range noise {
		if !isNoiseMediaURL(u) {
			t.Errorf("expected noise: %s", u)
		}
	}

	keep := []string{
		"https://video-nss.flixcdn.com/6GlycMBus7OfJ0XuoUZAxw==,1785268800/P0/P0eOPe/trailer/2160.mp4",
		"https://cdn.example.com/videos/full/1080.mp4",
		"https://stream.example.com/hls/master.m3u8",
	}
	for _, u := range keep {
		if isNoiseMediaURL(u) {
			t.Errorf("expected keep: %s", u)
		}
	}
}

func TestIsPlausibleMediaURL(t *testing.T) {
	if isPlausibleMediaURL(`https://cdn.example.com/a.mp4&quot;,junk`) {
		t.Fatal("rejected entity bleed")
	}
	if !isPlausibleMediaURL(`https://cdn.example.com/a/trailer/2160.mp4`) {
		t.Fatal("expected plausible trailer url")
	}
}

func TestMP4RegexStopsAtExtension(t *testing.T) {
	html := `{"url":"https://thumb-ah.flixcdn.com/xc/ya/heat-preview/clip-720.mp4","id":"6843462","path":"/api/history"}`
	matches := reMP4.FindAllString(html, -1)
	if len(matches) != 1 {
		t.Fatalf("matches=%v", matches)
	}
	got := cleanURL(matches[0])
	want := "https://thumb-ah.flixcdn.com/xc/ya/heat-preview/clip-720.mp4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !isNoiseMediaURL(got) {
		t.Fatal("heat-preview should be noise")
	}
}
