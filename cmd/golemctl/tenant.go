package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Tenant management operations",
}

var tenantLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all tenants",
	RunE:  runTenantLs,
}

var tenantExportCmd = &cobra.Command{
	Use:   "export <tenant-id>",
	Short: "Export tenant data as manifest",
	Args:  cobra.ExactArgs(1),
	RunE:  runTenantExport,
}

func init() {
	tenantCmd.AddCommand(tenantLsCmd)
	tenantCmd.AddCommand(tenantExportCmd)
}

func runTenantLs(cmd *cobra.Command, args []string) error {
	// Uses the admin API to list tenants.
	// Note: This requires a proper TenantCatalog implementation.
	// For now, we show a placeholder since TenantCatalog is in control plane.
	fmt.Println("Tenant listing requires TenantCatalog — not yet wired in data plane")
	return nil
}

func runTenantExport(cmd *cobra.Command, args []string) error {
	tenantID := args[0]
	url := getAdminURL("/admin/tenants/" + tenantID + "/export")
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	setAuthHeader(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("export failed: %s", resp.Status)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT ID\tEXPORTED AT\n")
	fmt.Fprintf(w, "%s\t%s\n", tenantID, result["exported_at"])
	w.Flush()

	if manifest, ok := result["manifest"].(string); ok {
		fmt.Println("\nManifest:")
		fmt.Println(manifest)
	}
	return nil
}
