// Command thready is the Helix Thready headless CLI entrypoint. It constructs a
// real sdk_go client from environment configuration, wraps it in the production
// SDKAdapter, and hands control to cli.Run.
//
// Configuration (environment):
//
//	THREADY_BASE_URL             gateway origin (default http://127.0.0.1:8080)
//	THREADY_TOKEN                JWT bearer access token (optional; `login` obtains one)
//	THREADY_API_KEY              scoped API key for non-interactive use (optional)
//	THREADY_PASSWORD             login password (preferred over `login --password`,
//	                             which is visible to other processes via argv)
//	THREADY_ALLOW_INSECURE_HTTP  set to a truthy value ("1"/"true") to allow the
//	                             SDK to send credentials over plaintext http to a
//	                             remote host (default off — see below)
//
// Security: for any REMOTE gateway use an https base URL. The SDK refuses to
// attach the bearer token or API key to a plaintext-http request bound for a
// non-loopback host (returning ErrInsecureTransport) unless
// THREADY_ALLOW_INSECURE_HTTP is set. The default loopback origin
// (http://127.0.0.1:8080) is always permitted.
//
// The process exit code is whatever cli.Run returns (0 ok, 1 error, 2 usage).
package main

import (
	"fmt"
	"os"
	"strconv"

	cli "digital.vasic.threadycli"
	thready "digital.vasic.threadysdk"
)

func main() {
	baseURL := os.Getenv("THREADY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	// Opt-in escape hatch for plaintext http to a remote host. Off by default;
	// an unparseable value is treated as false.
	allowInsecure, _ := strconv.ParseBool(os.Getenv("THREADY_ALLOW_INSECURE_HTTP"))

	sdk, err := thready.New(thready.Config{
		BaseURL:           baseURL,
		AccessToken:       os.Getenv("THREADY_TOKEN"),
		APIKey:            os.Getenv("THREADY_API_KEY"),
		AllowInsecureHTTP: allowInsecure,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "thready: %v\n", err)
		os.Exit(1)
	}

	client := cli.NewSDKAdapter(sdk)
	os.Exit(cli.Run(os.Args[1:], client, os.Stdout, os.Stderr))
}
