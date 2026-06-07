package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

func SaveEmailCache(dir string, inbox string, email any) {
	log := logrus.WithFields(logrus.Fields{
		"inbox": inbox,
		"dir":   dir,
	})

	data := map[string]any{
		"timestamp": time.Now().Unix(),
		"email":     email,
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithError(err).Error("failed to serialise email cache entry")
		return
	}

	path := filepath.Join(dir, inbox+".cache")
	if err := os.WriteFile(path, b, 0644); err != nil {
		log.WithError(err).WithField("path", path).Error("failed to write email cache file")
		return
	}

	log.WithField("path", path).Debug("email cache saved")
}

func LoadEmailCache(dir string, inbox string) ([]byte, error) {
	path := filepath.Join(dir, inbox+".cache")

	log := logrus.WithFields(logrus.Fields{
		"inbox": inbox,
		"path":  path,
	})

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("no cache file found for inbox")
		} else {
			log.WithError(err).Error("failed to read email cache file")
		}
		return nil, err
	}

	log.Debug("email cache loaded")
	return data, nil
}
