package internal

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type Config struct {
	CacheDir       string
	ApiKey         string
	TrustedProxies []string
}

var AppConfig Config

func LoadConfig(path string) error {
	log := logrus.WithField("path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		log.WithError(err).Error("failed to read config file")
		return err
	}

	log.Debug("config file read, parsing")

	lines := string(data)
	for _, line := range splitLines(lines) {
		if line == "" || line[0] == '#' {
			continue
		}

		parts := splitKeyValue(line)
		if len(parts) != 2 {
			logrus.WithField("line", line).Warn("skipping malformed config line")
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "CACHE_DIR":
			AppConfig.CacheDir = value
			logrus.WithField("cache_dir", value).Debug("config: set CACHE_DIR")
		case "API_KEY":
			AppConfig.ApiKey = value
			logrus.Debug("config: set API_KEY")
		case "TRUSTED_PROXIES":
			AppConfig.TrustedProxies = strings.Split(value, ",")
			logrus.WithField("proxies", AppConfig.TrustedProxies).Debug("config: set TRUSTED_PROXIES")
		default:
			logrus.WithField("key", key).Warn("unrecognised config key, ignoring")
		}
	}

	logrus.WithFields(logrus.Fields{
		"cache_dir":       AppConfig.CacheDir,
		"trusted_proxies": AppConfig.TrustedProxies,
		"api_key_set":     AppConfig.ApiKey != "",
	}).Info("config loaded")

	return nil
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else if r != '\r' {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitKeyValue(s string) []string {
	var parts []string
	current := ""
	for _, r := range s {
		if r == '=' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
