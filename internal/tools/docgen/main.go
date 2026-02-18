package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"walkscape-helper/internal/cli"

	"github.com/spf13/cobra/doc"
)

func main() {
	out := flag.String("out", "./docs/cli", "output directory")
	format := flag.String("format", "markdown", "markdown|man|rest")
	frontMatter := flag.Bool("frontmatter", false, "prepend yaml front matter for markdown")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	root := cli.NewRootCmd()
	root.DisableAutoGenTag = true

	switch *format {
	case "markdown":
		if *frontMatter {
			prepender := func(filename string) string {
				base := filepath.Base(filename)
				slug := strings.TrimSuffix(base, filepath.Ext(base))
				title := strings.ReplaceAll(slug, "_", " ")
				return fmt.Sprintf("---\ntitle: %q\nslug: %q\ndescription: %q\n---\n\n", title, slug, "CLI reference for "+title)
			}
			if err := doc.GenMarkdownTreeCustom(root, *out, prepender, strings.ToLower); err != nil {
				log.Fatal(err)
			}
			return
		}

		if err := doc.GenMarkdownTree(root, *out); err != nil {
			log.Fatal(err)
		}
	case "man":
		head := &doc.GenManHeader{Title: strings.ToUpper(root.Name()), Section: "1"}
		if err := doc.GenManTree(root, head, *out); err != nil {
			log.Fatal(err)
		}
	case "rest":
		if err := doc.GenReSTTree(root, *out); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown format: %s", *format)
	}
}
