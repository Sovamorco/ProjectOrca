package utils

import (
	"io"
	"os/exec"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

func GetYTDLPOutput(logger *zap.SugaredLogger, args ...string) ([]byte, error) {
	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
	}
	ytdlpArgs = append(ytdlpArgs, args...)

	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)

	logger.Debug(ytdlp)

	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp stdout pipe")
	}

	err = ytdlp.Start()
	if err != nil {
		return nil, errorx.Decorate(err, "start ytdlp")
	}

	jsonB, err := io.ReadAll(stdout)
	if err != nil {
		return nil, errorx.Decorate(err, "read Stream url")
	}

	err = ytdlp.Wait()
	if err != nil {
		return nil, errorx.Decorate(err, "wait for ytdlp")
	}

	return jsonB, nil
}
