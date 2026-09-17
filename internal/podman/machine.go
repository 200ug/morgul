package podman

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

// Reports whether the host runs darwin (podman requires a VM there).
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

type machineStats struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
	Default bool   `json:"Default"`
}

// Starts the default podman machine (VM) when it isn't running. Only relevant
// on darwin, where podman needs a VM; a no-op elsewhere.
func ensureMachineRunning() error {
	raw, err := exec.Command("podman", "machine", "list", "--format", "json").Output()
	if err != nil {
		return err
	}
	var machines []machineStats
	if err := json.Unmarshal(raw, &machines); err != nil {
		return fmt.Errorf("parsing machine list: %w", err)
	}
	if len(machines) == 0 {
		return fmt.Errorf("no podman machine initialized")
	}
	machine := machines[0]
	for _, m := range machines {
		if m.Default {
			machine = m
			break
		}
	}
	if machine.Running {
		return nil
	}
	log.Printf("No running machine found, starting %q...", machine.Name)
	cmd := exec.Command("podman", "machine", "start", machine.Name)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
