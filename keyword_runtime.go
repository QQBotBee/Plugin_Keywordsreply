package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var keywordStoreState struct {
	sync.RWMutex
	store *RuleStore
}

func initializeKeywordRules(bee *BeeAPI) error {
	if bee == nil {
		return fmt.Errorf("Bee API 不能为空")
	}
	dataDir, err := bee.GetAppDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建插件数据目录: %w", err)
	}
	store := NewRuleStore(filepath.Join(dataDir, "keyword_replies.json"))
	loadErr := store.Load()
	keywordStoreState.Lock()
	keywordStoreState.store = store
	keywordStoreState.Unlock()
	return loadErr
}

func currentKeywordStore() *RuleStore {
	keywordStoreState.RLock()
	defer keywordStoreState.RUnlock()
	return keywordStoreState.store
}

func processKeywordMessage(rules []KeywordRule, target keywordReplyTarget, area TriggerArea, message string) (bool, ReplyOutcome, error) {
	rule, ok := MatchKeywordRule(rules, area, message)
	if !ok {
		return false, ReplyOutcome{}, nil
	}
	outcome, err := SendKeywordReply(target, area, rule)
	if err != nil {
		return true, outcome, fmt.Errorf("规则 %q（%s）发送失败: %w", rule.Keyword, rule.ReplyType, err)
	}
	return true, outcome, err
}

func keywordTargetForArea(bee *BeeAPI, area TriggerArea, targetID string) *MessageTarget {
	if bee == nil {
		return nil
	}
	switch area {
	case AreaFriend:
		return bee.Friend(targetID)
	case AreaGroup:
		return bee.Group(targetID)
	case AreaChannel:
		return bee.Channel(targetID)
	case AreaChannelPrivate:
		return bee.ChannelDM(targetID)
	default:
		return nil
	}
}

func handleKeywordMessage(bee *BeeAPI, area TriggerArea, targetID, message string) {
	store := currentKeywordStore()
	if bee == nil || store == nil {
		return
	}
	target := keywordTargetForArea(bee, area, targetID)
	if target == nil {
		return
	}
	_, outcome, err := processKeywordMessage(store.Snapshot(), target, area, message)
	if err != nil {
		_ = bee.Log(fmt.Sprintf("关键词回复发送失败（区域=%s）：%v", area, err))
		return
	}
	if outcome.Degraded {
		_ = bee.Log("频道私信 Markdown 回复已降级为普通消息")
	}
}
