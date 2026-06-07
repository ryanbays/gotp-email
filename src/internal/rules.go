package internal

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type RulesFile struct {
	Services []ServiceRule `json:"services"`
}

type ServiceRule struct {
	Name        string     `json:"name"`
	MatchSender string     `json:"match_sender"`
	Steps       []RuleStep `json:"steps"`
}

type RuleStep struct {
	Selector string `json:"selector"`
	Regex    string `json:"regex"`
	Score    int    `json:"score"`
}

var Rules RulesFile

func LoadRules(path string) error {
	log := logrus.WithField("path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		log.WithError(err).Error("failed to read rules file")
		return err
	}

	if err := json.Unmarshal(data, &Rules); err != nil {
		log.WithError(err).Error("failed to parse rules file")
		return err
	}

	log.WithField("service_count", len(Rules.Services)).Info("rules loaded")

	for _, svc := range Rules.Services {
		logrus.WithFields(logrus.Fields{
			"name":         svc.Name,
			"match_sender": svc.MatchSender,
			"steps":        len(svc.Steps),
		}).Debug("registered service rule")
	}

	return nil
}

func MatchService(sender string) *ServiceRule {
	log := logrus.WithField("sender", sender)
	sender = strings.ToLower(sender)

	for i := range Rules.Services {
		r := Rules.Services[i]
		if r.MatchSender == "" {
			continue
		}
		if strings.Contains(sender, strings.ToLower(r.MatchSender)) {
			log.WithField("service", r.Name).Debug("sender matched service rule")
			return &Rules.Services[i]
		}
	}

	log.Debug("no direct sender match, trying default fallback")

	for i := range Rules.Services {
		if Rules.Services[i].Name == "default" {
			log.Debug("using default service rule")
			return &Rules.Services[i]
		}
	}

	log.Warn("no matching service rule and no default defined")
	return nil
}
