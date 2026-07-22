// Command thready is the Helix Thready headless CLI entrypoint. It constructs a
// real sdk_go client from environment configuration, wraps it in the production
// SDKAdapter, and hands control to cli.Run.
//
// Configuration (environment):
//
//	THREADY_BASE_URL   gateway origin (default http://127.0.0.1:8080)
//	THREADY_TOKEN      JWT bearer access token (optional; `login` obtains one)
//	THREADY_API_KEY    scoped API key for non-interactive use (optional)
//
// The process exit code is whatever cli.Run returns (0 ok, 1 error, 2 usage).
package main

import (
	"fmt"
	"os"

	cli "digital.vasic.threadycli"
	thready "digital.vasic.threadysdk"
)

func main() {
	baseURL := os.Getenv("THREADY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	sdk, err := thready.New(thready.Config{
		BaseURL:     baseURL,
		AccessToken: os.Getenv("THREADY_TOKEN"),
		APIKey:      os.Getenv("THREADY_API_KEY"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "thready: %v\n", err)
		os.Exit(1)
	}

	client := cli.NewSDKAdapter(sdk)
	os.Exit(cli.Run(os.Args[1:], client, os.Stdout, os.Stderr))
}
