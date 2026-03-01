package service

import (
	"NewAPI-Gateway/common"
	"net/url"
	"regexp"
	"strings"
)

var sensitiveKVRegex = regexp.MustCompile(`(?i)(password|token|secret|passphrase)=([^\s&]+)`)

func redactBackupMessage(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	output := sensitiveKVRegex.ReplaceAllString(trimmed, "$1=***")
	if parsed, err := url.Parse(output); err == nil && parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			username = "***"
		}
		parsed.User = url.UserPassword(username, "***")
		output = parsed.String()
	}
	return output
}

func logBackupInfo(message string) {
	commonMessage := redactBackupMessage(message)
	if commonMessage == "" {
		return
	}
	common.SysLog("backup: " + commonMessage)
}

func logBackupError(message string) {
	commonMessage := redactBackupMessage(message)
	if commonMessage == "" {
		return
	}
	common.SysError("backup: " + commonMessage)
}
