package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// RuleDraft is the editable form used by the native settings window.
type RuleDraft struct {
	Keyword            string
	MatchMode          MatchMode
	CaseSensitive      bool
	AreaFriend         bool
	AreaGroup          bool
	AreaChannel        bool
	AreaChannelPrivate bool
	ReplyType          ReplyType
	Content            string
}

func NewRuleDraft() RuleDraft {
	return RuleDraft{
		MatchMode:     MatchExact,
		CaseSensitive: true,
		ReplyType:     ReplyText,
	}
}

func RuleSummary(rule KeywordRule) string {
	matchLabel := "精准"
	if rule.MatchMode == MatchFuzzy {
		matchLabel = "模糊"
	}
	replyLabel := map[ReplyType]string{
		ReplyText:     "普通消息",
		ReplyMarkdown: "Markdown",
		ReplyAudio:    "语音",
		ReplyVideo:    "视频",
		ReplyFile:     "文件",
	}[rule.ReplyType]
	if replyLabel == "" {
		replyLabel = string(rule.ReplyType)
	}
	return strings.Join([]string{rule.Keyword, matchLabel, replyLabel, strconv.Itoa(len(rule.Areas)) + " 个区域"}, "｜")
}

func DraftFromRule(rule KeywordRule) RuleDraft {
	draft := NewRuleDraft()
	draft.Keyword = rule.Keyword
	draft.MatchMode = rule.MatchMode
	draft.CaseSensitive = rule.CaseSensitive
	draft.ReplyType = rule.ReplyType
	draft.Content = joinDraftContents(rule)
	for _, area := range rule.Areas {
		switch area {
		case AreaFriend:
			draft.AreaFriend = true
		case AreaGroup:
			draft.AreaGroup = true
		case AreaChannel:
			draft.AreaChannel = true
		case AreaChannelPrivate:
			draft.AreaChannelPrivate = true
		}
	}
	return draft
}

func (draft RuleDraft) Rule() KeywordRule {
	rule := KeywordRule{
		Keyword:       draft.Keyword,
		MatchMode:     draft.MatchMode,
		CaseSensitive: draft.CaseSensitive,
		ReplyType:     draft.ReplyType,
	}
	if draft.AreaFriend {
		rule.Areas = append(rule.Areas, AreaFriend)
	}
	if draft.AreaGroup {
		rule.Areas = append(rule.Areas, AreaGroup)
	}
	if draft.AreaChannel {
		rule.Areas = append(rule.Areas, AreaChannel)
	}
	if draft.AreaChannelPrivate {
		rule.Areas = append(rule.Areas, AreaChannelPrivate)
	}
	if isMediaReply(draft.ReplyType) {
		rule.Contents = MediaItems([]string{draft.Content})
	} else {
		rule.Contents = []string{draft.Content}
	}
	return rule
}

func joinDraftContents(rule KeywordRule) string {
	if isMediaReply(rule.ReplyType) {
		return joinLines(rule.Contents)
	}
	if len(rule.Contents) == 0 {
		return ""
	}
	content := rule.Contents[0]
	if len(rule.Contents) > 1 {
		for _, part := range rule.Contents[1:] {
			content += "\n" + part
		}
	}
	return content
}

func joinLines(contents []string) string {
	items := MediaItems(contents)
	if len(items) == 0 {
		return ""
	}
	content := items[0]
	for _, item := range items[1:] {
		content += "\n" + item
	}
	return content
}

type SettingsController struct {
	store *RuleStore
	rules []KeywordRule
}

func NewSettingsController(store *RuleStore) *SettingsController {
	controller := &SettingsController{store: store}
	if store != nil {
		controller.rules = store.Snapshot()
	}
	return controller
}

func (controller *SettingsController) Rules() []KeywordRule {
	if controller == nil {
		return nil
	}
	return cloneRules(controller.rules)
}

func (controller *SettingsController) Add(draft RuleDraft) error {
	if controller == nil || controller.store == nil {
		return errors.New("设置控制器未初始化")
	}
	rules := controller.Rules()
	rules = append(rules, draft.Rule())
	if err := controller.persist(rules); err != nil {
		return err
	}
	return nil
}

func (controller *SettingsController) Update(index int, draft RuleDraft) error {
	if controller == nil || controller.store == nil {
		return errors.New("设置控制器未初始化")
	}
	if index < 0 || index >= len(controller.rules) {
		return fmt.Errorf("规则索引越界: %d", index)
	}
	rules := controller.Rules()
	rules[index] = draft.Rule()
	return controller.persist(rules)
}

func (controller *SettingsController) Delete(index int) error {
	if controller == nil || controller.store == nil {
		return errors.New("设置控制器未初始化")
	}
	if index < 0 || index >= len(controller.rules) {
		return fmt.Errorf("规则索引越界: %d", index)
	}
	rules := controller.Rules()
	copy(rules[index:], rules[index+1:])
	rules = rules[:len(rules)-1]
	return controller.persist(rules)
}

func (controller *SettingsController) Move(index, delta int) (int, error) {
	if controller == nil || controller.store == nil {
		return index, errors.New("设置控制器未初始化")
	}
	if delta != -1 && delta != 1 {
		return index, errors.New("移动步长必须为 -1 或 1")
	}
	if index < 0 || index >= len(controller.rules) {
		return index, fmt.Errorf("规则索引越界: %d", index)
	}
	newIndex := index + delta
	if newIndex < 0 || newIndex >= len(controller.rules) {
		return index, nil
	}
	rules := controller.Rules()
	rules[index], rules[newIndex] = rules[newIndex], rules[index]
	if err := controller.persist(rules); err != nil {
		return index, err
	}
	return newIndex, nil
}

func (controller *SettingsController) persist(rules []KeywordRule) error {
	if err := controller.store.Replace(rules); err != nil {
		return err
	}
	controller.rules = cloneRules(rules)
	return nil
}
