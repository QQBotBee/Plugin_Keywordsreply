package main

import "strings"

func MatchKeywordRule(rules []KeywordRule, area TriggerArea, message string) (KeywordRule, bool) {
	message = strings.TrimSpace(message)

	for _, rule := range rules {
		if !ruleMatchesArea(rule, area) {
			continue
		}

		keyword := rule.Keyword
		comparisonMessage := message
		if !rule.CaseSensitive {
			keyword = strings.ToLower(keyword)
			comparisonMessage = strings.ToLower(comparisonMessage)
		}

		switch rule.MatchMode {
		case MatchExact:
			if comparisonMessage == keyword {
				return rule, true
			}
		case MatchFuzzy:
			if strings.Contains(comparisonMessage, keyword) {
				return rule, true
			}
		}
	}

	return KeywordRule{}, false
}

func ruleMatchesArea(rule KeywordRule, area TriggerArea) bool {
	for _, ruleArea := range rule.Areas {
		if ruleArea == area {
			return true
		}
	}
	return false
}
