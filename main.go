package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

var (
	wwwDir        string
	uploadsDir    string
	maxUploadSize int64
	host          string
	port          string
	pollInterval  = 1 * time.Second
	useCurrentDir bool
)

const helpBanner = `
 .d88888b.                            .d8888b.  888
d88P" "Y88b                          d88P  Y88b 888
888     888                          Y88b.      888
888     888 888  888  .d88b.  888d888 "Y888b.   88888b.   8888b.  888d888 .d88b.
888     888 888  888 d8P  Y8b 888P"      "Y88b. 888 "88b     "88b 888P"  d8P  Y8b
888     888 Y88  88P 88888888 888          "888 888  888 .d888888 888    88888888
Y88b. .d88P  Y8bd8P  Y8b.     888    Y88b  d88P 888  888 888  888 888    Y8b.
 "Y88888P"    Y88P    "Y8888  888     "Y8888P"  888  888 "Y888888 888     "Y8888

                Simple GoLang File Server made by casper AKA VexilonHacker
`

func printBanner() {
	fmt.Printf("%s%s%s\n", colorCyan, helpBanner, colorReset)
}

func printHelp() {
	fmt.Printf("%sUsage:%s\n", colorCyan, colorReset)
	fmt.Printf("  ./server [options]\n\n")
	fmt.Printf("%sOptions:%s\n", colorCyan, colorReset)
	fmt.Printf("  %s--host%s     %sHost to bind (default 0.0.0.0)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--port%s     %sPort to listen on (default 8080)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--www%s      %sDirectory to serve static files (default 'www')%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--uploads%s  %sDirectory to store uploads (default uses current directory if not specified)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--maxmb%s    %sMaximum upload size in MB (default 200)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--help%s     %sShow this help menu%s\n", colorYellow, colorReset, colorGreen, colorReset)
}

type Broker struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: make(map[chan string]struct{})}
}

func (b *Broker) AddClient(ch chan string) {
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
}

func (b *Broker) RemoveClient(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broker) Publish(msg string) {
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
	b.mu.Unlock()
}

var broker = NewBroker()

func sanitizeFileName(filename string) string {
	name := path.Base(filename)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func ensureDirs() {
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		if err := os.MkdirAll(wwwDir, 0755); err != nil {
			log.Fatalf("Failed to create %s: %v", wwwDir, err)
		}
		fmt.Printf("%s[init]%s Created missing directory: %s\n", colorCyan, colorReset, wwwDir)
	}

	if !useCurrentDir {
		if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(uploadsDir, 0755); err != nil {
				log.Fatalf("Failed to create %s: %v", uploadsDir, err)
			}
			fmt.Printf("%s[init]%s Created missing directory: %s\n", colorCyan, colorReset, uploadsDir)
		}
	} else {
		fmt.Printf("%s[init]%s Using current directory for uploads: %s\n", colorCyan, colorReset, uploadsDir)
	}
}

func uniqueFileName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	target := filepath.Join(uploadsDir, name)
	i := 1
	for {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		newName := fmt.Sprintf("%s_(%d)%s", base, i, ext)
		target = filepath.Join(uploadsDir, newName)
		i++
	}
	return target
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	remote := r.RemoteAddr
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	var savedFile string
	var totalBytes int64

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading multipart data", http.StatusInternalServerError)
			return
		}
		if part.FormName() != "file" {
			continue
		}

		origFilename := sanitizeFileName(part.FileName())
		if origFilename == "" {
			part.Close()
			continue
		}
		finalPath := uniqueFileName(origFilename)
		tmpPath := finalPath + ".tmp"

		fmt.Printf("%s[UPLOAD START]%s %s <- %s\n", colorBlue, colorReset, filepath.Base(finalPath), remote)
		if b, jerr := json.Marshal(map[string]string{"type": "start", "file": filepath.Base(finalPath)}); jerr == nil {
			broker.Publish(string(b))
		}

		out, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, "Unable to create file", http.StatusInternalServerError)
			return
		}

		written, err := io.Copy(out, part)
		out.Close()
		part.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		if err := os.Rename(tmpPath, finalPath); err != nil {
			_ = os.Remove(tmpPath)
			http.Error(w, "Failed to finalize file", http.StatusInternalServerError)
			return
		}

		savedFile = filepath.Base(finalPath)
		totalBytes = written

		if b, jerr := json.Marshal(map[string]string{"type": "new", "file": savedFile}); jerr == nil {
			broker.Publish(string(b))
		}

		fmt.Printf("%s[UPLOAD]%s %s (%d bytes) complete in %v\n", colorGreen, colorReset, savedFile, written, time.Since(start))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"file":   savedFile,
		"bytes":  fmt.Sprintf("%d", totalBytes),
	})
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(uploadsDir)
	if err != nil {
		http.Error(w, "Cannot read uploads dir", http.StatusInternalServerError)
		return
	}

	type fileInfo struct {
		Name string
		Mod  int64
	}

	var infos []fileInfo
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tmp") {
			continue
		}
		if fi, err := f.Info(); err == nil && !fi.IsDir() {
			infos = append(infos, fileInfo{f.Name(), fi.ModTime().Unix()})
		}
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Mod > infos[j].Mod })

	var names []string
	for _, f := range infos {
		names = append(names, f.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(names)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}
	filename = sanitizeFileName(filename)
	filePath := filepath.Join(uploadsDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	fmt.Printf("%s[DOWNLOAD]%s %s\n", colorYellow, colorReset, filename)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	http.ServeFile(w, r, filePath)
}

func maxSizeHandler(w http.ResponseWriter, r *http.Request) {
	sizeMB := maxUploadSize >> 20
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{"maxUploadMB": sizeMB})
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan string, 8)
	broker.AddClient(clientChan)
	defer broker.RemoveClient(clientChan)

	if b, err := json.Marshal(map[string]string{"type": "hello"}); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func logRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		elapsed := time.Since(start)

		methodColor := colorCyan
		switch r.Method {
		case "POST":
			methodColor = colorGreen
		case "GET":
			methodColor = colorBlue
		case "DELETE":
			methodColor = colorRed
		}

		fmt.Printf("%s[%s]%s %s %s %v\n", methodColor, r.Method, colorReset, r.RemoteAddr, r.URL.Path, elapsed)
	})
}

func watchUploadsPolling(dir string, interval time.Duration) {
	snapshot := make(map[string]int64)

	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if f.IsDir() || strings.HasSuffix(f.Name(), ".tmp") {
			continue
		}
		if fi, err := f.Info(); err == nil {
			snapshot[f.Name()] = fi.ModTime().Unix()
		}
	}

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			current := make(map[string]int64)
			files, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() || strings.HasSuffix(f.Name(), ".tmp") {
					continue
				}
				if fi, err := f.Info(); err == nil {
					current[f.Name()] = fi.ModTime().Unix()
				}
			}

			for name := range current {
				if _, ok := snapshot[name]; !ok {
					if b, jerr := json.Marshal(map[string]string{"type": "new", "file": name}); jerr == nil {
						broker.Publish(string(b))
					}
					fmt.Printf("%s[FS NEW]%s %s\n", colorGreen, colorReset, name)
				}
			}
			for name := range snapshot {
				if _, ok := current[name]; !ok {
					if b, jerr := json.Marshal(map[string]string{"type": "remove", "file": name}); jerr == nil {
						broker.Publish(string(b))
					}
					fmt.Printf("%s[FS REM]%s %s\n", colorYellow, colorReset, name)
				}
			}
			snapshot = current
		}
	}()
}

func getPrivateIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func getBinaryDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	real, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

func findWwwDir() string {
	binaryDir := getBinaryDir()
	binaryWww := filepath.Join(binaryDir, "www")
	if _, err := os.Stat(binaryWww); err == nil {
		fmt.Printf("%s[WWW]%s Found www next to binary: %s\n", colorCyan, colorReset, binaryWww)
		return binaryWww
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		localWww := filepath.Join(homeDir, ".local", "share", "overshare", "www")
		if _, err := os.Stat(localWww); err == nil {
			fmt.Printf("%s[WWW]%s Found www in user share: %s\n", colorCyan, colorReset, localWww)
			return localWww
		}
	}
	fmt.Printf("%s[WWW]%s Using default www in current directory\n", colorYellow, colorReset)
	return "www"
}

func main() {
	printBanner()
	var help bool
	flag.StringVar(&host, "host", "0.0.0.0", "Host to bind (default 0.0.0.0)")
	flag.StringVar(&port, "port", "8080", "Port to listen on")
	maxMB := flag.Int64("maxmb", 200, "Maximum upload size in MB")
	flag.StringVar(&wwwDir, "www", "www", "Directory to serve static files")
	flag.StringVar(&uploadsDir, "uploads", "", "Directory to store uploads (default uses current directory if not specified)")
	flag.BoolVar(&help, "help", false, "Show help menu")

	flag.Usage = func() {
		printHelp()
	}

	flag.Parse()

	if help {
		printHelp()
		return
	}

	if wwwDir == "www" {
		wwwDir = findWwwDir()
	}

	if uploadsDir == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current directory: %v", err)
		}
		uploadsDir = currentDir
		useCurrentDir = true
	} else {
		useCurrentDir = false
	}

	maxUploadSize = *maxMB << 20

	if p, err := strconv.Atoi(port); err != nil || p <= 0 || p > 65535 {
		log.Fatalf("Invalid port: %s", port)
	}
	if maxUploadSize <= 0 {
		log.Fatalf("Invalid maxmb: %d", *maxMB)
	}

	ensureDirs()
	watchUploadsPolling(uploadsDir, pollInterval)

	http.Handle("/", logRequest(http.FileServer(http.Dir(wwwDir))))
	http.Handle("/uploads/", logRequest(http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir)))))
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/files", filesHandler)
	http.HandleFunc("/download/", downloadHandler)
	http.HandleFunc("/maxsize", maxSizeHandler)
	http.HandleFunc("/events", eventsHandler)

	listenAddr := host + ":" + port

	uploadDisplay := uploadsDir
	if useCurrentDir {
		uploadDisplay = "current directory (" + uploadsDir + ")"
	}

	fmt.Printf("%s[SERVER]%s Listening on %s | Max upload: %d MB\n", colorCyan, colorReset, listenAddr, *maxMB)
	fmt.Printf("%s[UPLOADS]%s Using: %s\n", colorCyan, colorReset, uploadDisplay)

	fmt.Printf("\n%s[ACCESS URLs]%s\n", colorCyan, colorReset)
	fmt.Printf("  %s• Local:%s http://localhost:%s/\n", colorGreen, colorReset, port)

	privateIPs := getPrivateIPs()
	if len(privateIPs) > 0 {
		for _, ip := range privateIPs {
			fmt.Printf("  %s• Private:%s http://%s:%s/\n", colorGreen, colorReset, ip, port)
		}
	}

	if host != "0.0.0.0" && host != "::" {
		fmt.Printf("  %s• Custom:%s http://%s:%s/\n", colorGreen, colorReset, host, port)
	}
	fmt.Println()

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
