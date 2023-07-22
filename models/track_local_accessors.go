package models

import (
	"io"
	"os/exec"
	"time"
)

func (t *LocalTrack) getRemote() *RemoteTrack {
	t.remoteMu.RLock()
	defer t.remoteMu.RUnlock()

	return t.remote
}

func (t *LocalTrack) setRemote(v *RemoteTrack) {
	t.remoteMu.Lock()
	defer t.remoteMu.Unlock()

	t.remote = v

	if v != nil {
		t.setPos(v.Pos)
	}
}

func (t *LocalTrack) getCMD() *exec.Cmd {
	t.cmdMu.RLock()
	defer t.cmdMu.RUnlock()

	return t.cmd
}

func (t *LocalTrack) setCMD(v *exec.Cmd) {
	t.cmdMu.Lock()
	defer t.cmdMu.Unlock()

	t.cmd = v
}

func (t *LocalTrack) getStream() io.ReadCloser {
	t.streamMu.RLock()
	defer t.streamMu.RUnlock()

	return t.stream
}

func (t *LocalTrack) setStream(v io.ReadCloser) {
	t.streamMu.Lock()
	defer t.streamMu.Unlock()

	t.stream = v
}

func (t *LocalTrack) getPos() time.Duration {
	t.posMu.Lock()
	defer t.posMu.Unlock()

	return t.pos
}

func (t *LocalTrack) setPos(v time.Duration) {
	t.posMu.Lock()
	defer t.posMu.Unlock()

	t.pos = v
}
