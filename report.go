package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// DeviceResult holds the outcome of configuring a single device
type DeviceResult struct {
	DeviceName string
	IPAddress  string
	Status     string // "success" or "fail"
	AAAState   string
	Error      string
}

// PrintReport prints a formatted summary table to stdout and optionally
// writes the results to a CSV report file.
func PrintReport(results []DeviceResult, reportFile string) error {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 90))
	fmt.Println("TACACS+ Configuration Report")
	fmt.Println(strings.Repeat("=", 90))
	fmt.Printf("%-20s%-18s%-10s%-25s%s\n", "Device Name", "IP Address", "Status", "AAA State", "Error")
	fmt.Println(strings.Repeat("-", 90))

	successCount := 0
	for _, r := range results {
		aaaShort := r.AAAState
		if len(aaaShort) > 25 {
			aaaShort = aaaShort[:22] + "..."
		}
		fmt.Printf("%-20s%-18s%-10s%-25s%s\n", r.DeviceName, r.IPAddress, r.Status, aaaShort, r.Error)

		if r.Status == "success" {
			successCount++
		}
	}

	fmt.Println(strings.Repeat("=", 90))
	failCount := len(results) - successCount
	fmt.Printf("Total: %d | Success: %d | Failed: %d\n\n", len(results), successCount, failCount)

	if reportFile != "" {
		if err := writeReportCSV(results, reportFile); err != nil {
			return err
		}
		fmt.Printf("Report written to: %s\n", reportFile)
	}

	return nil
}

func writeReportCSV(results []DeviceResult, reportFile string) error {
	f, err := os.Create(reportFile)
	if err != nil {
		return fmt.Errorf("failed to create report file %s: %w", reportFile, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"device_name", "ip_address", "status", "aaa_state", "error"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range results {
		row := []string{r.DeviceName, r.IPAddress, r.Status, r.AAAState, r.Error}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
