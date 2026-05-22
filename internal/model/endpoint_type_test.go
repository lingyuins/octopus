package model

import "testing"

func TestIsSupportedEndpointType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string is wildcard", "", true},
		{"wildcard", "*", true},
		{"chat", "chat", true},
		{"deepseek", "deepseek", true},
		{"mimo", "mimo", true},
		{"responses", "responses", true},
		{"messages", "messages", true},
		{"embeddings", "embeddings", true},
		{"rerank", "rerank", true},
		{"moderations", "moderations", true},
		{"image_generation", "image_generation", true},
		{"audio_speech", "audio_speech", true},
		{"audio_transcription", "audio_transcription", true},
		{"video_generation", "video_generation", true},
		{"music_generation", "music_generation", true},
		{"search", "search", true},
		{"uppercase CHAT", "CHAT", true},
		{"whitespace padded", "  chat  ", true},
		{"unsupported", "foobar", false},
		{"unsupported with case", "INVALID", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedEndpointType(tt.input); got != tt.expected {
				t.Errorf("IsSupportedEndpointType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
