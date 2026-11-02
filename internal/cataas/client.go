// Package cataas is a small client for the Cat as a Service API (https://cataas.com).
package cataas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the public Cat as a Service endpoint.
const DefaultBaseURL = "https://cataas.com"

// Filters documented by the API ("custom" enables RGB / HSL adjustments).
var Filters = []string{"mono", "negate", "custom"}

// Types (sizes) supported by the API.
var Types = []string{"xsmall", "small", "medium", "square"}

// Fits supported by the API when resizing.
var Fits = []string{"cover", "contain", "fill", "inside", "outside"}

// Client talks to a Cat as a Service instance.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New returns a client for baseURL (DefaultBaseURL when empty).
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "cataas-cli",
	}
}

// CatRequest describes which cat to fetch and how to transform it.
type CatRequest struct {
	ID   string   // specific cat id (takes precedence over Tags and GIF)
	Tags []string // only cats having all these tags
	GIF  bool     // random animated cat
	Says string   // text written on the picture

	Font           string
	FontSize       int
	FontColor      string
	FontBackground string

	Type     string // xsmall, small, medium, square
	Width    int
	Height   int
	Fit      string // cover, contain, fill, inside, outside
	Position string // center, top, right top, ...
	Blur     int

	Filter string // mono, negate, custom
	// Adjustments used with Filter == "custom". Zero values are not sent.
	R, G, B    int
	Brightness float64
	Saturation float64
	Hue        int
	Lightness  int
}

// CatURL builds the image URL for req.
func (c *Client) CatURL(req CatRequest) string {
	segments := []string{"cat"}
	tags := req.Tags
	if req.GIF {
		// Animated cats are simply the ones tagged "gif".
		tags = append([]string{"gif"}, tags...)
	}
	switch {
	case req.ID != "":
		segments = append(segments, url.PathEscape(req.ID))
	case len(tags) > 0:
		escaped := make([]string, len(tags))
		for i, t := range tags {
			escaped[i] = url.PathEscape(t)
		}
		segments = append(segments, strings.Join(escaped, ","))
	}
	if req.Says != "" {
		segments = append(segments, "says", url.PathEscape(req.Says))
	}

	q := url.Values{}
	setInt := func(k string, v int) {
		if v > 0 {
			q.Set(k, strconv.Itoa(v))
		}
	}
	setStr := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	setFloat := func(k string, v float64) {
		if v != 0 {
			q.Set(k, strconv.FormatFloat(v, 'f', -1, 64))
		}
	}
	if req.Says != "" {
		setStr("font", req.Font)
		setInt("fontSize", req.FontSize)
		setStr("fontColor", req.FontColor)
		setStr("fontBackground", req.FontBackground)
	}
	setStr("type", req.Type)
	setInt("width", req.Width)
	setInt("height", req.Height)
	setStr("fit", req.Fit)
	setStr("position", req.Position)
	setInt("blur", req.Blur)
	setStr("filter", req.Filter)
	if req.Filter == "custom" {
		for k, v := range map[string]int{"r": req.R, "g": req.G, "b": req.B, "hue": req.Hue, "lightness": req.Lightness} {
			if v != 0 {
				q.Set(k, strconv.Itoa(v))
			}
		}
		setFloat("brightness", req.Brightness)
		setFloat("saturation", req.Saturation)
	}

	u := c.BaseURL + "/" + strings.Join(segments, "/")
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

// Image is a downloaded cat picture.
type Image struct {
	Data        []byte
	ContentType string
	URL         string
}

// Cat downloads the picture described by req.
func (c *Client) Cat(ctx context.Context, req CatRequest) (*Image, error) {
	u := c.CatURL(req)
	resp, err := c.get(ctx, u, "image/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	return &Image{Data: data, ContentType: resp.Header.Get("Content-Type"), URL: u}, nil
}

// CatInfo is a cat's metadata as returned by /api/cats.
type CatInfo struct {
	ID        string   `json:"_id"`
	Tags      []string `json:"tags"`
	Mimetype  string   `json:"mimetype"`
	Size      int64    `json:"size,omitempty"`
	CreatedAt string   `json:"createdAt,omitempty"`
}

// ListRequest filters /api/cats.
type ListRequest struct {
	Tags  []string
	Skip  int
	Limit int
}

// Cats lists cats matching req.
func (c *Client) Cats(ctx context.Context, req ListRequest) ([]CatInfo, error) {
	q := url.Values{}
	if len(req.Tags) > 0 {
		q.Set("tags", strings.Join(req.Tags, ","))
	}
	if req.Skip > 0 {
		q.Set("skip", strconv.Itoa(req.Skip))
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	u := c.BaseURL + "/api/cats"
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	var cats []CatInfo
	return cats, c.getJSON(ctx, u, &cats)
}

// Tags returns every tag known by the API.
func (c *Client) Tags(ctx context.Context) ([]string, error) {
	var tags []string
	if err := c.getJSON(ctx, c.BaseURL+"/api/tags", &tags); err != nil {
		return nil, err
	}
	// Some entries can be blank; drop them.
	out := tags[:0]
	for _, t := range tags {
		if strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, u string, v any) error {
	resp, err := c.get(ctx, u, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding %s: %w", u, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, u, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("no cat found (%s)", u)
		}
		if msg != "" {
			return nil, fmt.Errorf("GET %s: %s: %s", u, resp.Status, msg)
		}
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return resp, nil
}
