// Command golemctl is the GOLEM CLI for GOLEM operators (ADR-081).
//
// Minimal viable product: cell ls/drain/migrate, tenant ls/export,
// slo show/budget, dr snapshot/restore, meter query.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "golemctl",
	Short: "golemctl is the GOLEM operator CLI",
	Long: `golemctl provides operator access to GOLEM administrative operations:
cell management, tenant management, SLO monitoring, disaster recovery, and metering.

Authentication: set GOLEMCTL_TOKEN environment variable for OIDC bearer token.`,
}

func main() {
	root.AddCommand(cellCmd)
	root.AddCommand(tenantCmd)
	root.AddCommand(sloCmd)
	root.AddCommand(drCmd)
	root.AddCommand(meterCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
