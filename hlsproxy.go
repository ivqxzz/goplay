package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var mlhlsHostRe = regexp.MustCompile(`mlhls://[^/\s"']*/`)

type hlsProxy struct {
	srv      *http.Server
	ln       net.Listener
	port     int
	basePath string
	localURL string
	mpvCmd   *exec.Cmd
}

var (
	proxyMu       sync.Mutex
	activeProxy   *hlsProxy
	playbackArmed bool
)

func init() {
	go watchPlaylistCollection()
}

func watchPlaylistCollection() {
	for {
		time.Sleep(250 * time.Millisecond)

		mediaStore.mu.Lock()
		collected := mediaStore.collected
		master := mediaStore.playbackLocation
		mediaStore.mu.Unlock()

		proxyMu.Lock()
		if collected && !playbackArmed {
			playbackArmed = true
			proxyMu.Unlock()
			startPlaybackFromStore(master)
			continue
		}
		if !collected && playbackArmed {
			playbackArmed = false
		}
		proxyMu.Unlock()
	}
}

func startPlaybackFromStore(masterURL string) {
	if masterURL == "" {
		log.Printf("hls-proxy: collected, but master URL is empty — nothing to play")
		return
	}

	localURL, err := startHLSProxy(masterURL)
	if err != nil {
		log.Printf("hls-proxy: failed to start proxy: %v", err)
		return
	}

	player, baseArgs := findVideoPlayer()
	if player == "" {
		log.Printf("hls-proxy: neither mpv nor ffplay found — nothing to play HLS with")
		return
	}
	args := append(append([]string{}, baseArgs...), localURL)
	cmd := exec.Command(player, args...)
	if err := cmd.Start(); err != nil {
		log.Printf("hls-proxy: failed to start player: %v", err)
		return
	}

	proxyMu.Lock()
	if activeProxy != nil {
		activeProxy.mpvCmd = cmd
	}
	proxyMu.Unlock()

	log.Printf("hls-proxy: ▶ started %s at %s", filepath.Base(player), localURL)

	go func(c *exec.Cmd) {
		c.Wait()
		log.Printf("hls-proxy: player exited")
	}(cmd)
}

func startHLSProxy(mlhlsURL string) (string, error) {
	proxyMu.Lock()
	defer proxyMu.Unlock()

	if activeProxy != nil {
		activeProxy.stopLocked()
		activeProxy = nil
	}

	basePath := mlhlsPath(mlhlsURL)
	if basePath == "" {
		basePath = "/master.m3u8"
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("hls-proxy: failed to listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	p := &hlsProxy{
		ln:       ln,
		port:     port,
		basePath: basePath,
		localURL: fmt.Sprintf("http://127.0.0.1:%d%s", port, basePath),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handle)
	p.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := p.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("hls-proxy: server stopped: %v", err)
		}
	}()

	activeProxy = p
	log.Printf("hls-proxy: listening on 127.0.0.1:%d, base path=%s, master for mpv -> %s",
		port, basePath, p.localURL)
	return p.localURL, nil
}

func stopHLSProxy() {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if activeProxy != nil {
		activeProxy.stopLocked()
		activeProxy = nil
	}

}

func (p *hlsProxy) stopLocked() {
	if p.mpvCmd != nil && p.mpvCmd.Process != nil {
		p.mpvCmd.Process.Kill()
	}
	p.mpvCmd = nil
	if p.srv != nil {
		_ = p.srv.Close()
	}
	if p.ln != nil {
		_ = p.ln.Close()
	}
	log.Printf("hls-proxy: stopped (port %d)", p.port)
}

func (p *hlsProxy) handle(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path
	if r.URL.RawQuery != "" {
		reqPath += "?" + r.URL.RawQuery
	}

	data, ok := lookupStoredPlaylist(reqPath)
	if !ok {
		log.Printf("hls-proxy: %s — not in store (mpv fetches segments directly from the internet)", reqPath)
		http.NotFound(w, r)
		return
	}

	rewritten := p.rewritePlaylist(data)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rewritten)
	log.Printf("hls-proxy: %s — served playlist from store (%d -> %d bytes)", reqPath, len(data), len(rewritten))
}

func lookupStoredPlaylist(reqPath string) ([]byte, bool) {
	reqNoQuery := stripQuery(reqPath)

	mediaStore.mu.Lock()
	defer mediaStore.mu.Unlock()

	if mediaStore.masterRaw != nil {
		mp := mlhlsPath(mediaStore.playbackLocation)
		if mp == reqPath || stripQuery(mp) == reqNoQuery {
			return mediaStore.masterRaw, true
		}
	}

	for k, v := range mediaStore.mediaPlaylists {
		mp := mlhlsPath(k)
		if mp == reqPath || stripQuery(mp) == reqNoQuery {
			return v, true
		}
	}
	return nil, false
}

func stripQuery(s string) string {
	if i := strings.IndexByte(s, '?'); i >= 0 {
		return s[:i]
	}
	return s
}

func (p *hlsProxy) rewritePlaylist(body []byte) []byte {
	localBase := fmt.Sprintf("http://127.0.0.1:%d/", p.port)
	return mlhlsHostRe.ReplaceAll(body, []byte(localBase))
}

func mlhlsPath(u string) string {
	s := strings.TrimSpace(u)
	const scheme = "mlhls://"
	if !strings.HasPrefix(strings.ToLower(s), scheme) {

		if strings.HasPrefix(s, "/") {
			return s
		}
		return ""
	}
	rest := s[len(scheme):]

	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[i:]
	}

	return "/"
}
