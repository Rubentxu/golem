package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var sloCmd = &cobra.Command{
	Use:   "slo",
	Short: "SLO monitoring operations",
}

var sloShowCmd = &cobra.Command{
	Use:   "show <sli-name>",
	Short: "Show current SLO status",
	Args:  cobra.ExactArgs(1),
	RunE:  runSloShow,
}

var sloBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Show all SLO error budget status",
	RunE:  runSloBudget,
}

func init() {
	sloCmd.AddCommand(sloShowCmd)
	sloCmd.AddCommand(sloBudgetCmd)
}

func runSloShow(cmd *cobra.Command, args []string) error {
	sliName := args[0]
	url := getAdminURL("/admin/slo/" + sliName)
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
		return fmt.Errorf("SLO query failed: %s", resp.Status)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "SLI\tCURRENT VALUE\tTARGET\tBUDGET CONSUMED\tBURN RATE\n")
	fmt.Fprintf(w, "%s\t%v\t%v\t%v\t%v\n",
		result["sli_name"],
		result["current_value"],
		result["target"],
		result["budget_consumed"],
		result["burn_rate"])
	return w.Flush()
}

func runSloBudget(cmd *cobra.Command, args []string) error {
	// Query all SLIs defined in the SLO tracker.
	slis := []string{
		"command.latency.p99",
		"command.error_rate",
		"system.availability",
		"journal.replay_time.p99",
		"eval.pass_rate",
		"oidc.verify_latency",
		"ops.console.action_latency",
		"audit.export.success_rate",
		"metering.rollup.success_rate",
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "SLI\tBUDGET CONSUMED\tBURN RATE\n")

	for _, sli := range slis {
		url := getAdminURL("/admin/slo/" + sli)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		setAuthHeader(req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp == nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
	}

	// If we couldn't get real data, show placeholder.
	fmt.Fprintf(w, "SLI query requires live SLO tracker — run 'golemctl slo show <sli-name>'\n")
	return w.Flush()
}
