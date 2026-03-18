package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/pflag"
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
	wwwDir          string
	uploadsDir      string
	maxUploadSize   int64
	host            string
	port            string
	pollInterval    = 1 * time.Second
	useCurrentDir   bool
	oneshotMode     bool
	oneshotFile     string
	maxDownloads    int
	shutdownTimeout int
	authUsername    string
	authPassword    string
	logFilePath     string
	logFile         *os.File
	logMutex        sync.Mutex
)

type LogEntry struct {
	Timestamp    string   `json:"timestamp"`
	Level        string   `json:"level"`
	Event        string   `json:"event,omitempty"`
	Type         string   `json:"type,omitempty"`
	Status       string   `json:"status,omitempty"`
	Filename     string   `json:"filename,omitempty"`
	Size         int64    `json:"size,omitempty"`
	RemoteAddr   string   `json:"remote_addr,omitempty"`
	UserAgent    string   `json:"user_agent,omitempty"`
	Username     string   `json:"username,omitempty"`
	Method       string   `json:"method,omitempty"`
	Path         string   `json:"path,omitempty"`
	Duration     string   `json:"duration,omitempty"`
	Files        []string `json:"files,omitempty"`
	FileCount    int      `json:"file_count,omitempty"`
	Error        string   `json:"error,omitempty"`
	Message      string   `json:"message,omitempty"`
	URL          string   `json:"url,omitempty"`
	IPs          []string `json:"ips,omitempty"`
	Port         string   `json:"port,omitempty"`
	MaxUpload    int64    `json:"max_upload_mb,omitempty"`
	Downloads    int      `json:"downloads,omitempty"`
	MaxDownloads int      `json:"max_downloads,omitempty"`
	Remaining    int      `json:"remaining,omitempty"`
	Expired      bool     `json:"expired,omitempty"`
}

const helpBanner = `
 .d88888b.                            .d8888b.  888
d88P" "Y88b                          d88P  Y88b 888
888     888                          Y88b.      888
888     888 888  888  .d88b.  888d888 "Y888b.   88888b.   8888b.  888d888 .d88b.
888     888 888  888 d8P  Y8b 888P"      "Y88b. 888 "88b     "88b 888P"  d8P  Y8b
888     888 Y88  88P 88888888 888          "888 888  888 .d888888 888    88888888
Y88b. .d88P  Y8bd8P  Y8b.     888    Y88b  d88P 888  888 888  888 888    Y8b.
 "Y88888P"    Y88P    "Y8888  888     "Y8888P"  888  888 "Y888888 888     "Y8888

                Simple GoLang File Server made by VexilonHacker /ᐠ - ˕ -マ
`

func printBanner() {
	fmt.Printf("%s%s%s\n", colorCyan, helpBanner, colorReset)
}

func printHelp() {
	fmt.Printf("%sUsage:%s\n", colorCyan, colorReset)
	fmt.Printf("  %sovershare%s [options]\n", colorGreen, colorReset)
	fmt.Printf("  %sovershare%s --oneshot <file> [options] (One-shot transfer mode)\n\n", colorGreen, colorReset)
	fmt.Printf("%sOptions:%s\n", colorCyan, colorReset)
	fmt.Printf("  %s--host <ip>%s          %sHost to bind (default 0.0.0.0)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--port <n>%s           %sPort to listen on (default 8000)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--username <user>%s    %sUsername for HTTP authentication (default: disabled)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--password <pass>%s    %sPassword for HTTP authentication%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--www <dir>%s          %sDirectory to serve static files (default 'www')%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--uploads <dir>%s      %sDirectory to store uploads (default current dir)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--maxmb <n>%s          %sMaximum upload size in MB (default 200)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--log-file <path>%s    %sPath to JSON log file (all events logged)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--qr%s                 %sShow QR code for server URL%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--timeout <s>%s        %sExit after this many seconds (0 = disabled, default 0)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("\n%sOne-shot Mode Options:%s\n", colorCyan, colorReset)
	fmt.Printf("  %s--oneshot <file>%s     %sTransfer file with QR code (one-time use)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--max-downloads <n>%s  %sMax downloads before exit (default 1)%s\n", colorYellow, colorReset, colorGreen, colorReset)
	fmt.Printf("  %s--help%s               %sShow this help menu%s\n", colorYellow, colorReset, colorGreen, colorReset)
}

var broker = NewBroker()

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

func printQRCode(url string) {
	var qr *qrcode.QRCode
	var err error
	for version := 1; version <= 10; version++ {
		qr, err = qrcode.NewWithForcedVersion(url, version, qrcode.Medium)
		if err == nil {
			break
		}
	}
	if err != nil {
		qr, err = qrcode.New(url, qrcode.Medium)
		if err != nil {
			logMessage("error", "QR generation failed", map[string]interface{}{"error": err.Error()})
			fmt.Printf("%sURL: %s%s\n", colorCyan, url, colorReset)
			return
		}
	}
	qrString := qr.ToSmallString(false)
	lines := strings.Split(qrString, "\n")
	logMessage("info", "QR code generated", map[string]interface{}{"url": url})
	fmt.Printf("\n%s[QR CODE]%s\n\n", colorCyan, colorReset)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Printf("  %s%s%s\n", colorGreen, line, colorReset)
		}
	}
}

func sanitizeFileName(filename string) string {
	name := path.Base(filename)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func initLogFile() error {
	if logFilePath == "" {
		return nil
	}
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	logMessage("info", "Logging initialized", map[string]interface{}{"path": logFilePath})
	fmt.Printf("%s[LOG]%s Logging to: %s\n", colorCyan, colorReset, logFilePath)
	return nil
}

func logMessage(level string, event string, fields map[string]interface{}) {
	if logFile == nil {
		return
	}
	logMutex.Lock()
	defer logMutex.Unlock()
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Event:     event,
	}
	for k, v := range fields {
		switch k {
		case "type":
			entry.Type = v.(string)
		case "status":
			entry.Status = v.(string)
		case "filename":
			entry.Filename = v.(string)
		case "size":
			entry.Size = v.(int64)
		case "remote":
			entry.RemoteAddr = v.(string)
		case "user_agent":
			entry.UserAgent = v.(string)
		case "username":
			entry.Username = v.(string)
		case "method":
			entry.Method = v.(string)
		case "path":
			entry.Path = v.(string)
		case "duration":
			entry.Duration = v.(string)
		case "files":
			entry.Files = v.([]string)
		case "file_count":
			entry.FileCount = v.(int)
		case "error":
			entry.Error = v.(string)
		case "message":
			entry.Message = v.(string)
		case "url":
			entry.URL = v.(string)
		case "ips":
			entry.IPs = v.([]string)
		case "port":
			entry.Port = v.(string)
		case "max_upload":
			entry.MaxUpload = v.(int64)
		case "downloads":
			entry.Downloads = v.(int)
		case "max_downloads":
			entry.MaxDownloads = v.(int)
		case "remaining":
			entry.Remaining = v.(int)
		case "expired":
			entry.Expired = v.(bool)
		}
	}
	jsonEntry, _ := json.Marshal(entry)
	logFile.WriteString(string(jsonEntry) + "\n")
	logFile.Sync()
}

func ensureDirs() {
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		if err := os.MkdirAll(wwwDir, 0755); err != nil {
			log.Fatalf("Failed to create %s: %v", wwwDir, err)
		}
		logMessage("info", "Directory created", map[string]interface{}{"path": wwwDir})
		fmt.Printf("%s[init]%s Created missing directory: %s\n", colorCyan, colorReset, wwwDir)
	}
	if !useCurrentDir {
		if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(uploadsDir, 0755); err != nil {
				log.Fatalf("Failed to create %s: %v", uploadsDir, err)
			}
			logMessage("info", "Directory created", map[string]interface{}{"path": uploadsDir})
			fmt.Printf("%s[init]%s Created missing directory: %s\n", colorCyan, colorReset, uploadsDir)
		}
	} else {
		logMessage("info", "Using current directory for uploads", map[string]interface{}{"path": uploadsDir})
		fmt.Printf("%s[init]%s Using current directory for uploads: %s\n", colorCyan, colorReset, uploadsDir)
	}
}

func uniqueFileName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	for i := 0; i < 1000; i++ {
		var filename string
		if i == 0 {
			filename = name
		} else {
			filename = fmt.Sprintf("%s_(%d)%s", base, i, ext)
		}
		filePath := filepath.Join(uploadsDir, filename)
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			f.Close()
			return filePath
		}
		if !os.IsExist(err) {
			return filePath
		}
	}
	return filepath.Join(uploadsDir, fmt.Sprintf("%s_(%d)%s", base, time.Now().UnixNano(), ext))
}

func formatFileSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	case size < 1024*1024*1024:
		return fmt.Sprintf("%.2f MB", float64(size)/1024/1024)
	default:
		return fmt.Sprintf("%.2f GB", float64(size)/1024/1024/1024)
	}
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
		logMessage("error", "Invalid multipart form", map[string]interface{}{"error": err.Error(), "remote": remote})
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
			logMessage("error", "Error reading multipart data", map[string]interface{}{"error": err.Error(), "remote": remote})
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
		logMessage("info", "Upload started", map[string]interface{}{"filename": filepath.Base(finalPath), "remote": remote})
		fmt.Printf("%s[UPLOAD START]%s %s <- %s\n", colorBlue, colorReset, filepath.Base(finalPath), remote)
		if b, jerr := json.Marshal(map[string]string{"type": "start", "file": filepath.Base(finalPath)}); jerr == nil {
			broker.Publish(string(b))
		}
		out, err := os.Create(tmpPath)
		if err != nil {
			logMessage("error", "Unable to create file", map[string]interface{}{"error": err.Error(), "remote": remote})
			http.Error(w, "Unable to create file", http.StatusInternalServerError)
			return
		}
		written, err := io.Copy(out, part)
		out.Close()
		part.Close()
		if err != nil {
			os.Remove(tmpPath)
			logMessage("error", "Failed to save file", map[string]interface{}{"error": err.Error(), "remote": remote})
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmpPath, finalPath); err != nil {
			os.Remove(tmpPath)
			logMessage("error", "Failed to finalize file", map[string]interface{}{"error": err.Error(), "remote": remote})
			http.Error(w, "Failed to finalize file", http.StatusInternalServerError)
			return
		}
		savedFile = filepath.Base(finalPath)
		totalBytes = written
		if b, jerr := json.Marshal(map[string]string{"type": "new", "file": savedFile}); jerr == nil {
			broker.Publish(string(b))
		}
		logMessage("info", "Upload complete", map[string]interface{}{
			"type":     "upload",
			"status":   "success",
			"filename": savedFile,
			"size":     totalBytes,
			"remote":   remote,
			"duration": time.Since(start).String(),
		})
		fmt.Printf("%s[UPLOAD]%s %s (%d bytes) complete in %v\n", colorGreen, colorReset, savedFile, written, time.Since(start))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"file":   savedFile,
		"bytes":  fmt.Sprintf("%d", totalBytes),
	})
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(uploadsDir)
	if err != nil {
		logMessage("error", "Cannot read uploads dir", map[string]interface{}{"error": err.Error()})
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
	json.NewEncoder(w).Encode(names)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}
	filename = sanitizeFileName(filename)
	filePath := filepath.Join(uploadsDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logMessage("warning", "Download file not found", map[string]interface{}{"filename": filename, "remote": r.RemoteAddr})
		http.NotFound(w, r)
		return
	}
	logMessage("info", "Download started", map[string]interface{}{
		"type":     "download",
		"status":   "success",
		"filename": filename,
		"remote":   r.RemoteAddr,
		"duration": time.Since(start).String(),
	})
	fmt.Printf("%s[DOWNLOAD]%s %s\n", colorYellow, colorReset, filename)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	http.ServeFile(w, r, filePath)
}

func maxSizeHandler(w http.ResponseWriter, r *http.Request) {
	sizeMB := maxUploadSize >> 20
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"maxUploadMB": sizeMB})
}

func localIPHandler(w http.ResponseWriter, r *http.Request) {
	ips := getPrivateIPs()
	if len(ips) > 0 {
		logMessage("info", "Local IP requested", map[string]interface{}{"ips": ips})
		json.NewEncoder(w).Encode(map[string]string{"ip": ips[0]})
		return
	}
	logMessage("warning", "No local IP found", map[string]interface{}{})
	http.Error(w, "No local IP found", http.StatusNotFound)
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
	logMessage("info", "SSE client connected", map[string]interface{}{"remote": r.RemoteAddr})
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
			logMessage("info", "SSE client disconnected", map[string]interface{}{"remote": r.RemoteAddr})
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
		logMessage("info", "HTTP request", map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"remote":   r.RemoteAddr,
			"duration": elapsed.String(),
		})
		fmt.Printf("%s[%s]%s %s %s %v\n", methodColor, r.Method, colorReset, r.RemoteAddr, r.URL.Path, elapsed)
	})
}

func zipDownloadHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	filesParam := r.URL.Query().Get("files")
	if filesParam == "" {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}
	filenames := strings.Split(filesParam, ",")
	if len(filenames) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}
	var validFiles []string
	for _, f := range filenames {
		cleanName := sanitizeFileName(f)
		if cleanName == "" {
			continue
		}
		filePath := filepath.Join(uploadsDir, cleanName)
		if _, err := os.Stat(filePath); err == nil {
			validFiles = append(validFiles, cleanName)
		}
	}
	if len(validFiles) == 0 {
		logMessage("warning", "No valid files for ZIP", map[string]interface{}{"remote": r.RemoteAddr})
		http.Error(w, "No valid files found", http.StatusNotFound)
		return
	}
	timestamp := time.Now().Format("20060102_150405")
	zipName := fmt.Sprintf("overshare_%s.zip", timestamp)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipName))
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, filename := range validFiles {
		filePath := filepath.Join(uploadsDir, filename)
		file, err := os.Open(filePath)
		if err != nil {
			logMessage("error", "Failed to open file for ZIP", map[string]interface{}{"filename": filename, "error": err.Error()})
			continue
		}
		fileInfo, err := file.Stat()
		if err != nil {
			file.Close()
			logMessage("error", "Failed to stat file for ZIP", map[string]interface{}{"filename": filename, "error": err.Error()})
			continue
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			file.Close()
			logMessage("error", "Failed to create ZIP header", map[string]interface{}{"filename": filename, "error": err.Error()})
			continue
		}
		header.Name = filename
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			file.Close()
			logMessage("error", "Failed to create ZIP entry", map[string]interface{}{"filename": filename, "error": err.Error()})
			continue
		}
		_, err = io.Copy(writer, file)
		file.Close()
		if err != nil {
			logMessage("error", "Failed to write file to ZIP", map[string]interface{}{"filename": filename, "error": err.Error()})
			continue
		}
		fmt.Printf("%s[ZIP ADDED]%s %s\n", colorCyan, colorReset, filename)
	}
	logMessage("info", "ZIP created", map[string]interface{}{
		"type":       "zip",
		"status":     "success",
		"files":      validFiles,
		"file_count": len(validFiles),
		"remote":     r.RemoteAddr,
		"duration":   time.Since(start).String(),
	})
	fmt.Printf("%s[ZIP CREATED]%s %s with %d files\n", colorGreen, colorReset, zipName, len(validFiles))
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://cdn.jsdelivr.net https://unpkg.com 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' https://fonts.googleapis.com 'unsafe-inline'; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' https://api.qrserver.com data: blob:; "+
				"connect-src 'self' https://unpkg.com;")
		next.ServeHTTP(w, r)
	})
}

func createOneshotHandler(filePath string, downloadCount *int32, done chan bool, maxDownloads int) http.HandlerFunc {
	templatePath := filepath.Join(wwwDir, "oneshot.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		logMessage("warning", "Could not read oneshot.html", map[string]interface{}{"error": err.Error()})
		tmpl = template.Must(template.New("oneshot").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>OverShare</title><link rel="stylesheet" href="/oneshot.css"></head><body><h1>Download: {{.FileName}}</h1><p>Size: {{.FileSize}}</p><a href="?download=1">Download</a></body></html>`))
	}
	var shutdownScheduled int32
	return func(w http.ResponseWriter, r *http.Request) {
		filename := filepath.Base(filePath)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		fileSize := formatFileSize(fileInfo.Size())
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		current := atomic.LoadInt32(downloadCount)
		if r.URL.Query().Get("download") == "1" {
			if current >= int32(maxDownloads) {
				logMessage("warning", "Download limit reached", map[string]interface{}{"filename": filename, "remote": r.RemoteAddr})
				http.Error(w, "Download limit reached", http.StatusForbidden)
				return
			}
			newCount := atomic.AddInt32(downloadCount, 1)
			if newCount > int32(maxDownloads) {
				atomic.AddInt32(downloadCount, -1)
				http.Error(w, "Download limit reached", http.StatusForbidden)
				return
			}
			logMessage("info", "One-shot download", map[string]interface{}{
				"type":          "download",
				"status":        "success",
				"filename":      filename,
				"remote":        r.RemoteAddr,
				"downloads":     int(newCount),
				"max_downloads": maxDownloads,
			})
			fmt.Printf("%s[ONESHOT DOWNLOAD %d/%d]%s %s from %s\n",
				colorYellow, newCount, maxDownloads, colorReset, filename, r.RemoteAddr)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
			http.ServeFile(w, r, filePath)
			if newCount == int32(maxDownloads) {
				if atomic.CompareAndSwapInt32(&shutdownScheduled, 0, 1) {
					logMessage("info", "Max downloads reached, shutting down", map[string]interface{}{"max": maxDownloads})
					fmt.Printf("%s[ONESHOT]%s Max downloads (%d) reached, server will shut down in 3 seconds.\n",
						colorCyan, colorReset, maxDownloads)
					time.AfterFunc(3*time.Second, func() {
						done <- true
					})
				}
			}
			return
		}
		if r.URL.Query().Get("status") == "1" {
			current := atomic.LoadInt32(downloadCount)
			expired := current >= int32(maxDownloads)
			remaining := maxDownloads - int(current)
			if remaining < 0 {
				remaining = 0
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"expired":   expired,
				"remaining": remaining,
				"max":       maxDownloads,
			})
			return
		}
		data := struct {
			FileName string
			FileSize string
		}{
			FileName: filename,
			FileSize: fileSize,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
	}
}

func authAndLogHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if authUsername != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != authUsername || pass != authPassword {
				clientIP := r.RemoteAddr
				if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
					clientIP = clientIP[:idx]
				}
				status := "missing"
				if ok {
					status = "invalid"
				}
				logMessage("warning", "Authentication failed", map[string]interface{}{
					"username": user,
					"remote":   clientIP,
					"path":     r.URL.Path,
					"status":   status,
				})
				if !ok {
					fmt.Printf("%s[FAILED AUTH]%s Missing credentials from %s %s\n",
						colorRed, colorReset, clientIP, r.URL.Path)
				} else {
					fmt.Printf("%s[FAILED AUTH]%s Invalid credentials (user: %s) from %s %s\n",
						colorRed, colorReset, user, clientIP, r.URL.Path)
				}
				w.Header().Set("WWW-Authenticate", `Basic realm="OverShare File Server"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			logMessage("info", "Authentication success", map[string]interface{}{
				"username": user,
				"remote":   r.RemoteAddr,
				"path":     r.URL.Path,
			})
		}
		handler(w, r)
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
	}
}
func runOneshotMode(filePath string, timeout int, bindHost, bindPort string, maxDownloads int) error {
	if p, err := strconv.Atoi(bindPort); err != nil || p <= 0 || p > 65535 {
		return fmt.Errorf("invalid port: %s", bindPort)
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file error: %v", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, please specify a file")
	}
	listenAddr := fmt.Sprintf("%s:%s", bindHost, bindPort)
	var displayHost string
	if bindHost == "0.0.0.0" {
		privateIPs := getPrivateIPs()
		if len(privateIPs) > 0 {
			displayHost = privateIPs[0]
		} else {
			displayHost = "localhost"
		}
	} else {
		displayHost = bindHost
	}
	fullURL := fmt.Sprintf("http://%s:%s/", displayHost, bindPort)
	var downloadCount int32 = 0
	fmt.Printf("%s[DEBUG]%s Serving static files from: %s\n", colorYellow, colorReset, wwwDir)
	done := make(chan bool, 1)
	mux := http.NewServeMux()

	// REMOVE THIS LINE: fs := http.FileServer(http.Dir(wwwDir))

	// Handle static files explicitly
	mux.HandleFunc("/oneshot.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(wwwDir, "oneshot.css"))
	})
	mux.HandleFunc("/oneshot.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(wwwDir, "oneshot.js"))
	})
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(wwwDir, "style.css"))
	})
	mux.HandleFunc("/index.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(wwwDir, "index.js"))
	})
	mux.HandleFunc("/cat.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(wwwDir, "cat.png"))
	})

	mainHandler := createOneshotHandler(filePath, &downloadCount, done, maxDownloads)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mainHandler(w, r)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// For any other path, try to serve as static file
		staticPath := filepath.Join(wwwDir, r.URL.Path)
		if _, err := os.Stat(staticPath); err == nil {
			http.ServeFile(w, r, staticPath)
			return
		}
		http.NotFound(w, r)
	})

	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	logMessage("info", "One-shot mode started", map[string]interface{}{
		"file":          filepath.Base(filePath),
		"size":          fileInfo.Size(),
		"port":          bindPort,
		"max_downloads": maxDownloads,
		"url":           fullURL,
		"ips":           getPrivateIPs(),
	})
	fmt.Printf("\n%s[ONESHOT MODE]%s Ready to send: %s\n", colorCyan, colorReset, filepath.Base(filePath))
	fmt.Printf("%sFile size:%s %s\n", colorCyan, colorReset, formatFileSize(fileInfo.Size()))
	fmt.Printf("%sPort:%s %s\n", colorCyan, colorReset, bindPort)
	if authUsername != "" {
		fmt.Printf("%sAuth:%s %s (password protected)\n", colorCyan, colorReset, authUsername)
	} else {
		fmt.Printf("%sAuth:%s disabled\n", colorCyan, colorReset)
	}
	if timeout > 0 {
		fmt.Printf("%sTimeout:%s %d seconds\n", colorCyan, colorReset, timeout)
	} else {
		fmt.Printf("%sTimeout:%s disabled\n", colorCyan, colorReset)
	}
	fmt.Printf("%sMax downloads:%s %d\n", colorCyan, colorReset, maxDownloads)
	fmt.Printf("%sURL:%s %s\n", colorCyan, colorReset, fullURL)
	fmt.Printf("%sQR Code:%s\n", colorCyan, colorReset)
	printQRCode(fullURL)
	fmt.Printf("\n%s[ONE-TIME]%s Server will exit after %d download(s)\n", colorYellow, colorReset, maxDownloads)
	fmt.Printf("Press Ctrl+C to stop the server\n\n")
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()
	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timeoutChan = time.After(time.Duration(timeout) * time.Second)
	}
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	select {
	case <-done:
		logMessage("info", "Transfer complete, shutting down", nil)
		fmt.Printf("\n%s[TRANSFER COMPLETE]%s Max downloads reached. Shutting down...\n", colorGreen, colorReset)
	case <-timeoutChan:
		logMessage("info", "Timeout reached, shutting down", map[string]interface{}{"timeout": timeout})
		fmt.Printf("\n%s[TIMEOUT]%s %d seconds elapsed, shutting down\n", colorRed, colorReset, timeout)
	case err := <-serverErr:
		logMessage("error", "Server error", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("server error: %v", err)
	case <-interruptChan:
		logMessage("info", "Interrupted, shutting down", nil)
		fmt.Printf("\n%s[INTERRUPT]%s Shutting down...\n", colorYellow, colorReset)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
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
					logMessage("info", "File added", map[string]interface{}{"filename": name})
					fmt.Printf("%s[FS NEW]%s %s\n", colorGreen, colorReset, name)
				}
			}
			for name := range snapshot {
				if _, ok := current[name]; !ok {
					if b, jerr := json.Marshal(map[string]string{"type": "remove", "file": name}); jerr == nil {
						broker.Publish(string(b))
					}
					logMessage("info", "File removed", map[string]interface{}{"filename": name})
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
		logMessage("error", "Failed to get network interfaces", map[string]interface{}{"error": err.Error()})
		return ips
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
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

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authUsername == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != authUsername || pass != authPassword {
			clientIP := r.RemoteAddr
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
			logMessage("warning", "Authentication failed", map[string]interface{}{
				"username": user,
				"remote":   clientIP,
				"path":     r.URL.Path,
			})
			if !ok {
				fmt.Printf("%s[FAILED AUTH]%s Missing credentials from %s %s\n",
					colorRed, colorReset, clientIP, r.URL.Path)
			} else {
				fmt.Printf("%s[FAILED AUTH]%s Invalid credentials (user: %s) from %s %s\n",
					colorRed, colorReset, user, clientIP, r.URL.Path)
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="OverShare File Server"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func findWwwDir() string {
	binaryDir := getBinaryDir()
	binaryWww := filepath.Join(binaryDir, "www")
	if _, err := os.Stat(binaryWww); err == nil {
		logMessage("info", "Found www next to binary", map[string]interface{}{"path": binaryWww})
		fmt.Printf("%s[WWW]%s Found www next to binary: %s\n", colorCyan, colorReset, binaryWww)
		return binaryWww
	}
	homeDir, err := os.UserHomeDir()
	if err == nil {
		localWww := filepath.Join(homeDir, ".local", "share", "overshare", "www")
		if _, err := os.Stat(localWww); err == nil {
			logMessage("info", "Found www in user share", map[string]interface{}{"path": localWww})
			fmt.Printf("%s[WWW]%s Found www in user share: %s\n", colorCyan, colorReset, localWww)
			return localWww
		}
	}
	logMessage("info", "Using default www directory", map[string]interface{}{"path": "www"})
	fmt.Printf("%s[WWW]%s Using default www in current directory\n", colorYellow, colorReset)
	return "www"
}

func startAbsoluteTimeoutMonitor(server *http.Server, timeoutSeconds int) {
	if timeoutSeconds <= 0 {
		return
	}
	time.AfterFunc(time.Duration(timeoutSeconds)*time.Second, func() {
		logMessage("info", "Timeout reached, shutting down", map[string]interface{}{"timeout": timeoutSeconds})
		fmt.Printf("\n%s[TIMEOUT]%s %d seconds elapsed, shutting down.\n", colorRed, colorReset, timeoutSeconds)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logMessage("error", "Shutdown error", map[string]interface{}{"error": err.Error()})
		}
	})
}

func extractUnknownFlag(errMsg string) string {
	parts := strings.Split(errMsg, ":")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}
	return errMsg
}

func main() {
	printBanner()
	var help bool
	var maxMB int64
	var showQR bool
	pflag.StringVar(&host, "host", "0.0.0.0", "Host to bind")
	pflag.StringVar(&port, "port", "8000", "Port to listen on")
	pflag.Int64Var(&maxMB, "maxmb", 200, "Maximum upload size in MB")
	pflag.StringVar(&wwwDir, "www", "www", "Directory to serve static files")
	pflag.StringVar(&uploadsDir, "uploads", "", "Directory to store uploads (default current dir)")
	pflag.BoolVar(&showQR, "qr", false, "Show QR code for server URL")
	pflag.StringVar(&authUsername, "username", "", "Username for HTTP authentication (leave empty to disable auth)")
	pflag.StringVar(&authPassword, "password", "", "Password for HTTP authentication")
	pflag.IntVar(&shutdownTimeout, "timeout", 0, "Exit after this many seconds (0 = disabled)")
	pflag.StringVar(&logFilePath, "log-file", "", "Path to JSON log file (all events logged)")
	pflag.BoolVar(&oneshotMode, "oneshot", false, "Enable one-shot mode with file (e.g., --oneshot file.txt)")
	pflag.IntVar(&maxDownloads, "max-downloads", 1, "Maximum number of downloads in oneshot mode")
	pflag.BoolVar(&help, "help", false, "Show help")
	pflag.Usage = printHelp
	pflag.CommandLine.Init(pflag.CommandLine.Name(), pflag.ContinueOnError)
	pflag.CommandLine.SetOutput(os.Stderr)
	err := pflag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		if strings.Contains(err.Error(), "unknown flag") {
			unknownFlag := extractUnknownFlag(err.Error())
			fmt.Printf("\n%s[ERROR]%s Unknown flag: %s%s%s\n",
				colorRed, colorReset, colorYellow, unknownFlag, colorReset)
			fmt.Printf("%s[HELP]%s Run '%s--help%s' for available flags\n\n",
				colorCyan, colorReset, colorYellow, colorReset)
			os.Exit(1)
		}
		fmt.Printf("\n%s[ERROR]%s Flag parsing failed: %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}
	if help {
		printHelp()
		return
	}
	if oneshotMode {
		args := pflag.Args()
		if len(args) < 1 {
			log.Fatal("Error: filename required with --oneshot (e.g., --oneshot file.txt)")
		}
		oneshotFile = args[0]

		if wwwDir == "www" {
			wwwDir = findWwwDir()
			fmt.Printf("%s[WWW]%s Using www directory: %s\n", colorCyan, colorReset, wwwDir)
		}

		if err := runOneshotMode(oneshotFile, shutdownTimeout, host, port, maxDownloads); err != nil {
			log.Fatalf("Oneshot mode failed: %v", err)
		}
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
	maxUploadSize = maxMB << 20
	if p, err := strconv.Atoi(port); err != nil || p <= 0 || p > 65535 {
		log.Fatalf("Invalid port: %s", port)
	}
	if maxUploadSize <= 0 {
		log.Fatalf("Invalid maxmb: %d", maxMB)
	}
	if err := initLogFile(); err != nil {
		log.Fatalf("Failed to initialize log file: %v", err)
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()
	logMessage("info", "Server starting", map[string]interface{}{
		"host":         host,
		"port":         port,
		"max_upload":   maxMB,
		"uploads_dir":  uploadsDir,
		"www_dir":      wwwDir,
		"auth_enabled": authUsername != "",
		"timeout":      shutdownTimeout,
		"ips":          getPrivateIPs(),
	})
	ensureDirs()
	watchUploadsPolling(uploadsDir, pollInterval)
	fileServer := http.FileServer(http.Dir(wwwDir))
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(wwwDir, "index.html"))
			return
		}
		requestedPath := filepath.Join(wwwDir, r.URL.Path)
		if _, err := os.Stat(requestedPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	http.Handle("/", securityHeadersMiddleware(logRequest(authMiddleware(rootHandler))))
	http.Handle("/upload", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(uploadHandler)))))
	http.Handle("/files", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(filesHandler)))))
	http.Handle("/zip", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(zipDownloadHandler)))))
	http.Handle("/download/", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(downloadHandler)))))
	http.Handle("/maxsize", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(maxSizeHandler)))))
	http.Handle("/api/local-ip", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(localIPHandler)))))
	http.Handle("/events", securityHeadersMiddleware(logRequest(authMiddleware(http.HandlerFunc(eventsHandler)))))
	listenAddr := host + ":" + port
	server := &http.Server{Addr: listenAddr}
	uploadDisplay := uploadsDir
	if useCurrentDir {
		uploadDisplay = "current directory (" + uploadsDir + ")"
	}
	fmt.Printf("%s[SERVER]%s Listening on %s | Max upload: %d MB\n", colorCyan, colorReset, listenAddr, maxMB)
	fmt.Printf("%s[UPLOADS]%s Using: %s\n", colorCyan, colorReset, uploadDisplay)
	if shutdownTimeout > 0 {
		fmt.Printf("%s[TIMEOUT]%s Server will exit after %d seconds\n", colorCyan, colorReset, shutdownTimeout)
	} else {
		fmt.Printf("%s[TIMEOUT]%s No timeout\n", colorCyan, colorReset)
	}
	fmt.Printf("\n%s[ACCESS URLs]%s\n", colorCyan, colorReset)
	fmt.Printf("  %s• Server Reachable At:%s\n", colorGreen, colorReset)
	fmt.Printf("    → http://localhost:%s/\n", port)
	fmt.Printf("    → http://127.0.0.1:%s/\n", port)
	if host == "0.0.0.0" {
		fmt.Printf("    → http://0.0.0.0:%s/\n", port)
	}
	privateIPs := getPrivateIPs()
	if len(privateIPs) > 0 {
		for _, ip := range privateIPs {
			if ip != "127.0.0.1" {
				fmt.Printf("    → http://%s:%s/\n", ip, port)
			}
		}
	}
	if host != "0.0.0.0" && host != "127.0.0.1" && host != "localhost" && host != "::" {
		fmt.Printf("    → http://%s:%s/\n", host, port)
	}
	if showQR && len(privateIPs) > 0 {
		qrURL := "http://" + privateIPs[0] + ":" + port + "/"
		printQRCode(qrURL)
	}
	fmt.Printf("\n%s[READY]%s Initialization completed >;]\n", colorGreen, colorReset)
	fmt.Println()
	startAbsoluteTimeoutMonitor(server, shutdownTimeout)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
