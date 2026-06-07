package internal

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

type Candidate struct {
	Code  string
	Score int
}

func ExtractOTP(sender string, html string) string {
	log := logrus.WithField("sender", sender)

	service := MatchService(sender)
	if service == nil {
		log.Debug("no matching service rule found for sender")
		return ""
	}

	log.WithField("service", service.Name).Debug("matched service rule")

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.WithError(err).Error("failed to parse email HTML")
		return ""
	}

	var candidates []Candidate

	for _, step := range service.Steps {
		re := CompileRegex(step.Regex)
		stepLog := log.WithFields(logrus.Fields{
			"selector": step.Selector,
			"regex":    step.Regex,
		})

		doc.Find(step.Selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			log.WithField("text", text).Debug("checking element for OTP candidates")
			matches := re.FindAllString(text, -1)

			if len(matches) > 0 {
				stepLog.WithField("matches", matches).Debug("regex matches found in element")
			}

			for _, code := range matches {
				score := step.Score
				if len(code) == 6 {
					score += 5
				}
				candidates = append(candidates, Candidate{
					Code:  code,
					Score: score,
				})
			}
		})

		stepLog.WithField("candidates_so_far", len(candidates)).Debug("step complete")
	}

	best := ""
	bestScore := -1
	for _, c := range candidates {
		if c.Score > bestScore {
			bestScore = c.Score
			best = c.Code
		}
	}

	if best == "" {
		log.WithField("service", service.Name).Debug("no otp candidates found after all steps")
	} else {
		log.WithFields(logrus.Fields{
			"service": service.Name,
			"score":   bestScore,
		}).Info("otp extracted")
	}

	return best
}
