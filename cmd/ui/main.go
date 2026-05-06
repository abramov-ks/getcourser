package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

//go:embed index.html
var staticFiles embed.FS

func main() {
	mux := http.NewServeMux()

	static, err := fs.Sub(staticFiles, ".")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/run", handleRun)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	addr := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	fmt.Printf("[+] Grabber UI запущен: %s\n", addr)
	openBrowser(addr)

	log.Fatal(http.Serve(ln, mux))
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	output := r.FormValue("output")
	threads := r.FormValue("threads")
	verbose := r.FormValue("verbose") == "1"
	quality := r.FormValue("quality")

	grabber, err := grabberPath()
	if err != nil {
		http.Error(w, "[!] grabber не найден рядом с сервером\n", http.StatusInternalServerError)
		return
	}

	args := buildArgs(mode, r.FormValue("url"), r.FormValue("yadisk"), quality, output, threads, verbose)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "[+] Запуск: %s %v\n", filepath.Base(grabber), args)
	flusher.Flush()

	cmd := exec.CommandContext(r.Context(), grabber, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(w, "[!] %v\n", err)
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "[!] Не удалось запустить grabber: %v\n", err)
		return
	}

	scanner := bufio.NewScanner(io.MultiReader(stdout))
	for scanner.Scan() {
		fmt.Fprintln(w, scanner.Text())
		flusher.Flush()
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(w, "[!] %v\n", err)
		w.(interface{ WriteHeader(int) }).WriteHeader(http.StatusInternalServerError)
	}
}

func buildArgs(mode, url, yadisk, quality, output, threads string, verbose bool) []string {
	args := []string{}

	if t, err := strconv.Atoi(threads); err == nil && t > 0 {
		args = append(args, fmt.Sprintf("-threads=%d", t))
	}
	if verbose {
		args = append(args, "-verbose")
	}

	switch mode {
	case "yadisk":
		args = append(args, fmt.Sprintf("-yadisk=%s", yadisk))
		if quality != "" {
			args = append(args, fmt.Sprintf("-quality=%s", quality))
		}
	default:
		args = append(args, url)
	}

	if output != "" {
		args = append(args, output)
	}
	return args
}

// grabberPath looks for the grabber binary next to the running UI server.
func grabberPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)

	name := "grabber"
	if runtime.GOOS == "windows" {
		name = "grabber.exe"
	}

	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	// Fallback: search PATH
	return exec.LookPath(name)
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		fmt.Printf("[!] Не удалось открыть браузер: %v\n", err)
	}
}
