// Command cataas displays cats from https://cataas.com in your terminal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/term"

	"github.com/cataas/cli/internal/cataas"
	"github.com/cataas/cli/internal/render"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `cataas - cats in your terminal, powered by https://cataas.com

Usage:
  cataas [options] [tag...]      show a random cat (optionally having all the tags)
  cataas tags [filter]           list available tags
  cataas list [options]          list cats (id and tags)
  cataas version                 print the version

Examples:
  cataas
  cataas cute orange
  cataas --gif
  cataas --says "Hello world" --font-color orange --font-size 40
  cataas --filter mono --type square
  cataas --id 595f280c557291a9750ebf80
  cataas --output cat.jpg
  cataas --url --tag cute
  cataas list --tag cute --limit 5

Run "cataas <command> --help" for the options of a command.
Set CATAAS_URL to use another Cat as a Service instance.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "cataas:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "tags":
			return runTags(ctx, args[1:], stdout, stderr)
		case "list", "ls":
			return runList(ctx, args[1:], stdout, stderr)
		case "version", "--version", "-v":
			fmt.Fprintln(stdout, "cataas", version)
			return nil
		case "help", "-h", "--help":
			fmt.Fprint(stdout, usage)
			return nil
		case "show":
			args = args[1:]
		}
	}
	return runShow(ctx, args, stdout, stderr)
}

// listFlag accumulates repeated and/or comma separated values.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

func newFlagSet(name string, stderr io.Writer, help string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, help, "\nOptions:\n")
		fs.PrintDefaults()
	}
	baseURL := fs.String("base-url", envOr("CATAAS_URL", cataas.DefaultBaseURL), "Cat as a Service base URL (env CATAAS_URL)")
	return fs, baseURL
}

// parse parses args allowing flags and positional arguments to be mixed
// ("cataas cute --gif orange"). It returns the positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		if len(args) > len(rest) && args[len(args)-len(rest)-1] == "--" {
			// Everything after "--" is positional.
			return append(positional, rest...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func runShow(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs, baseURL := newFlagSet("cataas", stderr, usage)
	var req cataas.CatRequest
	var tags listFlag
	fs.Var(&tags, "tag", "only cats having this `tag` (repeatable, or comma separated)")
	fs.Var(&tags, "t", "shorthand for --tag")
	fs.StringVar(&req.ID, "id", "", "show the cat with this `id`")
	fs.BoolVar(&req.GIF, "gif", false, "show an animated cat")
	fs.StringVar(&req.Says, "says", "", "write `text` on the picture")
	fs.StringVar(&req.Says, "s", "", "shorthand for --says")
	fs.StringVar(&req.Font, "font", "", "text `font` (Impact, Arial, Comic Sans MS, ...)")
	fs.IntVar(&req.FontSize, "font-size", 0, "text `size` in pixels")
	fs.StringVar(&req.FontColor, "font-color", "", "text `color` (name or #hex)")
	fs.StringVar(&req.FontBackground, "font-background", "", "text background `color`")
	fs.StringVar(&req.Type, "type", "", "picture `type`: "+strings.Join(cataas.Types, ", "))
	fs.IntVar(&req.Width, "width", 0, "resize the picture to this `width` in pixels")
	fs.IntVar(&req.Height, "height", 0, "resize the picture to this `height` in pixels")
	fs.StringVar(&req.Fit, "fit", "", "resize `mode`: "+strings.Join(cataas.Fits, ", "))
	fs.StringVar(&req.Position, "position", "", "resize `anchor` (center, top, right top, ...)")
	fs.IntVar(&req.Blur, "blur", 0, "blur `radius`")
	fs.StringVar(&req.Filter, "filter", "", "image `filter`: "+strings.Join(cataas.Filters, ", "))
	fs.IntVar(&req.R, "r", 0, "red adjustment (with --filter custom)")
	fs.IntVar(&req.G, "g", 0, "green adjustment (with --filter custom)")
	fs.IntVar(&req.B, "b", 0, "blue adjustment (with --filter custom)")
	fs.Float64Var(&req.Brightness, "brightness", 0, "brightness multiplier (with --filter custom)")
	fs.Float64Var(&req.Saturation, "saturation", 0, "saturation multiplier (with --filter custom)")
	fs.IntVar(&req.Hue, "hue", 0, "hue rotation in degrees (with --filter custom)")
	fs.IntVar(&req.Lightness, "lightness", 0, "lightness addend (with --filter custom)")

	output := fs.String("output", "", "save the picture to `file` instead of displaying it (\"-\" for stdout)")
	fs.StringVar(output, "o", "", "shorthand for --output")
	printURL := fs.Bool("url", false, "only print the picture URL")
	columns := fs.Int("cols", 0, "picture width in terminal `columns` (default: terminal width, max 100)")
	mode := fs.String("mode", envOr("CATAAS_MODE", "auto"), "render `mode`: auto, kitty, iterm, blocks (env CATAAS_MODE)")
	loops := fs.Int("loops", 3, "how many times animated cats are played in blocks mode")

	positional, err := parse(fs, args)
	if err != nil {
		return err
	}
	// Positional arguments are tags: "cataas cute orange".
	for _, a := range positional {
		tags.Set(a)
	}
	req.Tags = tags

	renderMode, err := render.ParseMode(*mode)
	if err != nil {
		return err
	}

	client := cataas.New(*baseURL)
	if *printURL {
		fmt.Fprintln(stdout, client.CatURL(req))
		return nil
	}

	img, err := client.Cat(ctx, req)
	if err != nil {
		return err
	}

	stdoutIsTTY := isTerminal(stdout)
	switch {
	case *output == "-" || (*output == "" && !stdoutIsTTY):
		// Piped: write the raw picture, e.g. "cataas > cat.jpg".
		_, err := stdout.Write(img.Data)
		return err
	case *output != "":
		if err := os.WriteFile(*output, img.Data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "Saved %s (%d bytes)\n", *output, len(img.Data))
		return nil
	}

	width, height := terminalSize(stdout)
	cols := *columns
	maxRows := 0
	if cols <= 0 {
		cols = min(width, 100)
		maxRows = height - 2
	}
	return render.Render(ctx, stdout, img.Data, render.Options{
		Mode:    renderMode,
		Columns: cols,
		MaxRows: maxRows,
		Loops:   *loops,
	})
}

func runTags(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs, baseURL := newFlagSet("cataas tags", stderr, "Usage: cataas tags [options] [filter]\n\nList the available tags, optionally only those containing filter.\n")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parse(fs, args)
	if err != nil {
		return err
	}
	tags, err := cataas.New(*baseURL).Tags(ctx)
	if err != nil {
		return err
	}
	if filter := strings.ToLower(strings.Join(positional, " ")); filter != "" {
		kept := tags[:0]
		for _, t := range tags {
			if strings.Contains(strings.ToLower(t), filter) {
				kept = append(kept, t)
			}
		}
		tags = kept
	}
	if *asJSON {
		return writeJSON(stdout, tags)
	}
	for _, t := range tags {
		fmt.Fprintln(stdout, t)
	}
	return nil
}

func runList(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs, baseURL := newFlagSet("cataas list", stderr, "Usage: cataas list [options] [tag...]\n\nList cats with their id and tags.\n")
	var tags listFlag
	fs.Var(&tags, "tag", "only cats having this `tag` (repeatable, or comma separated)")
	fs.Var(&tags, "t", "shorthand for --tag")
	limit := fs.Int("limit", 10, "maximum number of cats")
	skip := fs.Int("skip", 0, "number of cats to skip")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parse(fs, args)
	if err != nil {
		return err
	}
	for _, a := range positional {
		tags.Set(a)
	}
	cats, err := cataas.New(*baseURL).Cats(ctx, cataas.ListRequest{Tags: tags, Limit: *limit, Skip: *skip})
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(stdout, cats)
	}
	for _, c := range cats {
		fmt.Fprintf(stdout, "%s\t%s\n", c.ID, strings.Join(c.Tags, ", "))
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func terminalSize(w io.Writer) (int, int) {
	if f, ok := w.(*os.File); ok {
		if width, height, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			return width, height
		}
	}
	return 80, 24
}
