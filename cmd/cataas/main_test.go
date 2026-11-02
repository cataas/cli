package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cat/cute,orange/says/hi":
			w.Write([]byte("image-bytes"))
		case "/api/tags":
			w.Write([]byte(`["cute","orange","Cute Cat"]`))
		case "/api/cats":
			w.Write([]byte(`[{"_id":"abc","tags":["cute","orange"]}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func exec(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(context.Background(), args, &out, &errOut)
	return out.String(), err
}

func TestShowPipedWritesRawImage(t *testing.T) {
	srv := newServer(t)
	out, err := exec(t, "--base-url", srv.URL, "-t", "cute", "--says", "hi", "orange")
	if err != nil || out != "image-bytes" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestShowURL(t *testing.T) {
	out, err := exec(t, "--base-url", "https://x.test", "--url", "--gif", "--filter", "mono")
	if err != nil || out != "https://x.test/cat/gif?filter=mono\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestShowErrors(t *testing.T) {
	srv := newServer(t)
	if _, err := exec(t, "--base-url", srv.URL, "unknown"); err == nil || !strings.Contains(err.Error(), "no cat found") {
		t.Errorf("err = %v", err)
	}
	if _, err := exec(t, "--mode", "nope"); err == nil {
		t.Error("expected an invalid mode error")
	}
}

func TestTags(t *testing.T) {
	srv := newServer(t)
	out, err := exec(t, "tags", "--base-url", srv.URL, "cute")
	if err != nil || out != "cute\nCute Cat\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestList(t *testing.T) {
	srv := newServer(t)
	out, err := exec(t, "list", "--base-url", srv.URL)
	if err != nil || out != "abc\tcute, orange\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestVersion(t *testing.T) {
	if out, _ := exec(t, "version"); out != "cataas dev\n" {
		t.Errorf("got %q", out)
	}
}

func TestMixedFlagsAndTags(t *testing.T) {
	out, err := exec(t, "--base-url", "https://x.test", "cute", "--says", "Bonjour !", "orange", "--url")
	if err != nil || out != "https://x.test/cat/cute,orange/says/Bonjour%20%21\n" {
		t.Fatalf("got %q, %v", out, err)
	}
	out, err = exec(t, "--base-url", "https://x.test", "--url", "--", "-weird")
	if err != nil || out != "https://x.test/cat/-weird\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}

func TestDisplayBox(t *testing.T) {
	cases := []struct{ w, h, wantCols, wantRows int }{
		{0, 0, 100, 48},  // default: terminal size, max 100 columns
		{40, 0, 40, 0},   // width only: height follows the aspect ratio
		{0, 10, 200, 10}, // height only: width limited by the terminal
		{40, 10, 40, 10}, // both: fit in the box
	}
	for _, c := range cases {
		cols, rows := displayBox(c.w, c.h, 200, 50)
		if cols != c.wantCols || rows != c.wantRows {
			t.Errorf("displayBox(%d, %d) = %d, %d; want %d, %d", c.w, c.h, cols, rows, c.wantCols, c.wantRows)
		}
	}
}

func TestImageSizeFlags(t *testing.T) {
	out, err := exec(t, "--base-url", "https://x.test", "--url", "--image-width", "300", "--width", "20", "--height", "5")
	if err != nil || out != "https://x.test/cat?width=300\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}
