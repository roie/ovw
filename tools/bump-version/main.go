package main

import (
	"fmt"
	"os"
	"regexp"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/bump-version <version>")
		os.Exit(2)
	}
	version := os.Args[1]
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`).MatchString(version) {
		fmt.Fprintf(os.Stderr, "invalid version %q\n", version)
		os.Exit(2)
	}

	replacements := []struct {
		path string
		from *regexp.Regexp
		to   string
	}{
		{
			path: "internal/buildinfo/buildinfo.go",
			from: regexp.MustCompile(`var Version = "([^"]+)"`),
			to:   fmt.Sprintf(`var Version = "%s"`, version),
		},
		{
			path: "web/index.html",
			from: regexp.MustCompile(`"softwareVersion": "([^"]+)"`),
			to:   fmt.Sprintf(`"softwareVersion": "%s"`, version),
		},
		{
			path: "web/index.html",
			from: regexp.MustCompile(`<span class="version">([^<]+)</span>`),
			to:   fmt.Sprintf(`<span class="version">%s</span>`, version),
		},
	}

	for _, replacement := range replacements {
		data, err := os.ReadFile(replacement.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", replacement.path, err)
			os.Exit(1)
		}
		text := string(data)
		if !replacement.from.MatchString(text) {
			fmt.Fprintf(os.Stderr, "%s: version pattern not found\n", replacement.path)
			os.Exit(1)
		}
		text = replacement.from.ReplaceAllString(text, replacement.to)
		if err := os.WriteFile(replacement.path, []byte(text), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", replacement.path, err)
			os.Exit(1)
		}
	}

	fmt.Printf("updated version to %s\n", version)
}
