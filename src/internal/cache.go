package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// decodeRawEmailBody parses a raw MIME email and returns the decoded HTML body.
// If no HTML part exists, it falls back to plain text.
func DecodeRawEmailBody(raw string) (string, error) {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return "", err
	}

	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Not multipart, decode directly.
		return decodePart(msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])

		var plainText string

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			partType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))

			body, err := decodePart(
				part.Header.Get("Content-Transfer-Encoding"),
				part,
			)
			if err != nil {
				continue
			}

			if partType == "text/html" {
				return body, nil
			}

			if partType == "text/plain" && plainText == "" {
				plainText = body
			}
		}

		return plainText, nil
	}

	// Single-part email.
	return decodePart(msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
}

func decodePart(encoding string, r io.Reader) (string, error) {
	var reader io.Reader = r

	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		reader = quotedprintable.NewReader(r)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return "", err
	}

	return buf.String(), nil
}

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
