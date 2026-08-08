package main

import (
	"errors"
	"reflect"
	"testing"
)

type recordingReplyTarget struct {
	calls []string
	errAt int
	err   error
}

func (target *recordingReplyTarget) record(call string) (string, error) {
	target.calls = append(target.calls, call)
	if target.errAt > 0 && len(target.calls) == target.errAt {
		if target.err != nil {
			return "", target.err
		}
		return "", errors.New("send failed")
	}
	return "ok", nil
}

func (target *recordingReplyTarget) SendText(content string) (string, error) {
	return target.record("text:" + content)
}
func (target *recordingReplyTarget) SendImage(content, image string) (string, error) {
	return target.record("image:" + content + ":" + image)
}
func (target *recordingReplyTarget) SendMarkdown(content string) (string, error) {
	return target.record("markdown:" + content)
}
func (target *recordingReplyTarget) SendAudio(file string) (string, error) {
	return target.record("audio:" + file)
}
func (target *recordingReplyTarget) SendVideo(file string) (string, error) {
	return target.record("video:" + file)
}
func (target *recordingReplyTarget) SendFile(file string) (string, error) {
	return target.record("file:" + file)
}

func TestParseTextReply(t *testing.T) {
	tests := []struct {
		name, input, text, image string
		wantErr                  bool
	}{
		{name: "plain", input: "  hello  ", text: "hello"},
		{name: "image", input: " hello [图片= https://example/image.png ] ", text: "hello", image: "https://example/image.png"},
		{name: "empty image", input: "hello[图片=]", wantErr: true},
		{name: "multiple images", input: "a[图片=x]b[图片=y]", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseTextReply(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Text != test.text || got.Image != test.image {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestMediaItems(t *testing.T) {
	got := MediaItems([]string{" first\r\n second \n\n", "\tthird\r\nfourth"})
	want := []string{"first", "second", "third", "fourth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestSendKeywordReplyTextAndImage(t *testing.T) {
	target := &recordingReplyTarget{}
	outcome, err := SendKeywordReply(target, AreaFriend, KeywordRule{ReplyType: ReplyText, Contents: []string{"hello [图片=pic.png]"}})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Sent != 1 || !reflect.DeepEqual(target.calls, []string{"image:hello:pic.png"}) {
		t.Fatalf("outcome=%+v calls=%v", outcome, target.calls)
	}
}

func TestSendKeywordReplyMediaOrderAndStopOnError(t *testing.T) {
	target := &recordingReplyTarget{errAt: 2, err: errors.New("boom")}
	rule := KeywordRule{ReplyType: ReplyAudio, Contents: []string{" one\n two\n three "}}
	outcome, err := SendKeywordReply(target, AreaFriend, rule)
	if err == nil || outcome.Sent != 1 || !reflect.DeepEqual(target.calls, []string{"audio:one", "audio:two"}) {
		t.Fatalf("outcome=%+v err=%v calls=%v", outcome, err, target.calls)
	}
}

func TestSendKeywordReplyMediaTypes(t *testing.T) {
	tests := []struct {
		replyType ReplyType
		want      string
	}{
		{replyType: ReplyAudio, want: "audio:item"},
		{replyType: ReplyVideo, want: "video:item"},
		{replyType: ReplyFile, want: "file:item"},
	}
	for _, test := range tests {
		t.Run(string(test.replyType), func(t *testing.T) {
			target := &recordingReplyTarget{}
			outcome, err := SendKeywordReply(target, AreaGroup, KeywordRule{ReplyType: test.replyType, Contents: []string{"item"}})
			if err != nil || outcome.Sent != 1 || !reflect.DeepEqual(target.calls, []string{test.want}) {
				t.Fatalf("outcome=%+v err=%v calls=%v", outcome, err, target.calls)
			}
		})
	}
}

func TestSendKeywordReplyMarkdown(t *testing.T) {
	target := &recordingReplyTarget{}
	rule := KeywordRule{ReplyType: ReplyMarkdown, Contents: []string{"**hello**", "world"}}
	outcome, err := SendKeywordReply(target, AreaGroup, rule)
	if err != nil || outcome.Sent != 1 || !reflect.DeepEqual(target.calls, []string{"markdown:**hello**\nworld"}) {
		t.Fatalf("outcome=%+v err=%v calls=%v", outcome, err, target.calls)
	}
}

func TestSendKeywordReplyDegradesChannelPrivateMarkdown(t *testing.T) {
	target := &recordingReplyTarget{}
	rule := KeywordRule{ReplyType: ReplyMarkdown, Contents: []string{"**你好**"}}
	outcome, err := SendKeywordReply(target, AreaChannelPrivate, rule)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Degraded || !reflect.DeepEqual(target.calls, []string{"text:**你好**"}) {
		t.Fatalf("outcome=%+v calls=%v", outcome, target.calls)
	}
}

func TestSendKeywordReplyRejectsMediaInChannelAreas(t *testing.T) {
	for _, area := range []TriggerArea{AreaChannel, AreaChannelPrivate} {
		t.Run(string(area), func(t *testing.T) {
			target := &recordingReplyTarget{}
			_, err := SendKeywordReply(target, area, KeywordRule{ReplyType: ReplyAudio, Contents: []string{"clip.mp3"}})
			if err == nil {
				t.Fatal("expected media area error")
			}
			if len(target.calls) != 0 {
				t.Fatalf("unexpected calls: %v", target.calls)
			}
		})
	}
}
