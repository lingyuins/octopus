package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindMatchedKeyword(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
		want     string
	}{
		{
			name:     "no keywords",
			text:     "hello world",
			keywords: nil,
			want:     "",
		},
		{
			name:     "empty text",
			text:     "",
			keywords: []string{"bad"},
			want:     "",
		},
		{
			name:     "exact match",
			text:     "bad word",
			keywords: []string{"bad"},
			want:     "bad",
		},
		{
			name:     "case insensitive",
			text:     "Hello BAD World",
			keywords: []string{"bad"},
			want:     "bad",
		},
		{
			name:     "partial match",
			text:     "this is a badword here",
			keywords: []string{"bad"},
			want:     "bad",
		},
		{
			name:     "no match",
			text:     "good clean content",
			keywords: []string{"bad", "evil"},
			want:     "",
		},
		{
			name:     "chinese keyword",
			text:     "这是一段包含敏感词的内容",
			keywords: []string{"敏感词"},
			want:     "敏感词",
		},
		{
			name:     "multiple keywords match first",
			text:     "foo bar baz",
			keywords: []string{"bar", "foo"},
			want:     "bar",
		},
		{
			name:     "skip empty keywords",
			text:     "some text",
			keywords: []string{"", "  ", ""},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchedKeyword(tt.text, tt.keywords)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceKeywordsInText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
		want     string
	}{
		{
			name:     "single keyword",
			text:     "this is bad content",
			keywords: []string{"bad"},
			want:     "this is *** content",
		},
		{
			name:     "case insensitive replace",
			text:     "Hello BAD World",
			keywords: []string{"bad"},
			want:     "Hello *** World",
		},
		{
			name:     "multiple keywords",
			text:     "bad and evil stuff",
			keywords: []string{"bad", "evil"},
			want:     "*** and **** stuff",
		},
		{
			name:     "chinese keyword replace",
			text:     "这是一段包含敏感词的内容",
			keywords: []string{"敏感词"},
			want:     "这是一段包含***的内容",
		},
		{
			name:     "no match",
			text:     "clean content",
			keywords: []string{"bad"},
			want:     "clean content",
		},
		{
			name:     "repeated keyword",
			text:     "bad bad bad",
			keywords: []string{"bad"},
			want:     "*** *** ***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceKeywordsInText(tt.text, tt.keywords)
			assert.Equal(t, tt.want, got)
		})
	}
}
