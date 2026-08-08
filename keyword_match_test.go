package main

import "testing"

func TestMatchKeywordRule(t *testing.T) {
	tests := []struct {
		name    string
		rules   []KeywordRule
		area    TriggerArea
		message string
		want    string
		wantOK  bool
	}{
		{
			name:    "trims message edges for exact match",
			rules:   []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaFriend}}},
			area:    AreaFriend,
			message: "  hello \t",
			want:    "hello",
			wantOK:  true,
		},
		{
			name:    "does not trim stored keyword",
			rules:   []KeywordRule{{Keyword: " hello ", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaFriend}}},
			area:    AreaFriend,
			message: "hello",
			wantOK:  false,
		},
		{
			name:    "matches fuzzy keyword contained in message",
			rules:   []KeywordRule{{Keyword: "help", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}}},
			area:    AreaGroup,
			message: "please help me",
			want:    "help",
			wantOK:  true,
		},
		{
			name:    "does not ignore case for case sensitive rule",
			rules:   []KeywordRule{{Keyword: "Hello", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaFriend}}},
			area:    AreaFriend,
			message: "hello",
			wantOK:  false,
		},
		{
			name:    "ignores case when explicitly configured",
			rules:   []KeywordRule{{Keyword: "Hello", MatchMode: MatchExact, CaseSensitive: false, Areas: []TriggerArea{AreaFriend}}},
			area:    AreaFriend,
			message: "hello",
			want:    "Hello",
			wantOK:  true,
		},
		{
			name:    "skips rules outside active area",
			rules:   []KeywordRule{{Keyword: "hello", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}}},
			area:    AreaFriend,
			message: "hello",
			wantOK:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := MatchKeywordRule(test.rules, test.area, test.message)
			if ok != test.wantOK || (ok && got.Keyword != test.want) {
				t.Fatalf("MatchKeywordRule() = (%#v, %v), want keyword %q and matched %v", got, ok, test.want, test.wantOK)
			}
		})
	}
}

func TestMatchKeywordRuleUsesListOrder(t *testing.T) {
	rules := []KeywordRule{
		{Keyword: "你好", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}},
		{Keyword: "你好呀", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}},
	}

	got, ok := MatchKeywordRule(rules, AreaGroup, " 你好呀 ")
	if !ok || got.Keyword != "你好" {
		t.Fatalf("MatchKeywordRule() = (%#v, %v), want first matching rule", got, ok)
	}
}
