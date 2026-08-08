package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func validKeywordRule() KeywordRule {
	return KeywordRule{
		Keyword:       "hello",
		MatchMode:     MatchExact,
		CaseSensitive: true,
		Areas:         []TriggerArea{AreaFriend},
		ReplyType:     ReplyText,
		Contents:      []string{"hello back"},
	}
}

func TestValidateRules(t *testing.T) {
	tests := []struct {
		name  string
		rules []KeywordRule
	}{
		{name: "accepts valid rule", rules: []KeywordRule{validKeywordRule()}},
		{name: "rejects missing keyword", rules: []KeywordRule{{MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"reply"}}}},
		{name: "rejects invalid match mode", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchMode("partial"), Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"reply"}}}},
		{name: "rejects missing area", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, ReplyType: ReplyText, Contents: []string{"reply"}}}},
		{name: "rejects invalid area", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{TriggerArea("direct")}, ReplyType: ReplyText, Contents: []string{"reply"}}}},
		{name: "rejects invalid reply type", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyType("image"), Contents: []string{"reply"}}}},
		{name: "rejects missing content", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText}}},
		{name: "rejects empty content", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{""}}}},
		{name: "rejects audio in channel", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaChannel}, ReplyType: ReplyAudio, Contents: []string{"clip.mp3"}}}},
		{name: "rejects video in channel private", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaChannelPrivate}, ReplyType: ReplyVideo, Contents: []string{"clip.mp4"}}}},
		{name: "rejects file in channel", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend, AreaChannel}, ReplyType: ReplyFile, Contents: []string{"file.zip"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRules(tt.rules)
			if tt.name == "accepts valid rule" {
				if err != nil {
					t.Fatalf("ValidateRules() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateRules() error = nil, want validation error")
			}
		})
	}
}

func TestValidateRulesRejectsDuplicateKeyword(t *testing.T) {
	rules := []KeywordRule{
		{Keyword: "hello", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"one"}},
		{Keyword: "hello", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"two"}},
	}
	if err := ValidateRules(rules); err == nil {
		t.Fatal("expected duplicate keyword error")
	}
}

func TestRuleStoreLoadMissingFileUsesEmptyRules(t *testing.T) {
	store := NewRuleStore(filepath.Join(t.TempDir(), "keyword_replies.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := store.Snapshot(); len(got) != 0 {
		t.Fatalf("Snapshot() = %#v, want empty rules", got)
	}
}

func TestRuleStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyword_replies.json")
	store := NewRuleStore(path)
	want := []KeywordRule{{Keyword: "菜单", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"请选择"}}}
	if err := store.Replace(want); err != nil {
		t.Fatal(err)
	}
	loaded := NewRuleStore(path)
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if got := loaded.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRuleStoreSnapshotIsIndependentCopy(t *testing.T) {
	store := NewRuleStore(filepath.Join(t.TempDir(), "keyword_replies.json"))
	if err := store.Replace([]KeywordRule{validKeywordRule()}); err != nil {
		t.Fatal(err)
	}

	snapshot := store.Snapshot()
	snapshot[0].Keyword = "changed"
	snapshot[0].Areas[0] = AreaGroup
	snapshot[0].Contents[0] = "changed reply"

	want := []KeywordRule{validKeywordRule()}
	if got := store.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestRuleStoreReplaceFailurePreservesFileAndSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyword_replies.json")
	store := NewRuleStore(path)
	oldRules := []KeywordRule{validKeywordRule()}
	if err := store.Replace(oldRules); err != nil {
		t.Fatal(err)
	}
	oldFile, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store.replaceFile = func(string, string) error { return errors.New("replace failed") }
	newRules := []KeywordRule{{Keyword: "new", MatchMode: MatchFuzzy, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyMarkdown, Contents: []string{"new reply"}}}
	if err := store.Replace(newRules); err == nil {
		t.Fatal("Replace() error = nil, want replacement error")
	}

	if got := store.Snapshot(); !reflect.DeepEqual(got, oldRules) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, oldRules)
	}
	if got, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(got, oldFile) {
		t.Fatalf("file changed after failed replacement: got %q want %q", got, oldFile)
	}
}

func TestValidateRulesChineseMessages(t *testing.T) {
	tests := []struct {
		name  string
		rules []KeywordRule
		want  string
	}{
		{name: "missing keyword", rules: []KeywordRule{{MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"reply"}}}, want: "第 1 条规则：关键词不能为空"},
		{name: "duplicate keyword", rules: []KeywordRule{validKeywordRule(), validKeywordRule()}, want: "第 2 条规则：关键词“hello”重复"},
		{name: "invalid match mode", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchMode("partial"), Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"reply"}}}, want: "第 1 条规则：匹配模式“partial”无效"},
		{name: "missing area", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, ReplyType: ReplyText, Contents: []string{"reply"}}}, want: "第 1 条规则：至少选择一个触发区域"},
		{name: "invalid area", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{TriggerArea("direct")}, ReplyType: ReplyText, Contents: []string{"reply"}}}, want: "第 1 条规则：触发区域“direct”无效"},
		{name: "invalid reply type", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyType("image"), Contents: []string{"reply"}}}, want: "第 1 条规则：回复类型“image”无效"},
		{name: "missing content", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText}}, want: "第 1 条规则：回复内容不能为空"},
		{name: "invalid image marker", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"[图片=]"}}}, want: "第 1 条规则：普通消息内容无效：图片标记中的地址不能为空"},
		{name: "media channel", rules: []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, Areas: []TriggerArea{AreaChannel}, ReplyType: ReplyAudio, Contents: []string{"clip.mp3"}}}, want: "第 1 条规则：语音回复不能用于频道消息或频道私信"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateRules(test.rules); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRuleStoreChineseErrorContext(t *testing.T) {
	t.Run("decode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "keyword_replies.json")
		if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := NewRuleStore(path).Load()
		if err == nil || !strings.HasPrefix(err.Error(), "解析规则配置失败：") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("replace", func(t *testing.T) {
		cause := errors.New("disk full")
		store := NewRuleStore(filepath.Join(t.TempDir(), "keyword_replies.json"))
		store.replaceFile = func(string, string) error { return cause }
		err := store.Replace([]KeywordRule{validKeywordRule()})
		if !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "替换规则配置文件失败：") {
			t.Fatalf("error = %v", err)
		}
	})
}
