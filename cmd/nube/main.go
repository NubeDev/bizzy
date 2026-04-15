// nube is the CLI for the NubeIO central server.
//
// Commands are generated from the OpenAPI spec at api/openapi.yaml.
// The spec is embedded in the binary — no external files needed.
//
// Usage:
//
//	nube login http://localhost:8090 <token>
//	nube status
//	nube apps list
//	nube apps install rubix-developer --settings rubix_host=http://localhost:9001
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/NubeDev/bizzy/pkg/cli"
	"github.com/NubeDev/bizzy/pkg/cli/openapi"
)

//go:embed openapi.yaml
var specData []byte

func main() {
	root := cli.NewRootCmd()

	// Hand-written commands.
	root.AddCommand(cli.NewLoginCmd())
	root.AddCommand(cli.NewLogoutCmd())
	root.AddCommand(cli.NewAskCmd())

	// Auto-generated commands from OpenAPI spec.
	if err := openapi.RegisterCommands(root, specData); err != nil {
		fmt.Fprintf(os.Stderr, "error loading spec: %v\n", err)
		os.Exit(1)
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
