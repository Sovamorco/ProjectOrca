package models

import "github.com/bwmarrin/discordgo"

func (q *Queue) getVc() *discordgo.VoiceConnection {
	q.vcMu.RLock()
	defer q.vcMu.RUnlock()

	return q.vc
}

func (q *Queue) setVc(v *discordgo.VoiceConnection) {
	q.vcMu.Lock()
	defer q.vcMu.Unlock()

	q.vc = v
}

func (q *Queue) GetPaused() bool {
	q.pausedMu.RLock()
	defer q.pausedMu.RUnlock()

	return q.Paused
}

func (q *Queue) SetPaused(v bool) {
	q.pausedMu.Lock()
	defer q.pausedMu.Unlock()

	q.Paused = v
}

func (q *Queue) GetLoop() bool {
	q.loopMu.RLock()
	defer q.loopMu.RUnlock()

	return q.Loop
}

func (q *Queue) SetLoop(v bool) {
	q.loopMu.Lock()
	defer q.loopMu.Unlock()

	q.Loop = v
}

func (q *Queue) GetTracks() []*MusicTrack {
	q.tracksMu.RLock()
	defer q.tracksMu.RUnlock()

	return q.Tracks
}

func (q *Queue) SetTracks(v []*MusicTrack) {
	q.tracksMu.Lock()
	defer q.tracksMu.Unlock()

	q.Tracks = v
}
