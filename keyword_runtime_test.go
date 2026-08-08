package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestHandleKeywordMessageNoMatch(t *testing.T) {
	target := &recordingReplyTarget{}
	rules := []KeywordRule{{
		Keyword:       "hello",
		MatchMode:     MatchExact,
		CaseSensitive: true,
		Areas:         []TriggerArea{AreaFriend},
		ReplyType:     ReplyText,
		Contents:      []string{"world"},
	}}

	matched, outcome, err := processKeywordMessage(rules, target, AreaFriend, "other")
	if err != nil || matched || outcome != (ReplyOutcome{}) || len(target.calls) != 0 {
		t.Fatalf("matched=%v outcome=%+v err=%v calls=%v", matched, outcome, err, target.calls)
	}
}

func TestHandleKeywordMessageUsesFirstMatch(t *testing.T) {
	target := &recordingReplyTarget{}
	rules := []KeywordRule{
		{Keyword: "hello", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"first"}},
		{Keyword: "hello there", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"second"}},
	}

	matched, outcome, err := processKeywordMessage(rules, target, AreaGroup, "hello there")
	if err != nil || !matched || outcome.Sent != 1 || !reflect.DeepEqual(target.calls, []string{"text:first"}) {
		t.Fatalf("matched=%v outcome=%+v err=%v calls=%v", matched, outcome, err, target.calls)
	}
}

func TestHandleKeywordMessageReturnsSendError(t *testing.T) {
	wantErr := errors.New("send failed")
	target := &recordingReplyTarget{errAt: 1, err: wantErr}
	rules := []KeywordRule{{
		Keyword:       "hello",
		MatchMode:     MatchExact,
		CaseSensitive: true,
		Areas:         []TriggerArea{AreaFriend},
		ReplyType:     ReplyText,
		Contents:      []string{"world"},
	}}

	matched, outcome, err := processKeywordMessage(rules, target, AreaFriend, "hello")
	if !matched || !errors.Is(err, wantErr) || outcome.Sent != 0 {
		t.Fatalf("matched=%v outcome=%+v err=%v", matched, outcome, err)
	}
	if !strings.Contains(err.Error(), "hello") || !strings.Contains(err.Error(), string(ReplyText)) {
		t.Fatalf("error lacks rule context: %v", err)
	}
}

func TestHandleKeywordMessageReportsMarkdownDegradation(t *testing.T) {
	target := &recordingReplyTarget{}
	rules := []KeywordRule{{
		Keyword:       "hello",
		MatchMode:     MatchExact,
		CaseSensitive: true,
		Areas:         []TriggerArea{AreaChannelPrivate},
		ReplyType:     ReplyMarkdown,
		Contents:      []string{"**world**"},
	}}

	matched, outcome, err := processKeywordMessage(rules, target, AreaChannelPrivate, "hello")
	if err != nil || !matched || !outcome.Degraded || outcome.Sent != 1 {
		t.Fatalf("matched=%v outcome=%+v err=%v", matched, outcome, err)
	}
}

func TestKeywordTargetForArea(t *testing.T) {
	bee := &BeeAPI{ctx: &RobotContext{}}
	tests := []struct {
		area TriggerArea
		kind int
	}{
		{area: AreaFriend, kind: targetFriend},
		{area: AreaGroup, kind: targetGroup},
		{area: AreaChannel, kind: targetChannel},
		{area: AreaChannelPrivate, kind: targetChannelDM},
	}

	for _, test := range tests {
		t.Run(string(test.area), func(t *testing.T) {
			target := keywordTargetForArea(bee, test.area, "source-id")
			if target == nil || target.kind != test.kind || target.targetID != "source-id" {
				t.Fatalf("target=%+v", target)
			}
		})
	}
}
