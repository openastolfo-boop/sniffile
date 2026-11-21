// Command sniffile is a small local file search that runs as a desktop app.
//
//	sniffile                 # open the native desktop window
//	sniffile -server         # serve the same UI in a browser
//	sniffile scan ~/Pictures # index from the terminal, no window
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/openastolfo-boop/sniffile/internal/config"
	"github.com/openastolfo-boop/sniffile/internal/server"
)

var version = "0.3.0"

func main() {
	log.SetFlags(log.Ltime)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "scan":
			os.Exit(runScan(os.Args[2:]))
		case "version", "-version", "--version":
			fmt.Println("sniffile " + version)
			return
		case "help", "-h", "--help":
			usage()
			return
		}
	}
	runServer()
}

func runServer() {
	fs := flag.NewFlagSet("sniffile", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8765", "listen address (desktop mode picks a free port unless set)")
	serverMode := fs.Bool("server", false, "run as a local web server and open the browser instead of a desktop window")
	dataDir := fs.String("data", "", "data folder (default: user config dir)")
	ollamaURL := fs.String("ollama", "auto", `ollama base url, "auto" to detect, "off" to disable`)
	openBrowser := fs.Bool("open", true, "open the browser on start (server mode)")
	maxFileMB := fs.Int64("max-file-mb", 0, "skip files bigger than this (0 = saved value, default 64)")
	noContent := fs.Bool("no-content", false, "index names only, do not read text contents")
	verbose := fs.Bool("v", false, "log every request")
	fs.Usage = func() { usage() }
	_ = fs.Parse(os.Args[1:])

	addrSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})

	dir, err := resolveDataDir(*dataDir)
	if err != nil {
		log.Fatalf("data dir: %v", err)
	}
	cfgPath := filepath.Join(dir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("read %s: %v", cfgPath, err)
	}

	switch *ollamaURL {
	case "auto":
		if cfg.OllamaURL == "" && ollamaReachable("http://127.0.0.1:11434") {
			cfg.OllamaURL = "http://127.0.0.1:11434"
			_ = cfg.Save(cfgPath)
		}
	case "off", "":
		cfg.OllamaURL = ""
		_ = cfg.Save(cfgPath)
	default:
		cfg.OllamaURL = *ollamaURL
		_ = cfg.Save(cfgPath)
	}

	if *maxFileMB > 0 {
		cfg.MaxFileMB = *maxFileMB
		_ = cfg.Save(cfgPath)
	}
	if *noContent {
		cfg.NoContent = true
		_ = cfg.Save(cfgPath)
	}

	m := server.NewManager(version, dir, cfgPath, cfg)
	handler := server.NewHandler(m)
	if *verbose {
		handler = logRequests(handler)
	}

	useDesktop := !*serverMode && desktopAvailable()
	listenAddr := *addr
	if useDesktop && !addrSet {
		listenAddr = "127.0.0.1:0" // let the OS pick a free port
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	if !isLoopback(host) {
		log.Printf("warning: listening on %s — anyone on your network can browse these files", host)
	}
	url := "http://" + net.JoinHostPort(host, port)

	fmt.Printf("sniffile %s\n", version)
	if useDesktop {
		fmt.Printf("  window native\n")
	} else {
		fmt.Printf("  open   %s\n", url)
	}
	fmt.Printf("  data   %s\n", dir)
	if cfg.OllamaURL != "" {
		fmt.Printf("  ollama %s\n", cfg.OllamaURL)
	} else {
		fmt.Printf("  ollama off (plain index only)\n")
	}
	fmt.Printf("  ctrl-c to stop\n\n")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		terminateWindow()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	if useDesktop {
		if err := runWindow(url); err != nil {
			log.Printf("desktop window: %v", err)
			log.Printf("falling back to the browser at %s", url)
			if *openBrowser {
				openInBrowser(url)
			}
			<-ctx.Done()
		}
	} else {
		if *openBrowser {
			openInBrowser(url)
		}
		<-ctx.Done()
	}

	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = srv.Shutdown(shutdown)
	cancel()
	fmt.Println("bye.")
}

// runScan is the cron-friendly path: index folders, print progress, exit.
func runScan(args []string) int {
	fs := flag.NewFlagSet("sniffile scan", flag.ExitOnError)
	describe := fs.Bool("describe", false, "also caption images with ollama")
	dataFlag := fs.String("data", "", "data folder (default: user config dir)")
	_ = fs.Parse(args)

	dir, err := resolveDataDir(*dataFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "data dir:", err)
		return 1
	}
	cfgPath := filepath.Join(dir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}

	var roots []string
	for _, p := range fs.Args() {
		clean, err := config.CleanRoot(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skip:", err)
			continue
		}
		if _, _, err := cfg.AddRoot(clean); err == nil {
			roots = append(roots, clean)
		}
	}
	if len(roots) == 0 {
		for _, r := range cfg.Roots {
			roots = append(roots, r.Path)
		}
	}
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to scan — pass a folder: sniffile scan ~/Pictures")
		return 1
	}
	if cfg.OllamaURL != "" || *describe {
		if cfg.OllamaURL == "" && ollamaReachable("http://127.0.0.1:11434") {
			cfg.OllamaURL = "http://127.0.0.1:11434"
		}
	}
	if err := cfg.Save(cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}

	m := server.NewManager(version, dir, cfgPath, cfg)
	if err := m.StartScan(roots, *describe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	lastLine := 0
	for {
		st := m.State()
		if !st.Job.Running {
			if st.Job.Error != "" {
				fmt.Fprintln(os.Stderr, "warning:", st.Job.Error)
			}
			fmt.Printf("indexed %s files in %d folder(s).\n", commas(st.Total), len(st.Roots))
			return 0
		}
		line := fmt.Sprintf("\r%s · %s files · %s", st.Job.Phase, commas(st.Job.Files), st.Job.Path)
		if len(line) > lastLine {
			lastLine = len(line)
		}
		fmt.Print(line + strings.Repeat(" ", max(0, lastLine-len(line))))
		time.Sleep(400 * time.Millisecond)
	}
}

func resolveDataDir(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	return config.DefaultDataDir()
}

func ollamaReachable(base string) bool {
	client := http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start() // headless machines just ignore this
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}

func commas(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func usage() {
	fmt.Print(`sniffile — local file search, black and white, no cloud

usage:
  sniffile                 open the desktop window
  sniffile -server         run the local web UI in a browser
  sniffile scan [folders]  index folders from the terminal
  sniffile version         print the version

desktop / server flags:
  -server run as a local web server instead of a desktop window
  -addr   listen address (default 127.0.0.1:8765; desktop picks a free port)
  -data   data folder (default ~/.config/sniffile)
  -ollama "auto" | "off" | http://127.0.0.1:11434
  -open   open the browser on start (default true, server mode only)
  -v      log every request

scan flags:
  -describe   caption images with a local ollama vision model

Everything stays on this machine. Source: MIT.
`)
}
