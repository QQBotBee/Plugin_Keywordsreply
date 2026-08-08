package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewRuleDraftDefaultsCaseSensitive(t *testing.T) {
	draft := NewRuleDraft()
	if !draft.CaseSensitive || draft.MatchMode != MatchExact || draft.ReplyType != ReplyText {
		t.Fatalf("unexpected defaults: %+v", draft)
	}
}

func TestRuleDraftRoundTrip(t *testing.T) {
	want := KeywordRule{
		Keyword:       "菜单",
		MatchMode:     MatchFuzzy,
		CaseSensitive: false,
		Areas:         []TriggerArea{AreaFriend, AreaGroup},
		ReplyType:     ReplyText,
		Contents:      []string{"请选择\n下一步"},
	}
	got := DraftFromRule(want).Rule()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestRuleDraftMediaSplitsContent(t *testing.T) {
	draft := NewRuleDraft()
	draft.Keyword = "音频"
	draft.ReplyType = ReplyAudio
	draft.AreaFriend = true
	draft.Content = " first.mp3\r\n\n second.mp3 "
	got := draft.Rule()
	if !reflect.DeepEqual(got.Contents, []string{"first.mp3", "second.mp3"}) {
		t.Fatalf("contents=%#v", got.Contents)
	}
}

func TestSettingsControllerMutationsPersist(t *testing.T) {
	store := NewRuleStore(t.TempDir() + "\\keyword_replies.json")
	controller := NewSettingsController(store)
	first := NewRuleDraft()
	first.Keyword = "one"
	first.AreaFriend = true
	first.Content = "first"
	if err := controller.Add(first); err != nil {
		t.Fatal(err)
	}
	second := NewRuleDraft()
	second.Keyword = "two"
	second.AreaGroup = true
	second.Content = "second"
	if err := controller.Add(second); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(controller.Rules(), store.Snapshot()) {
		t.Fatalf("controller=%#v store=%#v", controller.Rules(), store.Snapshot())
	}
	if err := controller.Delete(0); err != nil {
		t.Fatal(err)
	}
	if got := controller.Rules(); len(got) != 1 || got[0].Keyword != "two" {
		t.Fatalf("after delete=%#v", got)
	}
}

func TestSettingsControllerRejectsDuplicateWithoutMutation(t *testing.T) {
	store := NewRuleStore(t.TempDir() + "\\keyword_replies.json")
	controller := NewSettingsController(store)
	for _, keyword := range []string{"one", "two"} {
		draft := NewRuleDraft()
		draft.Keyword = keyword
		draft.AreaFriend = true
		draft.Content = keyword
		if err := controller.Add(draft); err != nil {
			t.Fatal(err)
		}
	}
	want := controller.Rules()
	duplicate := NewRuleDraft()
	duplicate.Keyword = "one"
	duplicate.AreaGroup = true
	duplicate.Content = "changed"
	if err := controller.Update(1, duplicate); err == nil {
		t.Fatal("expected duplicate error")
	}
	if got := controller.Rules(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mutated after rejection: got=%#v want=%#v", got, want)
	}
}

func TestSettingsControllerRejectsMediaChannel(t *testing.T) {
	store := NewRuleStore(t.TempDir() + "\\keyword_replies.json")
	controller := NewSettingsController(store)
	draft := NewRuleDraft()
	draft.Keyword = "file"
	draft.ReplyType = ReplyFile
	draft.AreaChannel = true
	draft.Content = "file.zip"
	if err := controller.Add(draft); err == nil || !strings.Contains(err.Error(), "channel") {
		t.Fatalf("err=%v", err)
	}
	if len(controller.Rules()) != 0 {
		t.Fatalf("unexpected rules=%#v", controller.Rules())
	}
}

func TestSettingsControllerMoveAndBoundaries(t *testing.T) {
	store := NewRuleStore(t.TempDir() + "\\keyword_replies.json")
	controller := NewSettingsController(store)
	for _, keyword := range []string{"one", "two", "three"} {
		draft := NewRuleDraft()
		draft.Keyword = keyword
		draft.AreaFriend = true
		draft.Content = keyword
		if err := controller.Add(draft); err != nil {
			t.Fatal(err)
		}
	}
	if index, err := controller.Move(1, -1); err != nil || index != 0 {
		t.Fatalf("up index=%d err=%v", index, err)
	}
	if got := controller.Rules(); got[0].Keyword != "two" {
		t.Fatalf("after up=%#v", got)
	}
	if index, err := controller.Move(2, 1); err != nil || index != 2 {
		t.Fatalf("down boundary index=%d err=%v", index, err)
	}
	if index, err := controller.Move(0, 0); err == nil || index != 0 {
		t.Fatalf("invalid delta index=%d err=%v", index, err)
	}
}
