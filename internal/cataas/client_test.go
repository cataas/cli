package cataas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatURL(t *testing.T) {
	c := New("https://example.test/")
	tests := []struct {
		name string
		req  CatRequest
		want string
	}{
		{"random", CatRequest{}, "https://example.test/cat"},
		{"tags", CatRequest{Tags: []string{"cute", "orange"}}, "https://example.test/cat/cute,orange"},
		{"gif", CatRequest{GIF: true}, "https://example.test/cat/gif"},
		{"gif and tag", CatRequest{GIF: true, Tags: []string{"cute"}}, "https://example.test/cat/gif,cute"},
		{"id wins", CatRequest{ID: "abc", Tags: []string{"cute"}, GIF: true}, "https://example.test/cat/abc"},
		{"says", CatRequest{Says: "Hello world/!"}, "https://example.test/cat/says/Hello%20world%2F%21"},
		{"tag says", CatRequest{Tags: []string{"cute"}, Says: "hi", FontSize: 50, FontColor: "red"},
			"https://example.test/cat/cute/says/hi?fontColor=red&fontSize=50"},
		{"font options need text", CatRequest{FontSize: 50}, "https://example.test/cat"},
		{"resize", CatRequest{Type: "square", Width: 100, Height: 50, Fit: "contain", Position: "left top"},
			"https://example.test/cat?fit=contain&height=50&position=left+top&type=square&width=100"},
		{"filter", CatRequest{Filter: "mono", Blur: 3, R: 10}, "https://example.test/cat?blur=3&filter=mono"},
		{"custom filter", CatRequest{Filter: "custom", R: 7, G: 12, B: 50, Brightness: 1.5, Hue: 90},
			"https://example.test/cat?b=50&brightness=1.5&filter=custom&g=12&hue=90&r=7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.CatURL(tt.req); got != tt.want {
				t.Errorf("CatURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultBaseURL(t *testing.T) {
	if got := New("").CatURL(CatRequest{}); got != "https://cataas.com/cat" {
		t.Errorf("got %q", got)
	}
}

func TestAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cat/cute":
			if r.Header.Get("Accept") != "image/*" {
				t.Errorf("Accept = %q", r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("png-bytes"))
		case "/api/tags":
			w.Write([]byte(`["cute","", "orange"]`))
		case "/api/cats":
			if got := r.URL.RawQuery; got != "limit=2&skip=1&tags=cute" {
				t.Errorf("query = %q", got)
			}
			w.Write([]byte(`[{"_id":"abc","tags":["cute"],"mimetype":"image/jpeg"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx := context.Background()

	img, err := c.Cat(ctx, CatRequest{Tags: []string{"cute"}})
	if err != nil || string(img.Data) != "png-bytes" || img.ContentType != "image/png" {
		t.Fatalf("Cat() = %+v, %v", img, err)
	}

	if _, err := c.Cat(ctx, CatRequest{Tags: []string{"nope"}}); err == nil {
		t.Error("expected an error for an unknown tag")
	}

	tags, err := c.Tags(ctx)
	if err != nil || len(tags) != 2 || tags[1] != "orange" {
		t.Fatalf("Tags() = %v, %v", tags, err)
	}

	cats, err := c.Cats(ctx, ListRequest{Tags: []string{"cute"}, Limit: 2, Skip: 1})
	if err != nil || len(cats) != 1 || cats[0].ID != "abc" {
		t.Fatalf("Cats() = %+v, %v", cats, err)
	}
}
