package assets

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/beck-8/subs-check/config"
	"github.com/beck-8/subs-check/save/method"
	"github.com/klauspost/compress/zstd"
	"github.com/shirou/gopsutil/v4/process"
	"gopkg.in/natefinch/lumberjack.v2"
)

func RunSubStoreService() {
	for {
		if err := startSubStore(); err != nil {
			slog.Error("Sub-store service crashed, restarting...", "error", err)
		}
		time.Sleep(time.Second * 30)
	}
}

func startSubStore() error {
	saver, err := method.NewLocalSaver()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(saver.OutputPath) {
		// Handle user-provided relative paths.
		saver.OutputPath = filepath.Join(saver.BasePath, saver.OutputPath)
	}
	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName += ".exe"
	}

	os.MkdirAll(saver.OutputPath, 0755)
	nodePath := filepath.Join(saver.OutputPath, nodeName)
	jsPath := filepath.Join(saver.OutputPath, "sub-store.bundle.js")
	overYamlPath := filepath.Join(saver.OutputPath, "ACL4SSR_Online_Full.yaml")
	logPath := filepath.Join(saver.OutputPath, "sub-store.log")

	killNode := func() {
		pid, err := findProcesses(nodePath)
		if err == nil {
			err := killProcess(pid)
			if err != nil {
				slog.Debug("Sub-store service kill failed", "error", err)
			}
			slog.Debug("Sub-store service already killed", "pid", pid)
		}
	}
	defer killNode()

	// If subs-check exits due to memory pressure, the node binary can be
	// corrupted and leave a stale node process, so remove it first.
	os.Remove(nodePath)
	os.Remove(jsPath)
	os.Remove(overYamlPath)
	if err := decodeZstd(nodePath, jsPath, overYamlPath); err != nil {
		return err
	}

	// Configure log rotation.
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10, // Maximum 10MB per log file.
		MaxBackups: 3,  // Keep 3 old files.
		MaxAge:     7,  // Keep files for 7 days.
	}
	defer logWriter.Close()

	// Support a custom node binary path for more devices.
	if nodeBinPath := os.Getenv("NODEBIN_PATH"); nodeBinPath != "" {
		nodePath = nodeBinPath
	}
	// Support a custom sub-store script path.
	if subStoreBinPath := os.Getenv("SUB_STORE_PATH"); subStoreBinPath != "" {
		jsPath = subStoreBinPath
	}
	// Run the JavaScript file.
	cmd := exec.Command(nodePath, jsPath)
	// The JS releases dependency files in the working directory.
	cmd.Dir = saver.OutputPath
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.Env = os.Environ()

	// If MihomoOverwriteUrl contains a local IP, remove proxy environment variables.
	cleanProxyEnv := false
	if config.GlobalConfig.MihomoOverwriteUrl != "" {
		parsedURL, err := url.Parse(config.GlobalConfig.MihomoOverwriteUrl)
		if err == nil {
			host := parsedURL.Hostname()
			if isLocalIP(host) {
				cleanProxyEnv = true
				slog.Debug("MihomoOverwriteUrl contains local IP, removing proxy environment variables")
			}
		}
	}

	// IPv4 and IPv6 are both supported.
	hostPort := strings.Split(config.GlobalConfig.SubStorePort, ":")
	// host can be empty; port cannot.
	if len(hostPort) == 2 && hostPort[1] != "" {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("SUB_STORE_BACKEND_API_HOST=%s", hostPort[0]),
			fmt.Sprintf("SUB_STORE_BACKEND_API_PORT=%s", hostPort[1]),
		)
	} else if len(hostPort) == 1 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SUB_STORE_BACKEND_API_PORT=%s", hostPort[0])) // Set port.
	} else {
		return fmt.Errorf("sub-store-port invalid port format: %s", config.GlobalConfig.SubStorePort)
	}

	// https://hub.docker.com/r/xream/sub-store
	// There are detailed variable descriptions here. Filtering 127.0.0.1 via
	// NO_PROXY may be more appropriate. If MihomoOverwriteUrl contains a local
	// IP, remove all proxy environment variables.
	if cleanProxyEnv {
		filteredEnv := make([]string, 0, len(cmd.Env))
		proxyVars := []string{"http_proxy", "https_proxy", "all_proxy", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"}

		for _, env := range cmd.Env {
			isProxyVar := false
			for _, proxyVar := range proxyVars {
				if strings.HasPrefix(strings.ToLower(env), strings.ToLower(proxyVar)+"=") {
					isProxyVar = true
					break
				}
			}
			if !isProxyVar {
				filteredEnv = append(filteredEnv, env)
			}
		}
		cmd.Env = filteredEnv
	}

	// Increase the body limit. Default is 1M.
	if os.Getenv("SUB_STORE_BODY_JSON_LIMIT") == "" {
		cmd.Env = append(cmd.Env, "SUB_STORE_BODY_JSON_LIMIT=30mb")
	}
	// Add custom access path.
	if config.GlobalConfig.SubStorePath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SUB_STORE_FRONTEND_BACKEND_PATH=%s", config.GlobalConfig.SubStorePath))
		cmd.Env = append(cmd.Env, "SUB_STORE_BACKEND_MERGE=1")
	}

	// sub-store environment variable: backend uploads files to gist.
	if config.GlobalConfig.SubStoreSyncCron != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SUB_STORE_BACKEND_SYNC_CRON=%s", config.GlobalConfig.SubStoreSyncCron))
	}

	// sub-store environment variable: automatically fetch subscription content.
	if config.GlobalConfig.SubStoreProduceCron != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SUB_STORE_PRODUCE_CRON=%s", config.GlobalConfig.SubStoreProduceCron))
	}

	// sub-store environment variable: send notifications on errors.
	if config.GlobalConfig.SubStorePushService != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SUB_STORE_PUSH_SERVICE=%s", config.GlobalConfig.SubStorePushService))
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sub-store: %w", err)
	}

	slog.Info("Sub-store service started", "pid", cmd.Process.Pid, "port", config.GlobalConfig.SubStorePort, "log", logPath)

	// Wait for the process to exit.
	return cmd.Wait()
}

// isLocalIP checks whether an IP is local (127.0.0.1 or a private LAN IP).
func isLocalIP(host string) bool {
	// Check whether it is localhost or 127.0.0.1.
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check whether the IP is valid.
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// Check whether it is in a private IP range.
	privateIPBlocks := []string{
		"10.0.0.0/8",     // 10.0.0.0 - 10.255.255.255
		"172.16.0.0/12",  // 172.16.0.0 - 172.31.255.255
		"192.168.0.0/16", // 192.168.0.0 - 192.168.255.255
		"169.254.0.0/16", // 169.254.0.0 - 169.254.255.255
		"fd00::/8",       // fd00:: - fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff
	}

	for _, block := range privateIPBlocks {
		_, ipNet, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

func decodeZstd(nodePath, jsPath, overYamlPath string) error {
	// Create zstd decoder.
	zstdDecoder, err := zstd.NewReader(nil)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer zstdDecoder.Close()

	// Decompress node binary.
	nodeFile, err := os.OpenFile(nodePath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to create node file: %w", err)
	}
	defer nodeFile.Close()

	zstdDecoder.Reset(bytes.NewReader(EmbeddedNode))
	if _, err := io.Copy(nodeFile, zstdDecoder); err != nil {
		return fmt.Errorf("failed to decompress node binary: %w", err)
	}

	// Decompress sub-store script.
	jsFile, err := os.OpenFile(jsPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create sub-store script file: %w", err)
	}
	defer jsFile.Close()

	zstdDecoder.Reset(bytes.NewReader(EmbeddedSubStore))
	if _, err := io.Copy(jsFile, zstdDecoder); err != nil {
		return fmt.Errorf("failed to decompress sub-store script: %w", err)
	}

	// Decompress override file.
	overYamlFile, err := os.OpenFile(overYamlPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create ACL4SSR_Online_Full.yaml file: %w", err)
	}
	defer overYamlFile.Close()

	zstdDecoder.Reset(bytes.NewReader(EmbeddedOverrideYaml))
	if _, err := io.Copy(overYamlFile, zstdDecoder); err != nil {
		return fmt.Errorf("failed to decompress ACL4SSR_Online_Full.yaml: %w", err)
	}
	return nil
}

func findProcesses(targetName string) (int32, error) {
	processes, err := process.Processes()
	if err != nil {
		return 0, fmt.Errorf("failed to get process list: %v", err)
	}

	for _, p := range processes {
		name, err := p.Exe()
		// if err != nil {
		// 	// slog.Debug("Failed to get process name", "error", err)
		// }
		if err == nil && name == targetName {
			return p.Pid, nil
		}
	}
	return 0, fmt.Errorf("process not found")
}

func killProcess(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process %d: %v", pid, err)
	}

	if err := p.Kill(); err != nil {
		return fmt.Errorf("failed to kill process %d: %v", pid, err)
	}
	return nil
}
