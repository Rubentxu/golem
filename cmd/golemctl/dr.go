package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var drCmd = &cobra.Command{
	Use:   "dr",
	Short: "Disaster recovery operations",
}

var drSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Trigger a journal snapshot (backup)",
	RunE:  runDrSnapshot,
}

var drRestoreCmd = &cobra.Command{
	Use:   "restore <snapshot-id>",
	Short: "Restore from a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runDrRestore,
}

func init() {
	drCmd.AddCommand(drSnapshotCmd)
	drCmd.AddCommand(drRestoreCmd)
}

func runDrSnapshot(cmd *cobra.Command, args []string) error {
	// Snapshot is triggered via the worker, not directly.
	// For now, emit a message about the operation.
	fmt.Println("Snapshot must be triggered via the worker scheduler (daily at 03:00 UTC)")
	fmt.Println("To trigger manually, use the worker API:")
	fmt.Println("  POST /admin/worker/snapshot")
	return nil
}

func runDrRestore(cmd *cobra.Command, args []string) error {
	snapshotID := args[0]
	url := getAdminURL("/admin/dr/restore")
	reqBody := map[string]string{"snapshot_id": snapshotID}
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
		return fmt.Errorf("restore failed: %s", resp.Status)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Restore %s: %s\n", snapshotID, result["message"])
	return nil
}
