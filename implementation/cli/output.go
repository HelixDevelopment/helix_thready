package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Exit codes returned by Run. They follow the common CLI convention:
//
//	0  success
//	1  a runtime / API error (the request reached the client and failed)
//	2  a usage error (unknown command, bad flags, missing required argument)
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// usage is the top-level help text, printed for `help`, no args, and unknown
// commands.
const usage = `thready — Helix Thready headless CLI (drives the /v1 control API)

Usage:
  thready <command> [flags]

Commands:
  login                 Authenticate and print the access token
  channels list         List registered channels
  channels add          Register a channel (--name required)
  post get <id>         Fetch a single post
  post reprocess <id>   Queue a fresh processing run for a post (202)
  search <query>        Search posts and generated materials
  skills                List Skill-Graph knowledge units
  whoami                Show the authenticated identity
  help                  Show this help

Global flag (accepted by every command):
  --json                Emit machine-readable JSON instead of a table

Examples:
  thready login --email me@example.com --password s3cret
  thready channels add --name "Design QA" --platform telegram --external-ref -100123
  thready post reprocess post_42
  thready search "vector db" --mode semantic --top-k 5 --sources posts,generated --rerank
`

// printJSON writes v as indented JSON followed by a newline.
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// newTab builds a tabwriter with the CLI's standard column padding.
func newTab(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
}

// fail reports a runtime/API error on stderr and returns the error exit code.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "thready: %v\n", err)
	return exitError
}

// usageErr reports a usage problem on stderr and returns the usage exit code.
func usageErr(stderr io.Writer, format string, a ...any) int {
	fmt.Fprintf(stderr, "thready: "+format+"\n", a...)
	return exitUsage
}
