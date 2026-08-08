package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type MatchMode string

const (
	MatchExact MatchMode = "exact"
	MatchFuzzy MatchMode = "fuzzy"
)

type TriggerArea string

const (
	AreaFriend         TriggerArea = "friend"
	AreaGroup          TriggerArea = "group"
	AreaChannel        TriggerArea = "channel"
	AreaChannelPrivate TriggerArea = "channel_private"
)

type ReplyType string

const (
	ReplyText     ReplyType = "text"
	ReplyMarkdown ReplyType = "markdown"
	ReplyAudio    ReplyType = "audio"
	ReplyVideo    ReplyType = "video"
	ReplyFile     ReplyType = "file"
)

type KeywordRule struct {
	Keyword       string        `json:"keyword"`
	MatchMode     MatchMode     `json:"match_mode"`
	CaseSensitive bool          `json:"case_sensitive"`
	Areas         []TriggerArea `json:"areas"`
	ReplyType     ReplyType     `json:"reply_type"`
	Contents      []string      `json:"contents"`
}

func ValidateRules(rules []KeywordRule) error {
	keywords := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		if rule.Keyword == "" {
			return fmt.Errorf("rule %d: keyword is required", i)
		}
		if _, exists := keywords[rule.Keyword]; exists {
			return fmt.Errorf("rule %d: duplicate keyword %q", i, rule.Keyword)
		}
		keywords[rule.Keyword] = struct{}{}

		if rule.MatchMode != MatchExact && rule.MatchMode != MatchFuzzy {
			return fmt.Errorf("rule %d: invalid match mode %q", i, rule.MatchMode)
		}
		if len(rule.Areas) == 0 {
			return fmt.Errorf("rule %d: at least one area is required", i)
		}
		for _, area := range rule.Areas {
			if !isValidTriggerArea(area) {
				return fmt.Errorf("rule %d: invalid trigger area %q", i, area)
			}
		}
		if !isValidReplyType(rule.ReplyType) {
			return fmt.Errorf("rule %d: invalid reply type %q", i, rule.ReplyType)
		}
		if !hasContent(rule.Contents) {
			return fmt.Errorf("rule %d: reply content is required", i)
		}
		if rule.ReplyType == ReplyText {
			if _, err := ParseTextReply(strings.Join(rule.Contents, "\n")); err != nil {
				return fmt.Errorf("rule %d: invalid text reply: %w", i, err)
			}
		}
		if isMediaReply(rule.ReplyType) {
			for _, area := range rule.Areas {
				if area == AreaChannel || area == AreaChannelPrivate {
					return fmt.Errorf("rule %d: %s replies cannot use channel areas", i, rule.ReplyType)
				}
			}
		}
	}
	return nil
}

func isValidTriggerArea(area TriggerArea) bool {
	return area == AreaFriend || area == AreaGroup || area == AreaChannel || area == AreaChannelPrivate
}

func isValidReplyType(replyType ReplyType) bool {
	return replyType == ReplyText || replyType == ReplyMarkdown || replyType == ReplyAudio || replyType == ReplyVideo || replyType == ReplyFile
}

func isMediaReply(replyType ReplyType) bool {
	return replyType == ReplyAudio || replyType == ReplyVideo || replyType == ReplyFile
}

func hasContent(contents []string) bool {
	for _, content := range contents {
		if strings.TrimSpace(content) != "" {
			return true
		}
	}
	return false
}

type RuleStore struct {
	mu          sync.RWMutex
	path        string
	rules       []KeywordRule
	replaceFile func(string, string) error
}

func NewRuleStore(path string) *RuleStore {
	return &RuleStore{
		path:        path,
		replaceFile: replaceFileAtomically,
	}
}

func (store *RuleStore) Load() error {
	store.mu.Lock()
	defer store.mu.Unlock()

	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		store.rules = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("read keyword rules: %w", err)
	}

	var rules []KeywordRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("decode keyword rules: %w", err)
	}
	if err := ValidateRules(rules); err != nil {
		return fmt.Errorf("validate keyword rules: %w", err)
	}
	store.rules = cloneRules(rules)
	return nil
}

func (store *RuleStore) Replace(rules []KeywordRule) error {
	copiedRules := cloneRules(rules)
	if err := ValidateRules(copiedRules); err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	data, err := json.MarshalIndent(copiedRules, "", "  ")
	if err != nil {
		return fmt.Errorf("encode keyword rules: %w", err)
	}
	data = append(data, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(store.path), ".keyword_replies-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary keyword rules file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary keyword rules file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary keyword rules file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary keyword rules file: %w", err)
	}
	if err := store.replaceFile(temporaryPath, store.path); err != nil {
		return fmt.Errorf("replace keyword rules file: %w", err)
	}

	removeTemporary = false
	store.rules = copiedRules
	return nil
}

func (store *RuleStore) Snapshot() []KeywordRule {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return cloneRules(store.rules)
}

func cloneRules(rules []KeywordRule) []KeywordRule {
	if rules == nil {
		return nil
	}
	copied := make([]KeywordRule, len(rules))
	for i, rule := range rules {
		copied[i] = rule
		copied[i].Areas = append([]TriggerArea(nil), rule.Areas...)
		copied[i].Contents = append([]string(nil), rule.Contents...)
	}
	return copied
}
