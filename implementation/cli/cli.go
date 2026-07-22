// Package cli is the Helix Thready headless command-line front end. It drives the
// same `/v1` control API that the sdk_go client and rest_gateway implement, for
// pipeline and automation use.
//
// The package is split so the command layer is testable in isolation:
//
//   - Run + the subcommand handlers depend ONLY on the APIClient interface and
//     CLI-local DTOs (client.go) — no SDK, no network. They are exercised in
//     cli_test.go against an in-memory fake.
//   - SDKAdapter (adapter.go) is the thin production implementation of APIClient;
//     it wraps the real digital.vasic.threadysdk client and is compile-checked
//     against it.
//   - cmd/thready/main.go wires a real SDK client + adapter to os.Args.
//
// Output is a human-readable table by default and JSON when --json is passed.
// Run returns a process exit code (see output.go).
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Run parses args (the program arguments WITHOUT argv[0]), dispatches to the
// matching subcommand, and returns a process exit code. All human output goes to
// stdout; diagnostics and usage go to stderr. The provided APIClient is the only
// external dependency, which makes Run fully unit-testable with a fake.
func Run(args []string, client APIClient, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	ctx := context.Background()
	cmd, rest := args[0], args[1:]

	switch cmd {
	case "login":
		return cmdLogin(ctx, client, rest, stdout, stderr)
	case "channels":
		return cmdChannels(ctx, client, rest, stdout, stderr)
	case "post":
		return cmdPost(ctx, client, rest, stdout, stderr)
	case "search":
		return cmdSearch(ctx, client, rest, stdout, stderr)
	case "skills":
		return cmdSkills(ctx, client, rest, stdout, stderr)
	case "whoami":
		return cmdWhoami(ctx, client, rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "thready: unknown command %q\n\n", cmd)
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// parseInterspersed parses fs against args while tolerating positional arguments
// mixed in with flags in any order (the stdlib flag package otherwise stops at
// the first non-flag token). It returns the collected positionals. A flag parse
// error (bad syntax, unknown flag) is returned to the caller.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, args[0])
		args = args[1:]
	}
}

// newFlagSet builds a ContinueOnError flag set that reports to stderr and
// registers the shared --json flag, returning the set and the bound bool.
func newFlagSet(name string, stderr io.Writer) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of a table")
	return fs, asJSON
}

// ----- login -----

func cmdLogin(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("login", stderr)
	email := fs.String("email", "", "account email")
	passwordFlag := fs.String("password", "", "account password (INSECURE: visible in the process list; prefer the THREADY_PASSWORD env var)")
	totp := fs.String("totp", "", "TOTP code (required for admin tiers)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return exitUsage
	}

	// Resolve the password securely. The THREADY_PASSWORD env var is the
	// preferred path: it is not exposed in argv (ps, /proc/<pid>/cmdline) or the
	// shell history. A --password flag still works for compatibility, but a
	// value passed there is visible to other processes, so we warn whenever one
	// is present. Precedence: THREADY_PASSWORD wins when set; otherwise --password.
	password := os.Getenv("THREADY_PASSWORD")
	if *passwordFlag != "" {
		fmt.Fprintln(stderr, "warning: --password on the command line is visible to other processes; prefer THREADY_PASSWORD")
		if password == "" {
			password = *passwordFlag
		}
	}
	if *email == "" || password == "" {
		return usageErr(stderr, "login requires --email and --password")
	}

	tp, err := client.Login(ctx, Credentials{Email: *email, Password: password, TOTP: *totp})
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, tp); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "access_token: %s\n", tp.AccessToken)
	fmt.Fprintf(stdout, "token_type:   %s\n", tp.TokenType)
	fmt.Fprintf(stdout, "expires_in:   %ds\n", tp.ExpiresIn)
	return exitOK
}

// ----- channels -----

func cmdChannels(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usageErr(stderr, "channels requires a subcommand: list | add")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdChannelsList(ctx, client, rest, stdout, stderr)
	case "add":
		return cmdChannelsAdd(ctx, client, rest, stdout, stderr)
	default:
		return usageErr(stderr, "unknown channels subcommand %q (want: list | add)", sub)
	}
}

func cmdChannelsList(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("channels list", stderr)
	if _, err := parseInterspersed(fs, args); err != nil {
		return exitUsage
	}

	channels, err := client.ListChannels(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, channels); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	tw := newTab(stdout)
	fmt.Fprintln(tw, "ID\tNAME\tPLATFORM\tEXTERNAL_REF\tCREATED_AT")
	for _, ch := range channels {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			ch.ID, ch.Name, ch.Platform, ch.ExternalRef, ch.CreatedAt.Format(time.RFC3339))
	}
	tw.Flush()
	return exitOK
}

func cmdChannelsAdd(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("channels add", stderr)
	name := fs.String("name", "", "channel display name (required)")
	platform := fs.String("platform", "", "source platform (telegram | max | ...)")
	externalRef := fs.String("external-ref", "", "platform-native channel/group id")
	if _, err := parseInterspersed(fs, args); err != nil {
		return exitUsage
	}
	if *name == "" {
		return usageErr(stderr, "channels add requires --name")
	}

	ch, err := client.CreateChannel(ctx, CreateChannelInput{
		Name:        *name,
		Platform:    *platform,
		ExternalRef: *externalRef,
	})
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, ch); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "created channel %s\n", ch.ID)
	fmt.Fprintf(stdout, "name:         %s\n", ch.Name)
	fmt.Fprintf(stdout, "platform:     %s\n", ch.Platform)
	fmt.Fprintf(stdout, "external_ref: %s\n", ch.ExternalRef)
	return exitOK
}

// ----- post -----

func cmdPost(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usageErr(stderr, "post requires a subcommand: get <id> | reprocess <id>")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "get":
		return cmdPostGet(ctx, client, rest, stdout, stderr)
	case "reprocess":
		return cmdPostReprocess(ctx, client, rest, stdout, stderr)
	default:
		return usageErr(stderr, "unknown post subcommand %q (want: get | reprocess)", sub)
	}
}

func cmdPostGet(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("post get", stderr)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 1 {
		return usageErr(stderr, "post get requires exactly one <id>")
	}

	post, err := client.GetPost(ctx, pos[0])
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, post); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "id:         %s\n", post.ID)
	fmt.Fprintf(stdout, "channel_id: %s\n", post.ChannelID)
	fmt.Fprintf(stdout, "status:     %s\n", post.Status)
	fmt.Fprintf(stdout, "hashtags:   %s\n", strings.Join(post.Hashtags, ", "))
	fmt.Fprintf(stdout, "categories: %s\n", strings.Join(post.Categories, ", "))
	fmt.Fprintf(stdout, "body:       %s\n", post.Body)
	return exitOK
}

func cmdPostReprocess(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("post reprocess", stderr)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 1 {
		return usageErr(stderr, "post reprocess requires exactly one <id>")
	}

	job, err := client.Reprocess(ctx, pos[0])
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, job); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	fmt.Fprintln(stdout, "202 Accepted")
	fmt.Fprintf(stdout, "job_id:     %s\n", job.JobID)
	fmt.Fprintf(stdout, "post_id:    %s\n", job.PostID)
	fmt.Fprintf(stdout, "status:     %s\n", job.Status)
	fmt.Fprintf(stdout, "precedence: %s\n", strings.Join(job.Precedence, " > "))
	return exitOK
}

// ----- search -----

func cmdSearch(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("search", stderr)
	mode := fs.String("mode", "", "search mode: semantic | keyword | hybrid")
	sources := fs.String("sources", "", "comma-separated corpora: posts,generated,assets")
	topK := fs.Int("top-k", 0, "max results to return")
	rerank := fs.Bool("rerank", false, "apply a rerank pass over the candidates")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 1 {
		return usageErr(stderr, "search requires exactly one <query>")
	}

	opts := SearchOptions{
		Mode:    *mode,
		Sources: splitCSV(*sources),
		TopK:    *topK,
		Rerank:  *rerank,
	}
	res, err := client.Search(ctx, pos[0], opts)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, res); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	tw := newTab(stdout)
	fmt.Fprintln(tw, "SOURCE_ID\tKIND\tSCORE\tSNIPPET")
	for _, hit := range res.Results {
		fmt.Fprintf(tw, "%s\t%s\t%.3f\t%s\n", hit.SourceID, hit.Kind, hit.Score, hit.Snippet)
	}
	tw.Flush()
	fmt.Fprintf(stdout, "(%d results in %dms via %s)\n", len(res.Results), res.TookMs, res.Embedder)
	return exitOK
}

// splitCSV splits a comma-separated flag value into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, tok := range strings.Split(s, ",") {
		if t := strings.TrimSpace(tok); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ----- skills -----

func cmdSkills(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("skills", stderr)
	if _, err := parseInterspersed(fs, args); err != nil {
		return exitUsage
	}

	skills, err := client.ListSkills(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, skills); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	tw := newTab(stdout)
	fmt.Fprintln(tw, "ID\tNAME\tKIND\tSORT_ORDER")
	for _, sk := range skills {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", sk.ID, sk.Name, sk.Kind, sk.SortOrder)
	}
	tw.Flush()
	return exitOK
}

// ----- whoami -----

func cmdWhoami(ctx context.Context, client APIClient, args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlagSet("whoami", stderr)
	if _, err := parseInterspersed(fs, args); err != nil {
		return exitUsage
	}

	id, err := client.Whoami(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		if err := printJSON(stdout, id); err != nil {
			return fail(stderr, err)
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "subject:       %s\n", id.Subject)
	fmt.Fprintf(stdout, "email:         %s\n", id.Email)
	fmt.Fprintf(stdout, "tier:          %s\n", id.Tier)
	fmt.Fprintf(stdout, "token_present: %t\n", id.TokenPresent)
	return exitOK
}
