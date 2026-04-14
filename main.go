package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	// handle control args before flag.Parse()
	for _, a := range os.Args[1:] {
		if a == "--status" || a == "-status" {
			showStatus()
			return
		}
		if a == "--stop" || a == "-stop" {
			stopDaemon()
			return
		}
	}

	consoleLog := flag.Bool("log-console", true, "log requests to console")
	fileLog := flag.Bool("log-file", true, "log requests to file (PROXY_LOG_FILE)")
	daemon := flag.Bool("daemon", false, "run in background")
	flag.Bool("foreground", false, "suppress daemon fork (internal)")
	flag.Parse()

	if *daemon {
		daemonize()
		return
	}

	cfg := LoadConfig()

	// CLI flag overrides
	if !*consoleLog {
		cfg.ConsoleLogOn = false
	}
	if !*fileLog {
		cfg.LogEnabled = false
	}

	if cfg.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY required")
	}

	proxy := NewProxy(cfg)

	log.Printf("proxy listening on %s → %s/v1/chat/completions", cfg.BindAddr, cfg.OpenAIAPIURL)
	log.Printf("console log: %v | file log: %v (%s)", cfg.ConsoleLogOn, cfg.LogEnabled, cfg.LogFile)
	log.Fatal(http.ListenAndServe(cfg.BindAddr, proxy))
}

func daemonize() {
	// check if already running
	if isRunning() {
		fmt.Println("already running (use --stop first)")
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	exe, _ = filepath.Abs(exe)

	logFile := filepath.Join(".", "log", "daemon.log")
	os.MkdirAll(filepath.Dir(logFile), 0755)
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log: %v\n", err)
		os.Exit(1)
	}

	// rebuild args: remove --daemon, add --foreground
	args := append([]string{}, os.Args[1:]...)
	args = filterArgs(args, "--daemon")
	args = append(args, "--foreground")

	cmd := exec.Command(exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(1)
	}

	os.WriteFile(".daemon.pid", []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	fmt.Printf("started (pid %d, log: %s)\n", cmd.Process.Pid, logFile)
	os.Exit(0)
}

func isRunning() bool {
	data, err := os.ReadFile(".daemon.pid")
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func showStatus() {
	data, err := os.ReadFile(".daemon.pid")
	if err != nil {
		fmt.Println("not running")
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("not running (bad pidfile)")
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		fmt.Println("dead (stale pidfile)")
		os.Remove(".daemon.pid")
		return
	}
	fmt.Printf("running (pid %d)\n", pid)
}

func stopDaemon() {
	data, err := os.ReadFile(".daemon.pid")
	if err != nil {
		fmt.Println("not running")
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("not running (bad pidfile)")
		os.Remove(".daemon.pid")
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("not running")
		return
	}
	if err := proc.Kill(); err != nil {
		fmt.Printf("kill failed: %v\n", err)
	}
	os.Remove(".daemon.pid")
	fmt.Printf("stopped (pid %d)\n", pid)
}

func filterArgs(args []string, remove string) []string {
	var out []string
	for _, a := range args {
		if a != remove {
			out = append(out, a)
		}
	}
	return out
}
