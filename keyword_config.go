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
		ruleNumber := i + 1
		if rule.Keyword == "" {
			return fmt.Errorf("第 %d 条规则：关键词不能为空", ruleNumber)
		}
		if _, exists := keywords[rule.Keyword]; exists {
			return fmt.Errorf("第 %d 条规则：关键词“%s”重复", ruleNumber, rule.Keyword)
		}
		keywords[rule.Keyword] = struct{}{}

		if rule.MatchMode != MatchExact && rule.MatchMode != MatchFuzzy {
			return fmt.Errorf("第 %d 条规则：匹配模式“%s”无效", ruleNumber, rule.MatchMode)
		}
		if len(rule.Areas) == 0 {
			return fmt.Errorf("第 %d 条规则：至少选择一个触发区域", ruleNumber)
		}
		for _, area := range rule.Areas {
			if !isValidTriggerArea(area) {
				return fmt.Errorf("第 %d 条规则：触发区域“%s”无效", ruleNumber, area)
			}
		}
		if !isValidReplyType(rule.ReplyType) {
			return fmt.Errorf("第 %d 条规则：回复类型“%s”无效", ruleNumber, rule.ReplyType)
		}
		if !hasContent(rule.Contents) {
			return fmt.Errorf("第 %d 条规则：回复内容不能为空", ruleNumber)
		}
		if rule.ReplyType == ReplyText {
			if _, err := ParseTextReply(strings.Join(rule.Contents, "\n")); err != nil {
				return fmt.Errorf("第 %d 条规则：普通消息内容无效：%w", ruleNumber, err)
			}
		}
		if isMediaReply(rule.ReplyType) {
			for _, area := range rule.Areas {
				if area == AreaChannel || area == AreaChannelPrivate {
					return fmt.Errorf("第 %d 条规则：%s回复不能用于频道消息或频道私信", ruleNumber, replyTypeChineseName(rule.ReplyType))
				}
			}
		}
	}
	return nil
}

func replyTypeChineseName(replyType ReplyType) string {
	switch replyType {
	case ReplyText:
		return "普通消息"
	case ReplyMarkdown:
		return "Markdown"
	case ReplyAudio:
		return "语音"
	case ReplyVideo:
		return "视频"
	case ReplyFile:
		return "文件"
	default:
		return string(replyType)
	}
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
		return fmt.Errorf("读取规则配置失败：%w", err)
	}

	var rules []KeywordRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("解析规则配置失败：%w", err)
	}
	if err := ValidateRules(rules); err != nil {
		return fmt.Errorf("规则配置校验失败：%w", err)
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
		return fmt.Errorf("生成规则配置失败：%w", err)
	}
	data = append(data, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(store.path), ".keyword_replies-*.tmp")
	if err != nil {
		return fmt.Errorf("创建规则配置临时文件失败：%w", err)
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
		return fmt.Errorf("写入规则配置临时文件失败：%w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("同步规则配置临时文件失败：%w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭规则配置临时文件失败：%w", err)
	}
	if err := store.replaceFile(temporaryPath, store.path); err != nil {
		return fmt.Errorf("替换规则配置文件失败：%w", err)
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
