package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var guiSetStreaming = func(active bool) {}

const restartCooldown = 2 * time.Second

const videoQueueCap = 600

type videoPlayer struct {
	mu        sync.Mutex
	mpvCmd    *exec.Cmd
	mpvStdin  io.WriteCloser
	running   bool
	lastStart time.Time
	needIDR   bool
	closeOnce sync.Once
	closed    bool

	spspps []byte

	queue chan []byte
	done  chan struct{}

	qPeak  int
	qLogAt time.Time
}

func newVideoPlayer() (*videoPlayer, error) {
	v := &videoPlayer{
		queue: make(chan []byte, videoQueueCap),
		done:  make(chan struct{}),
	}
	go v.writeLoop()
	v.mu.Lock()
	v.startLocked()
	ok := v.running
	v.mu.Unlock()
	if !ok {
		return v, errors.New("mpv did not start")
	}
	return v, nil
}

func (v *videoPlayer) close() {
	v.mu.Lock()
	first := false
	v.closeOnce.Do(func() { v.closed = true; first = true })
	v.stopLocked()
	v.mu.Unlock()
	if first {
		close(v.done)
	}
}

func (v *videoPlayer) write(au []byte) error {
	if len(au) == 0 {
		return nil
	}

	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return errors.New("video player closed")
	}

	if cfg := mirrorExtractConfig(au); cfg != nil {
		v.spspps = cfg
	}

	v.startLocked()
	if !v.running {
		v.mu.Unlock()
		return nil
	}

	if vclPresent(au) {
		if v.needIDR {
			if !nalPresent(au, 5) {
				v.mu.Unlock()
				return nil
			}
			v.needIDR = false
		}
	}

	var out []byte
	if nalPresent(au, 5) && !(nalPresent(au, 7) && nalPresent(au, 8)) && len(v.spspps) > 0 {
		out = make([]byte, 0, len(v.spspps)+len(au))
		out = append(out, v.spspps...)
		out = append(out, au...)
	} else {

		out = make([]byte, len(au))
		copy(out, au)
	}

	q := v.queue
	v.mu.Unlock()

	select {
	case q <- out:
	default:
		q <- out
	}
	v.noteQueueDepth(len(q))
	return nil
}

func (v *videoPlayer) writeLoop() {
	for {
		select {
		case <-v.done:
			return
		case out := <-v.queue:
			v.mu.Lock()
			w := v.mpvStdin
			closed := v.closed
			v.mu.Unlock()
			if closed {
				return
			}
			if w == nil {
				continue
			}
			if _, err := w.Write(out); err != nil {
				v.mu.Lock()
				if v.mpvStdin == w {
					log.Printf("video: mpv died during write (%v)", err)
					v.stopLocked()
				}
				v.mu.Unlock()
			}
		}
	}
}

func (v *videoPlayer) noteQueueDepth(depth int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if depth > v.qPeak {
		v.qPeak = depth
	}
	now := time.Now()
	if v.qLogAt.IsZero() {
		v.qLogAt = now
		return
	}
	if now.Sub(v.qLogAt) >= time.Second {
		log.Printf("video: frame queue over 1s: peak=%d, now=%d (cap=%d)", v.qPeak, depth, videoQueueCap)
		v.qPeak = depth
		v.qLogAt = now
	}
}

func (v *videoPlayer) drainQueue() {
	for {
		select {
		case <-v.queue:
		default:
			return
		}
	}
}

func (v *videoPlayer) startLocked() {
	if v.running || v.closed {
		return
	}
	if !v.lastStart.IsZero() && time.Since(v.lastStart) < restartCooldown {
		return
	}
	v.lastStart = time.Now()

	dir := mirrorExeDir()
	mpv := findMirrorMpv()

	mpvArgs := []string{
		"--no-config",
		"--no-audio",

		"--hwdec=auto",

		"--demuxer=lavf",
		"--demuxer-lavf-format=h264",
		"--demuxer-lavf-analyzeduration=0",
		"--demuxer-lavf-probesize=32768",

		"--video-sync=desync",
		"--cache=no",

		"--demuxer-readahead-secs=0",
		"--demuxer-max-bytes=4MiB",
		"--demuxer-max-back-bytes=0",

		"--video-latency-hacks=yes",

		"--untimed",
		"--no-correct-pts",

		"--container-fps-override=120",
		"--framedrop=vo",
		"--keepaspect=yes",
		"--keep-open=no",
		"--osc=no",
		"--title=GoPlay — AirPlay",
		"--log-file=" + filepath.Join(dir, "mpv.log"),
		"--msg-level=all=status",
		"--gpu-api=d3d11",

		"--d3d11-sync-interval=0",
		"--swapchain-depth=1",
	}

	mpvArgs = append(mpvArgs, "--force-window=immediate", "--autofit=50%")
	mpvArgs = append(mpvArgs, "-")

	mpvCmd := exec.Command(mpv, mpvArgs...)
	mpvStdin, err := mpvCmd.StdinPipe()
	if err != nil {
		log.Printf("video: failed to open mpv stdin (%v)", err)
		return
	}
	mpvCmd.Stderr = nil

	if err := mpvCmd.Start(); err != nil {
		log.Printf("video: failed to start mpv (%v)", err)
		mpvStdin.Close()
		return
	}

	v.mpvCmd = mpvCmd
	v.mpvStdin = mpvStdin
	v.running = true
	v.needIDR = true

	v.drainQueue()

	log.Printf("video: started mpv(%s) with direct H.264 (hwdec=auto), log -> %s",
		mpv, filepath.Join(dir, "mpv.log"))

	guiSetStreaming(true)

	go v.monitor(mpvCmd)
}

func (v *videoPlayer) monitor(mpv *exec.Cmd) {
	mpv.Wait()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.mpvCmd == mpv {
		if !v.closed {
			log.Printf("video: mpv exited — stopping")
		}
		v.stopLocked()
	} else if mpv.Process != nil {

		_ = mpv.Process.Kill()
	}
}

func (v *videoPlayer) stopLocked() {
	if v.mpvStdin != nil {
		v.mpvStdin.Close()
		v.mpvStdin = nil
	}
	if v.mpvCmd != nil && v.mpvCmd.Process != nil {
		_ = v.mpvCmd.Process.Kill()
	}
	v.mpvCmd = nil
	v.running = false

	guiSetStreaming(false)
}

func nalPresent(b []byte, nalType byte) bool {
	for i := 0; i+4 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 0 && b[i+3] == 1 {
			if b[i+4]&0x1f == nalType {
				return true
			}
			i += 3
		}
	}
	return false
}

func vclPresent(b []byte) bool {
	for i := 0; i+4 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 0 && b[i+3] == 1 {
			t := b[i+4] & 0x1f
			if t >= 1 && t <= 5 {
				return true
			}
			i += 3
		}
	}
	return false
}

func mirrorSplitAnnexB(b []byte) [][]byte {
	var starts []int
	for i := 0; i+3 <= len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			starts = append(starts, i)
			i += 2
		}
	}
	var nals [][]byte
	for k := 0; k < len(starts); k++ {
		begin := starts[k] + 3
		var end int
		if k+1 < len(starts) {
			end = starts[k+1]

			if end > begin && b[end-1] == 0 {
				end--
			}
		} else {
			end = len(b)
		}
		if end > begin {
			nals = append(nals, b[begin:end])
		}
	}
	return nals
}

func mirrorExtractConfig(au []byte) []byte {
	var sps, pps []byte
	for _, n := range mirrorSplitAnnexB(au) {
		if len(n) == 0 {
			continue
		}
		switch n[0] & 0x1f {
		case 7:
			if sps == nil {
				sps = n
			}
		case 8:
			if pps == nil {
				pps = n
			}
		}
	}
	if sps == nil || pps == nil {
		return nil
	}
	out := make([]byte, 0, len(sps)+len(pps)+8)
	out = append(out, 0, 0, 0, 1)
	out = append(out, sps...)
	out = append(out, 0, 0, 0, 1)
	out = append(out, pps...)
	return out
}

func mirrorExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func mirrorMpvName() string {
	if runtime.GOOS == "windows" {
		return "mpv.exe"
	}
	return "mpv"
}

func findMirrorMpv() string {
	cand := filepath.Join(mirrorExeDir(), mirrorMpvName())
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return mirrorMpvName()
}
