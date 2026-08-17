// Command golemctl is the GOLEM CLI.
//
// Skeleton for Fase 0 (bootstrap). Subcommands arrive with the kernel
// (journal inspect, replay, digest verification, provider profiles).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "golemctl: skeleton — no subcommands yet")
	os.Exit(2)
}
