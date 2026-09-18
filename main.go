package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// AAAStateCheckDelay mirrors the Python script's wait before checking
// the tacacs_server_state_map after applying config.
const AAAStateCheckDelay = 5 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Fatal error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := LoadConfig(DefaultConfigFile)
	if err != nil {
		return err
	}

	devices, err := LoadDevices(cfg.Files.CSVFile)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("no devices found in %s", cfg.Files.CSVFile)
	}

	var results []DeviceResult
	for _, d := range devices {
		result := configureDevice(cfg, d)
		results = append(results, result)
	}

	return PrintReport(results, cfg.Files.ReportFile)
}

// connect wraps NewVisionWebApi, matching the requested "connect" function
// for the VisionWebApi (nto) client.
func connect(creds Creds, host string) (*VisionWebApi, error) {
	return NewVisionWebApi(
		host,
		creds.Username,
		creds.Password,
		creds.Port,
		creds.Debug,
		time.Duration(creds.Timeout)*time.Second,
		creds.Retries,
	)
}

// buildTacacsConfig builds a per-device tacacs_servers payload, injecting
// the device's secret and the aaa_username from creds into the server entry.
func buildTacacsConfig(base map[string]interface{}, secret, aaaUsername string) map[string]interface{} {
	tacacsConfig := deepCopyMap(base)

	serverDefaults := map[string]interface{}{}
	if sd, ok := tacacsConfig["server_defaults"].(map[string]interface{}); ok {
		serverDefaults = deepCopyMap(sd)
	}
	delete(tacacsConfig, "server_defaults")

	serverEntry := deepCopyMap(serverDefaults)
	serverEntry["secret"] = secret
	serverEntry["aaa_username"] = aaaUsername

	tacacsConfig["servers"] = []interface{}{serverEntry}

	return tacacsConfig
}

// deepCopyMap performs a JSON round-trip deep copy of a map, equivalent
// in effect to Python's copy.deepcopy for JSON-serializable structures.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	data, err := json.Marshal(m)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

// testAAAConnectivity waits for the device's background AAA validation to
// run, then polls and returns the tacacs_server_state_map.
func testAAAConnectivity(nto *VisionWebApi) (interface{}, error) {
	time.Sleep(AAAStateCheckDelay)
	return nto.GetSystemProperty("tacacs_server_state_map")
}

// configureDevice connects to a single device, applies TACACS+ config,
// tests AAA connectivity, and returns a DeviceResult for reporting.
func configureDevice(cfg *Config, device Device) DeviceResult {
	result := DeviceResult{
		DeviceName: device.DeviceName,
		IPAddress:  device.IPAddress,
		Status:     "fail",
	}

	nto, err := connect(cfg.Creds, device.IPAddress)
	if err != nil {
		result.Error = err.Error()
		fmt.Fprintf(os.Stderr, "[%s] Error connecting: %v\n", device.DeviceName, err)
		return result
	}
	defer func() {
		if logoutErr := nto.Logout(); logoutErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: logout failed: %v\n", device.DeviceName, logoutErr)
		}
	}()

	tacacsConfig := buildTacacsConfig(cfg.TacacsConfig, device.Secret, cfg.Creds.AAAUsername)

	systemArgs := map[string]interface{}{
		"tacacs_servers":      tacacsConfig,
		"authentication_mode": "TACACS",
	}

	fmt.Printf("[%s] Applying TACACS+ configuration to %s...\n", device.DeviceName, device.IPAddress)
	modifyResult, err := nto.ModifySystem(systemArgs)
	if err != nil {
		result.Error = err.Error()
		fmt.Fprintf(os.Stderr, "[%s] Error applying TACACS+ config: %v\n", device.DeviceName, err)
		return result
	}
	fmt.Printf("[%s] modifySystem result: %v\n", device.DeviceName, modifyResult)

	// Re-authenticate to verify config (mirrors Python's re-connect)
	nto2, err := connect(cfg.Creds, device.IPAddress)
	if err != nil {
		result.Error = fmt.Sprintf("failed to reconnect for verification: %v", err)
		fmt.Fprintf(os.Stderr, "[%s] %s\n", device.DeviceName, result.Error)
		return result
	}
	defer func() {
		if logoutErr := nto2.Logout(); logoutErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: logout failed (verify conn): %v\n", device.DeviceName, logoutErr)
		}
	}()

	tacacs, err := nto2.GetSystemProperty("tacacs_servers")
	if err != nil {
		result.Error = err.Error()
		return result
	}
	mode, err := nto2.GetSystemProperty("authentication_mode")
	if err != nil {
		result.Error = err.Error()
		return result
	}

	tacacsJSON, _ := json.MarshalIndent(tacacs, "", "    ")
	modeJSON, _ := json.MarshalIndent(mode, "", "    ")
	fmt.Printf("[%s] Updated tacacs config: %s\n", device.DeviceName, string(tacacsJSON))
	fmt.Printf("[%s] Updated tacacs mode: %s\n", device.DeviceName, string(modeJSON))

	fmt.Printf("[%s] Testing AAA connectivity as '%s'...\n", device.DeviceName, cfg.Creds.AAAUsername)
	state, err := testAAAConnectivity(nto2)
	if err != nil {
		result.Error = fmt.Sprintf("AAA connectivity test failed: %v", err)
		fmt.Fprintf(os.Stderr, "[%s] %s\n", device.DeviceName, result.Error)
		return result
	}

	stateJSON, _ := json.Marshal(state)
	fmt.Printf("[%s] tacacs_server_state_map: %s\n", device.DeviceName, string(stateJSON))

	result.AAAState = string(stateJSON)
	result.Status = "success"

	return result
}
