package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var meterCmd = &cobra.Command{
	Use:   "meter",
	Short: "Metering query operations",
}

var meterQueryCmd = &cobra.Command{
	Use:   "query <tenant-id>",
	Short: "Query metering data for a tenant (last 24h)",
	Args:  cobra.ExactArgs(1),
	RunE:  runMeterQuery,
}

func init() {
	meterCmd.AddCommand(meterQueryCmd)
}

func runMeterQuery(cmd *cobra.Command, args []string) error {
	tenantID := args[0]
	url := getAdminURL("/admin/metering?tenant=" + tenantID)
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
		return fmt.Errorf("metering query failed: %s", resp.Status)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	rollups, ok := result["rollups"].([]any)
	if !ok {
		fmt.Println("No metering data available")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tCAPABILITY\tTOTAL UNITS\tTOTAL COST\n")
	for _, r := range rollups {
		rm := r.(map[string]any)
		fmt.Fprintf(w, "%s\t%s\t%d\t$%.4f\n",
			rm["tenant_id"], rm["capability"],
			int64(rm["total_units"].(float64)),
			rm["total_cost_usd"].(float64))
	}
	return w.Flush()
}
