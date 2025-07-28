package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Manager struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	logFile   *os.File
	startTime time.Time
}

type StatusResponse struct {
	Running bool      `json:"running"`
	PID     int       `json:"pid,omitempty"`
	Uptime  string    `json:"uptime,omitempty"`
	Started time.Time `json:"started,omitempty"`
}

// PrefixedWriter wraps an io.Writer to add a prefix to each line
type PrefixedWriter struct {
	writer io.Writer
	prefix string
}

func NewPrefixedWriter(w io.Writer, prefix string) *PrefixedWriter {
	return &PrefixedWriter{
		writer: w,
		prefix: prefix,
	}
}

func (pw *PrefixedWriter) Write(p []byte) (n int, err error) {
	// Convert to string and add prefix to each line
	lines := string(p)
	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" { // Skip empty lines
			prefixedLine := fmt.Sprintf("%s %s\n", pw.prefix, line)
			if _, err := pw.writer.Write([]byte(prefixedLine)); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return fmt.Errorf("process already running")
	}

	// Create log file for cc-slack output
	logFileName := fmt.Sprintf("logs/cc-slack-managed-%s.log", time.Now().Format("20060102-150405"))
	log.Printf("📝 Creating log file: %s", logFileName)

	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	m.logFile = logFile

	log.Println("🏗️ Starting cc-slack process...")

	// Get the directory where the manager is running from
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Determine project root directory
	// If running with 'go run', execPath will be in a temp directory
	// So we need to use the current working directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// If we're in scripts directory, go up one level
	if strings.HasSuffix(projectRoot, "/scripts") {
		projectRoot = filepath.Dir(projectRoot)
	}

	log.Printf("📁 Project root: %s", projectRoot)
	log.Printf("📁 Executable path: %s", execPath)

	cmd := exec.CommandContext(m.ctx, "./cc-slack")
	cmd.Dir = projectRoot // Set working directory to project root

	// Create prefixed writers for console output
	stdoutConsole := NewPrefixedWriter(os.Stdout, "[cc-slack stdout]")
	stderrConsole := NewPrefixedWriter(os.Stderr, "[cc-slack stderr]")

	// Use MultiWriter to output to both log file and console
	cmd.Stdout = io.MultiWriter(logFile, stdoutConsole)
	cmd.Stderr = io.MultiWriter(logFile, stderrConsole)
	cmd.Env = os.Environ()

	// Set process group so we can kill the entire group later
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	m.cmd = cmd
	m.startTime = time.Now()
	log.Printf("✅ Started cc-slack (PID: %d)", cmd.Process.Pid)
	log.Printf("📁 Output log: %s", logFileName)

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.cmd = nil
		if m.logFile != nil {
			m.logFile.Close()
			m.logFile = nil
		}
		m.mu.Unlock()

		if err != nil {
			log.Printf("❌ cc-slack exited with error: %v", err)
		} else {
			log.Printf("🛑 cc-slack exited normally")
		}
	}()

	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return fmt.Errorf("no process running")
	}

	log.Printf("🔄 Stopping cc-slack (PID: %d)...", m.cmd.Process.Pid)

	// Try graceful shutdown first
	// Note: When using 'go run', we need to send signal to the process group
	pgid, err := syscall.Getpgid(m.cmd.Process.Pid)
	if err == nil {
		// Send signal to the entire process group
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			log.Printf("⚠️ Failed to send SIGTERM to process group: %v", err)
			// Fall back to sending to the process directly
			if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("failed to send SIGTERM: %w", err)
			}
		}
	} else {
		// Fall back to sending to the process directly
		if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to send SIGTERM: %w", err)
		}
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		_, err := m.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
		log.Println("✅ cc-slack stopped gracefully")
		m.cmd = nil
		if m.logFile != nil {
			m.logFile.Close()
			m.logFile = nil
		}
		return nil
	case <-time.After(10 * time.Second):
		log.Println("⚠️ Graceful shutdown timeout, force killing...")
		if err := m.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		m.cmd = nil
		if m.logFile != nil {
			m.logFile.Close()
			m.logFile = nil
		}
		return nil
	}
}

func (m *Manager) Restart() error {
	log.Println("🔄 Restarting cc-slack...")

	// Stop existing process if running
	if m.cmd != nil && m.cmd.Process != nil {
		oldPID := m.cmd.Process.Pid
		log.Printf("📋 Stopping existing process (PID: %d)...", oldPID)

		if err := m.Stop(); err != nil {
			log.Printf("⚠️ Error stopping process: %v", err)
			// Continue with restart anyway
		} else {
			log.Printf("✅ Process %d stopped successfully", oldPID)
		}

		// Brief pause to ensure clean shutdown
		time.Sleep(1 * time.Second)
	}

	// Start new process
	log.Println("🚀 Starting new cc-slack process...")
	if err := m.Start(); err != nil {
		log.Printf("❌ Failed to start new process: %v", err)
		return fmt.Errorf("restart failed: %w", err)
	}

	log.Printf("✅ Restart completed! New process PID: %d", m.cmd.Process.Pid)
	return nil
}

func (m *Manager) Status() StatusResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return StatusResponse{Running: false}
	}

	// Check if process is still alive
	if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return StatusResponse{Running: false}
	}

	return StatusResponse{
		Running: true,
		PID:     m.cmd.Process.Pid,
		Started: m.startTime,
		Uptime:  time.Since(m.startTime).String(),
	}
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	// Ensure logs directory exists
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	manager := NewManager()

	// Start cc-slack on launch
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start cc-slack: %v", err)
	}

	// HTTP handlers
	http.HandleFunc("/status", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := manager.Status()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}))

	http.HandleFunc("/restart", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("📥 Received restart request")

		// Execute restart synchronously
		startTime := time.Now()
		err := manager.Restart()
		duration := time.Since(startTime)

		// Prepare response
		response := make(map[string]interface{})
		response["duration"] = duration.String()

		if err != nil {
			log.Printf("❌ Restart failed: %v", err)
			response["success"] = false
			response["error"] = err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Printf("✅ Restart completed successfully in %v", duration)
			response["success"] = true
			response["pid"] = manager.Status().PID
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("📥 Received stop request")

		// Execute stop synchronously
		err := manager.Stop()

		response := make(map[string]interface{})
		if err != nil {
			log.Printf("❌ Stop failed: %v", err)
			response["success"] = false
			response["error"] = err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Println("✅ cc-slack stopped successfully")
			response["success"] = true
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("📥 Received start request")

		// Execute start synchronously
		err := manager.Start()

		response := make(map[string]interface{})
		if err != nil {
			log.Printf("❌ Start failed: %v", err)
			response["success"] = false
			response["error"] = err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Println("✅ cc-slack started successfully")
			response["success"] = true
			response["pid"] = manager.Status().PID
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Received shutdown signal, stopping cc-slack...")
		manager.Stop()
		manager.cancel()
		os.Exit(0)
	}()

	log.Println("🌐 cc-slack manager listening on :10080")
	log.Println("📌 Available endpoints:")
	log.Println("  GET  /status   - Check process status")
	log.Println("  POST /restart  - Restart cc-slack")
	log.Println("  POST /stop     - Stop cc-slack")
	log.Println("  POST /start    - Start cc-slack")

	if err := http.ListenAndServe(":10080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
