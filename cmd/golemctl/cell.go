package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var cellCmd = &cobra.Command{
	Use:   "cell",
	Short: "Cell management operations",
}

var cellLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all cells",
	RunE:  runCellLs,
}

var cellDrainCmd = &cobra.Command{
	Use:   "drain <cell-id>",
	Short: "Drain a cell (stops accepting new appends)",
	Args:  cobra.ExactArgs(1),
	RunE:  runCellDrain,
}

var cellMigrateCmd = &cobra.Command{
	Use:   "migrate <tenant-id> <to-cell> [--dry-run]",
	Short: "Migrate a tenant to a target cell",
	Args:  cobra.ExactArgs(2),
	RunE:  runCellMigrate,
}

var dryRunFlag bool

func init() {
	cellCmd.AddCommand(cellLsCmd)
	cellCmd.AddCommand(cellDrainCmd)
	cellMigrateCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Perform dry-run (no mutations)")
	cellCmd.AddCommand(cellMigrateCmd)
}

func runCellLs(cmd *cobra.Command, args []string) error {
	url := getAdminURL("/admin/cells/")
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		return fmt.Errorf("request failed: %s", resp.Status)
	}

	var cells []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&cells); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "CELL ID\tREGION\tSTATUS\n")
	for _, c := range cells {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			c["cell_id"], c["region"], c["status"])
	}
	return w.Flush()
}

func runCellDrain(cmd *cobra.Command, args []string) error {
	cellID := args[0]
	url := getAdminURL("/admin/cells/" + cellID + "/drain")
	body := `{"force":false}`
	req, err := http.NewRequest(http.MethodPost, url, stringReader(body))
	if err != nil {
		return err
	}
	setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("drain failed: %s", resp.Status)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Cell %s: %s\n", cellID, result["message"])
	return nil
}

func runCellMigrate(cmd *cobra.Command, args []string) error {
	tenantID := args[0]
	toCell := args[1]
	url := getAdminURL("/admin/cells/" + toCell + "/migrate")

	reqBody := map[string]any{
		"tenant_id": tenantID,
		"to_cell":   toCell,
		"dry_run":   dryRunFlag,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, url, bodyReader(bodyBytes))
	if err != nil {
		return err
	}
	setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("migration failed: %s", resp.Status)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Migration: %s → %s (%s)\n",
		result["from_cell"], result["to_cell"], result["message"])
	return nil
}
