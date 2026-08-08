package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var imageMarkerPattern = regexp.MustCompile(`\[图片=([^\]\r\n]*)\]`)

// ParsedTextReply is an ordinary reply split into its text and optional image.
type ParsedTextReply struct {
	Text  string
	Image string
}

// ParseTextReply parses the optional [图片=...] marker in an ordinary reply.
func ParseTextReply(content string) (ParsedTextReply, error) {
	matches := imageMarkerPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) > 1 {
		return ParsedTextReply{}, errors.New("普通回复只能包含一个图片标记")
	}
	if len(matches) == 0 {
		return ParsedTextReply{Text: strings.TrimSpace(content)}, nil
	}

	match := matches[0]
	image := strings.TrimSpace(content[match[2]:match[3]])
	if image == "" {
		return ParsedTextReply{}, errors.New("图片标记中的地址不能为空")
	}
	text := strings.TrimSpace(content[:match[0]] + content[match[1]:])
	return ParsedTextReply{Text: text, Image: image}, nil
}

// MediaItems splits media configuration lines, trims them, and removes blanks.
func MediaItems(contents []string) []string {
	var items []string
	for _, content := range contents {
		for _, line := range strings.FieldsFunc(content, func(r rune) bool { return r == '\r' || r == '\n' }) {
			if item := strings.TrimSpace(line); item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}

type keywordReplyTarget interface {
	SendText(string) (string, error)
	SendImage(string, string) (string, error)
	SendMarkdown(string) (string, error)
	SendAudio(string) (string, error)
	SendVideo(string) (string, error)
	SendFile(string) (string, error)
}

// ReplyOutcome records how much of a rule was sent and whether it was degraded.
type ReplyOutcome struct {
	Sent     int
	Degraded bool
}

// SendKeywordReply dispatches one configured reply to its selected target.
func SendKeywordReply(target keywordReplyTarget, area TriggerArea, rule KeywordRule) (ReplyOutcome, error) {
	if target == nil {
		return ReplyOutcome{}, errors.New("回复目标不能为空")
	}

	content := strings.Join(rule.Contents, "\n")
	switch rule.ReplyType {
	case ReplyText:
		parsed, err := ParseTextReply(content)
		if err != nil {
			return ReplyOutcome{}, err
		}
		if parsed.Image != "" {
			_, err = target.SendImage(parsed.Text, parsed.Image)
		} else {
			_, err = target.SendText(parsed.Text)
		}
		if err != nil {
			return ReplyOutcome{}, err
		}
		return ReplyOutcome{Sent: 1}, nil
	case ReplyMarkdown:
		outcome := ReplyOutcome{}
		var err error
		if area == AreaChannelPrivate {
			outcome.Degraded = true
			_, err = target.SendText(content)
		} else {
			_, err = target.SendMarkdown(content)
		}
		if err != nil {
			return outcome, err
		}
		outcome.Sent = 1
		return outcome, nil
	case ReplyAudio:
		if !isFriendOrGroupArea(area) {
			return ReplyOutcome{}, fmt.Errorf("%s replies are only supported for friend or group areas", rule.ReplyType)
		}
		return sendMediaItems(target.SendAudio, MediaItems(rule.Contents))
	case ReplyVideo:
		if !isFriendOrGroupArea(area) {
			return ReplyOutcome{}, fmt.Errorf("%s replies are only supported for friend or group areas", rule.ReplyType)
		}
		return sendMediaItems(target.SendVideo, MediaItems(rule.Contents))
	case ReplyFile:
		if !isFriendOrGroupArea(area) {
			return ReplyOutcome{}, fmt.Errorf("%s replies are only supported for friend or group areas", rule.ReplyType)
		}
		return sendMediaItems(target.SendFile, MediaItems(rule.Contents))
	default:
		return ReplyOutcome{}, fmt.Errorf("不支持的回复类型 %q", rule.ReplyType)
	}
}

func isFriendOrGroupArea(area TriggerArea) bool {
	return area == AreaFriend || area == AreaGroup
}

func sendMediaItems(send func(string) (string, error), items []string) (ReplyOutcome, error) {
	outcome := ReplyOutcome{}
	for _, item := range items {
		if _, err := send(item); err != nil {
			return outcome, err
		}
		outcome.Sent++
	}
	return outcome, nil
}
