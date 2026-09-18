package main

import (
	"encoding/csv"
	"fmt"
	"os"
)

// Device represents a single row from the devices CSV file
type Device struct {
	DeviceName string
	IPAddress  string
	Secret     string
}

// LoadDevices reads the CSV file and returns a slice of Device structs.
// Expects headers: device_name, ip_address, secret
func LoadDevices(csvFile string) ([]Device, error) {
	f, err := os.Open(csvFile)
	if err != nil {
		return nil, fmt.Errorf("CSV file not found: %s: %w", csvFile, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file %s: %w", csvFile, err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file %s is empty", csvFile)
	}

	header := records[0]
	colIdx := map[string]int{}
	for i, col := range header {
		colIdx[col] = i
	}

	required := []string{"device_name", "ip_address", "secret"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("CSV file must contain columns %v, found: %v", required, header)
		}
	}

	var devices []Device
	for _, row := range records[1:] {
		if len(row) == 0 {
			continue
		}
		devices = append(devices, Device{
			DeviceName: row[colIdx["device_name"]],
			IPAddress:  row[colIdx["ip_address"]],
			Secret:     row[colIdx["secret"]],
		})
	}

	return devices, nil
}
