package models

import (
	"io"
	"os/exec"
	"time"
)

func (ms *MusicTrack) getCMD() *exec.Cmd {
	ms.cmdMu.RLock()
	defer ms.cmdMu.RUnlock()

	return ms.cmd
}

func (ms *MusicTrack) setCMD(v *exec.Cmd) {
	ms.cmdMu.Lock()
	defer ms.cmdMu.Unlock()

	ms.cmd = v
}

func (ms *MusicTrack) getStream() io.ReadCloser {
	ms.streamMu.RLock()
	defer ms.streamMu.RUnlock()

	return ms.stream
}

func (ms *MusicTrack) setStream(v io.ReadCloser) {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()

	ms.stream = v
}

func (ms *MusicTrack) getInitialized() bool {
	ms.initializedMu.RLock()
	defer ms.initializedMu.RUnlock()

	return ms.initialized
}

func (ms *MusicTrack) setInitialized(v bool) {
	ms.initializedMu.Lock()
	defer ms.initializedMu.Unlock()

	ms.initialized = v
}

func (ms *MusicTrack) GetPos() time.Duration {
	ms.posMu.RLock()
	defer ms.posMu.RUnlock()

	return ms.Pos
}

func (ms *MusicTrack) SetPos(v time.Duration) {
	ms.posMu.Lock()
	defer ms.posMu.Unlock()

	ms.Pos = v
}

func (ms *MusicTrack) GetOrdKey() float64 {
	ms.ordKeyMu.RLock()
	defer ms.ordKeyMu.RUnlock()

	return ms.OrdKey
}

func (ms *MusicTrack) SetOrdKey(v float64) {
	ms.ordKeyMu.Lock()
	defer ms.ordKeyMu.Unlock()

	ms.OrdKey = v
}

func (ms *MusicTrack) GetURL() string {
	ms.urlMu.RLock()
	defer ms.urlMu.RUnlock()

	return ms.URL
}

func (ms *MusicTrack) SetURL(v string) {
	ms.urlMu.Lock()
	defer ms.urlMu.Unlock()

	ms.URL = v
}
