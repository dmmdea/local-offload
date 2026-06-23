package config

import "testing"

func TestDefaultVideoFields(t *testing.T) {
	c := Default()
	if c.VideoFPS != 2.0 {
		t.Errorf("VideoFPS = %v, want 2.0", c.VideoFPS)
	}
	if c.VideoMaxFrames != 12 {
		t.Errorf("VideoMaxFrames = %d, want 12", c.VideoMaxFrames)
	}
	if c.VideoFrameWidth != 512 {
		t.Errorf("VideoFrameWidth = %d, want 512", c.VideoFrameWidth)
	}
	if c.FFmpegPath != "ffmpeg" {
		t.Errorf("FFmpegPath = %q, want \"ffmpeg\"", c.FFmpegPath)
	}
}

func TestDefaultSTTFields(t *testing.T) {
	c := Default()
	if c.STTModel != "whisper-stt" {
		t.Errorf("STTModel = %q, want \"whisper-stt\"", c.STTModel)
	}
	if c.STTModelHQ != "whisper-stt-hq" {
		t.Errorf("STTModelHQ = %q, want \"whisper-stt-hq\"", c.STTModelHQ)
	}
	if !c.STTVAD {
		t.Error("STTVAD should default true")
	}
	if c.STTMaxInlineSegments != 120 {
		t.Errorf("STTMaxInlineSegments = %d, want 120", c.STTMaxInlineSegments)
	}
	if !c.STTUnloadAfter {
		t.Error("STTUnloadAfter should default true (zero-always-warm)")
	}
	if c.STTRequestTimeoutSec != 1800 {
		t.Errorf("STTRequestTimeoutSec = %d, want 1800", c.STTRequestTimeoutSec)
	}
	if c.MediaDir == "" {
		t.Error("MediaDir should default to a non-empty path")
	}
}
