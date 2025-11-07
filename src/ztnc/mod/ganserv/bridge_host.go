// File: src/plugins/ztnc/mod/ganserv/bridge_host.go
package ganserv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Candidates in order of preference:
// 1) ZT_BRIDGE_SCRIPT env override
// 2) Staged path inside plugin tree (mod/utils/zt-bridge-setup.sh)
// 3) System install paths (/usr/local/sbin, /usr/sbin)
func findBridgeScript() (string, error) {
	// 1) env override
	if p := os.Getenv("ZT_BRIDGE_SCRIPT"); p != "" {
		if isFile(p) {
			return p, nil
		}
	}

	// 2) staged in repo: ./mod/utils/zt-bridge-setup.sh (try from CWD and from exe dir)
	staged := filepath.Join("mod", "utils", "zt-bridge-setup.sh")
	if isFile(staged) {
		return staged, nil
	}
	if exeDir := exeDir(); exeDir != "" {
		if p := filepath.Join(exeDir, "mod", "utils", "zt-bridge-setup.sh"); isFile(p) {
			return p, nil
		}
	}

	// 3) system install
	sysCandidates := []string{
		"/usr/local/sbin/zt-bridge-setup.sh",
		"/usr/sbin/zt-bridge-setup.sh",
	}
	for _, p := range sysCandidates {
		if isFile(p) {
			return p, nil
		}
	}

	return "", errors.New("zt-bridge-setup.sh not found; stage it under mod/utils or install to /usr/local/sbin; or set ZT_BRIDGE_SCRIPT")
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ""
	}
	return filepath.Dir(exe)
}

// InvokeHostBridge runs the bridge script with the expected args.
// It will happily use the staged script in mod/utils if you haven’t promoted it to /usr/local/sbin yet.
func InvokeHostBridge(netid, uplink, ztIfHint string, hostOnly bool) error {
	script, err := findBridgeScript()
	if err != nil {
		return err
	}
	if uplink == "" {
		uplink = "eth0"
	}
	args := []string{netid, uplink}
	if ztIfHint != "" {
		args = append(args, ztIfHint)
	}
	if hostOnly {
		args = append(args, "--host-only")
	}

	cmd := exec.Command(script, args...)
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Dir(script)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bridge script failed: %w: %s", err, string(out))
	}
	return nil
}

// InstallHostBridgeAssets copies the staged script/unit from mod/utils into system paths.
// Run as root once, then the systemd service can persist the bridge across reboots.
func InstallHostBridgeAssets() error {
	if os.Geteuid() != 0 {
		return errors.New("InstallHostBridgeAssets requires root")
	}

	// locate staged script
	srcScript := firstExisting(
		filepath.Join("mod", "utils", "zt-bridge-setup.sh"),
		filepath.Join(exeDir(), "mod", "utils", "zt-bridge-setup.sh"),
	)
	if srcScript == "" {
		return errors.New("staged zt-bridge-setup.sh not found in mod/utils")
	}
	if err := copyWithMode(srcScript, "/usr/local/sbin/zt-bridge-setup.sh", 0o755); err != nil {
		return fmt.Errorf("install script: %w", err)
	}

	// locate staged unit
	srcUnit := firstExisting(
		filepath.Join("mod", "utils", "zt-bridge@.service"),
		filepath.Join(exeDir(), "mod", "utils", "zt-bridge@.service"),
	)
	if srcUnit == "" {
		return errors.New("staged zt-bridge@.service not found in mod/utils")
	}
	if err := copyWithMode(srcUnit, "/etc/systemd/system/zt-bridge@.service", 0o644); err != nil {
		return fmt.Errorf("install unit: %w", err)
	}

	// reload systemd if present
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

// EnableBridgeService enables and starts the templated bridge unit for a given network.
// Note: the unit you staged should pass %i (netid) and uplink to the script (see your ExecStart line).
func EnableBridgeService(netid string) error {
	if os.Geteuid() != 0 {
		return errors.New("EnableBridgeService requires root")
	}
	if netid == "" {
		return errors.New("netid is required")
	}
	out, err := exec.Command("systemctl", "enable", "--now", "zt-bridge@"+netid+".service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable/start failed: %w: %s", err, string(out))
	}
	return nil
}

// helpers

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if isFile(p) {
			return p
		}
	}
	return ""
}

func copyWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}
