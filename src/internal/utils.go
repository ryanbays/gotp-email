package internal

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

var otpRegexCache = map[string]*regexp.Regexp{}

func CompileRegex(pattern string) *regexp.Regexp {
	if r, ok := otpRegexCache[pattern]; ok {
		logrus.WithField("pattern", pattern).Debug("regex cache hit")
		return r
	}

	logrus.WithField("pattern", pattern).Debug("compiling and caching regex")
	r := regexp.MustCompile(pattern)
	otpRegexCache[pattern] = r
	return r
}

func ExtractInboxID(email string) string {
	log := logrus.WithField("email", email)

	parts := strings.Split(email, "@")
	if len(parts) < 2 {
		log.Warn("could not extract inbox id: no @ found in address")
		return ""
	}

	inbox := strings.ToLower(strings.TrimSpace(parts[0]))
	log.WithField("inbox", inbox).Debug("extracted inbox id")
	return inbox
}
