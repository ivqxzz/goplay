package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"howett.net/plist"
)

type videoSession struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	url        string
	pendingURL string
	rate       float64
	duration   float64
	startedAt  time.Time
}

var videoSes = &videoSession{}

var urlRe = regexp.MustCompile(`(?:https?|mlhls)://[^\s"'<>\\]+`)

func isMLHLS(u string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(u)), "mlhls://")
}

func extractMediaURL(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if u := parseTextHeaders(body); u != "" {
		return u
	}
	var data interface{}
	if _, err := plist.Unmarshal(body, &data); err == nil {
		if u := findURLInValue(data); u != "" {
			return u
		}
	}
	if m := urlRe.Find(body); m != nil {
		return string(m)
	}
	return ""
}

func parseTextHeaders(body []byte) string {
	s := string(body)
	if strings.HasPrefix(s, "bplist") || strings.ContainsRune(s, 0) {
		return ""
	}
	for _, line := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-location:") {
			val := strings.TrimSpace(line[len("content-location:"):])

			if m := urlRe.FindString(val); m != "" {
				return m
			}
			if f := strings.Fields(val); len(f) > 0 {
				return f[0]
			}
		}
	}

	if m := urlRe.FindString(s); m != "" {
		return m
	}
	return ""
}

func findURLInValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") || isMLHLS(t) {
			return t
		}
	case []interface{}:
		for _, e := range t {
			if u := findURLInValue(e); u != "" {
				return u
			}
		}
	case map[string]interface{}:

		for _, k := range []string{"Content-Location", "ContentLocation", "ContentURL", "WebRTCStreamURL", "url", "URL"} {
			if val, ok := t[k]; ok {
				if u := findURLInValue(val); u != "" {
					return u
				}
			}
		}
		for _, val := range t {
			if u := findURLInValue(val); u != "" {
				return u
			}
		}
	}
	return ""
}

func lookPlayer(name string) string {
	exeName := name
	if runtime.GOOS == "windows" {
		exeName = name + ".exe"
	}
	if self, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(self), exeName)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	if _, err := os.Stat(exeName); err == nil {
		return exeName
	}
	if p, err := exec.LookPath(exeName); err == nil {
		return p
	}
	return ""
}

func findVideoPlayer() (string, []string) {
	if p := lookPlayer("mpv"); p != "" {
		return p, []string{
			"--force-window=yes",
			"--no-terminal",
			"--cache=yes",
			"--hls-bitrate=max",
			"--title=GoPlay — AirPlay Video",
		}
	}
	if p := lookPlayer("ffplay"); p != "" {
		return p, []string{
			"-autoexit",
			"-window_title", "GoPlay — AirPlay Video",
			"-loglevel", "warning",
		}
	}
	return "", nil
}

func (s *videoSession) setPending(u string) {
	s.mu.Lock()
	s.pendingURL = u
	s.mu.Unlock()
}

func (s *videoSession) play(body []byte) {
	url := extractMediaURL(body)

	s.mu.Lock()
	if url == "" {
		url = s.pendingURL
	}
	s.mu.Unlock()

	if url == "" {
		log.Printf("airplay-video: /play without URL (body %d bytes) and nothing pending", len(body))
		return
	}

	if isMLHLS(url) {
		startFCUPPlayback(url)
		return
	}

	stopHLSProxy()

	s.mu.Lock()
	if s.cmd != nil && s.url == url {
		s.mu.Unlock()
		log.Printf("airplay-video: this URL is already playing, skipping")
		return
	}
	s.stopLocked()

	player, baseArgs := findVideoPlayer()
	if player == "" {
		s.mu.Unlock()
		log.Printf("airplay-video: neither mpv nor ffplay found — install one of them")
		return
	}
	args := append(append([]string{}, baseArgs...), url)
	cmd := exec.Command(player, args...)
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		log.Printf("airplay-video: failed to start player: %v", err)
		return
	}
	s.cmd = cmd
	s.url = url
	s.rate = 1.0
	s.duration = 0
	s.startedAt = time.Now()
	s.mu.Unlock()

	log.Printf("airplay-video: ▶ playing %s (via %s)", url, filepath.Base(player))

	go func(c *exec.Cmd) {
		c.Wait()
		s.mu.Lock()
		if s.cmd == c {
			s.cmd = nil
			s.url = ""
		}
		s.mu.Unlock()
		log.Printf("airplay-video: player exited")
	}(cmd)
}

func (s *videoSession) stop() {
	s.mu.Lock()
	s.stopLocked()
	s.pendingURL = ""
	s.mu.Unlock()
	stopHLSProxy()
}

func (s *videoSession) stopLocked() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		log.Printf("airplay-video: ⏹ stopped")
	}
	s.cmd = nil
	s.url = ""
}

func (s *videoSession) setRate(r float64) {
	s.mu.Lock()
	s.rate = r
	s.mu.Unlock()
	log.Printf("airplay-video: rate=%.2f", r)

}

func (s *videoSession) playbackInfoPlist() []byte {
	s.mu.Lock()
	playing := s.cmd != nil
	pos := 0.0
	if playing {
		pos = time.Since(s.startedAt).Seconds()
	}
	dur := s.duration
	s.mu.Unlock()

	rate := 0.0
	if playing {
		rate = 1.0
	}
	if dur <= 0 {
		dur = pos + 600
	}

	info := map[string]interface{}{
		"uuid":                   "goplay-video",
		"duration":               dur,
		"position":               pos,
		"rate":                   rate,
		"readyToPlay":            playing,
		"playbackBufferEmpty":    false,
		"playbackBufferFull":     true,
		"playbackLikelyToKeepUp": true,
		"loadedTimeRanges": []interface{}{
			map[string]interface{}{"start": 0.0, "duration": dur},
		},
		"seekableTimeRanges": []interface{}{
			map[string]interface{}{"start": 0.0, "duration": dur},
		},
	}
	buf, err := plist.Marshal(info, plist.XMLFormat)
	if err != nil {
		log.Printf("airplay-video: failed to build playback-info: %v", err)
		return nil
	}
	return buf
}

func parseRate(path string) float64 {
	i := strings.Index(path, "value=")
	if i < 0 {
		return 1.0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(path[i+len("value="):]), 64)
	if err != nil {
		return 1.0
	}
	return v
}

func handleAirplayVideo(method, path string, body []byte) ([]byte, string, bool) {
	p := path
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}

	switch {
	case method == "POST" && p == "/play":
		videoSes.play(body)
		return nil, "", true

	case method == "POST" && p == "/action":
		handleAction(body)
		return nil, "", true

	case p == "/playback-info":
		return videoSes.playbackInfoPlist(), "text/x-apple-plist+xml", true

	case method == "POST" && p == "/rate":
		videoSes.setRate(parseRate(path))
		return nil, "", true

	case method == "POST" && p == "/stop":
		videoSes.stop()
		return nil, "", true

	case p == "/scrub":

		return nil, "", true

	case method == "PUT" && strings.Contains(p, "setProperty"):

		if u := extractMediaURL(body); u != "" {
			videoSes.setPending(u)
			log.Printf("airplay-video: stored URL from setProperty: %s", u)
		}
		return nil, "", false
	}

	return nil, "", false
}

type mediaDataStore struct {
	mu               sync.Mutex
	masterRaw        []byte
	mediaPlaylists   map[string][]byte
	uriList          []string
	cursor           int
	reqID            int
	sid              string
	playbackLocation string
	collected        bool
}

var mediaStore = &mediaDataStore{}

func (m *mediaDataStore) reset(masterURL, sid string) {
	m.mu.Lock()
	m.masterRaw = nil
	m.mediaPlaylists = map[string][]byte{}
	m.uriList = nil
	m.cursor = 0
	m.reqID = 0
	m.sid = sid
	m.playbackLocation = masterURL
	m.collected = false
	m.mu.Unlock()
}

func (m *mediaDataStore) nextReqID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqID++
	return m.reqID
}

func startFCUPPlayback(masterURL string) {
	if !reverseAlive() {
		log.Printf("airplay-video: /play mlhls (%s), but reverse channel not ready — FCUP impossible", masterURL)
		return
	}
	sid := reverseSID()
	mediaStore.reset(masterURL, sid)
	rid := mediaStore.nextReqID()
	log.Printf("airplay-video: ▶ starting FCUP collection for %s (sid=%s)", masterURL, sid)
	if err := sendFCUPRequest(masterURL, rid, sid); err != nil {
		log.Printf("airplay-video: first FCUP request failed to send: %v", err)
	}
}

func handleAction(body []byte) {
	if len(body) == 0 {
		log.Printf("action: empty body")
		return
	}
	var root map[string]interface{}
	if _, err := plist.Unmarshal(body, &root); err != nil {
		log.Printf("action: not a plist (%d bytes): %v", len(body), err)
		return
	}
	typ, _ := root["type"].(string)
	params, _ := root["params"].(map[string]interface{})

	switch typ {
	case "unhandledURLResponse":
		mediaStore.ingestURLResponse(params)
	case "playlistRemove", "playlistInsert", "playlistAdd":
		log.Printf("action: %s (v1: skipping playlist control)", typ)
	default:
		log.Printf("action: unknown type=%q (body %d bytes)", typ, len(body))
	}
}

func (m *mediaDataStore) ingestURLResponse(params map[string]interface{}) {
	if params == nil {
		log.Printf("action: unhandledURLResponse without params")
		return
	}
	u, _ := params["FCUP_Response_URL"].(string)
	data, _ := params["FCUP_Response_Data"].([]byte)
	if u == "" {
		log.Printf("action: unhandledURLResponse without FCUP_Response_URL")
		return
	}

	m.mu.Lock()
	isMaster := strings.Contains(u, "/master.m3u8")
	if isMaster {
		m.masterRaw = data
		m.uriList = parseMasterMediaURIs(data, u)
		m.cursor = 0
		log.Printf("action: received MASTER %s (%d bytes), media playlists in it: %d", u, len(data), len(m.uriList))
	} else {
		if m.mediaPlaylists == nil {
			m.mediaPlaylists = map[string][]byte{}
		}
		m.mediaPlaylists[u] = data
		log.Printf("action: received MEDIA playlist %s (%d bytes) [%d/%d]", u, len(data), m.cursor, len(m.uriList))
	}

	numURI := len(m.uriList)
	var nextURL string
	var rid int
	if m.cursor < numURI {
		nextURL = m.uriList[m.cursor]
		m.cursor++
		m.reqID++
		rid = m.reqID
	}
	sid := m.sid
	m.mu.Unlock()

	if nextURL != "" {
		if err := sendFCUPRequest(nextURL, rid, sid); err != nil {
			log.Printf("action: failed to request media #%d %s: %v", rid, nextURL, err)
		}
		return
	}
	m.onCollected()
}

func (m *mediaDataStore) onCollected() {
	m.mu.Lock()
	m.collected = true
	nMaster := len(m.masterRaw)
	nMedia := len(m.mediaPlaylists)
	m.mu.Unlock()
	log.Printf("action: ✓ all playlists collected (master %d bytes, media playlists %d). "+
		"mpv launch will be wired up in the next step (hlsproxy.go).", nMaster, nMedia)
}

func parseMasterMediaURIs(master []byte, masterURL string) []string {
	base := uriBase(masterURL)
	var uris []string
	seen := map[string]bool{}
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		full := resolveURI(base, ref)
		if !seen[full] {
			seen[full] = true
			uris = append(uris, full)
		}
	}

	expectURI := false
	for _, raw := range strings.Split(string(master), "\n") {
		t := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if t == "" {
			continue
		}
		switch {
		case strings.HasPrefix(t, "#EXT-X-STREAM-INF"):
			expectURI = true
		case strings.HasPrefix(t, "#EXT-X-MEDIA"):
			if u := extractAttr(t, "URI"); u != "" {
				add(u)
			}
		case strings.HasPrefix(t, "#EXT-X-I-FRAME-STREAM-INF"):
			if u := extractAttr(t, "URI"); u != "" {
				add(u)
			}
		case strings.HasPrefix(t, "#"):

		default:

			if expectURI {
				add(t)
				expectURI = false
			}
		}
	}
	return uris
}

func extractAttr(line, name string) string {
	key := name + "=\""
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	if j := strings.IndexByte(rest, '"'); j >= 0 {
		return rest[:j]
	}
	return ""
}

func uriBase(u string) string {
	if i := strings.LastIndexByte(u, '/'); i >= 0 {
		return u[:i+1]
	}
	return ""
}

func resolveURI(base, ref string) string {
	if strings.Contains(ref, "://") {
		return ref
	}
	if strings.HasPrefix(ref, "/") {

		if i := strings.Index(base, "://"); i >= 0 {
			rest := base[i+3:]
			if j := strings.IndexByte(rest, '/'); j >= 0 {
				return base[:i+3] + rest[:j] + ref
			}
			return base[:i+3] + rest + ref
		}
		return ref
	}
	return base + ref
}
