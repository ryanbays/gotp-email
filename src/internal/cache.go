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

// DecodeRawEmailBody parses a raw MIME email and returns the decoded HTML body.
func DecodeRawEmailBody(raw string) (string, error) {
	logrus.Trace("parsing raw email message")

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return "", err
	}

	contentType := msg.Header.Get("Content-Type")
	logrus.WithField("content_type", contentType).Trace("parsed email content type")

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Not multipart, decode directly.
		logrus.Trace("email is not multipart, decoding body directly")
		return decodePart(msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		logrus.WithFields(logrus.Fields{
			"media_type": mediaType,
			"boundary":   params["boundary"],
		}).Trace("email is multipart, iterating parts")

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
			partEncoding := part.Header.Get("Content-Transfer-Encoding")

			logrus.WithFields(logrus.Fields{
				"part_type":    partType,
				"encoding":     partEncoding,
				"content_type": part.Header.Get("Content-Type"),
				"content_disp": part.Header.Get("Content-Disposition"),
			}).Trace("decoding multipart email part")

			body, err := decodePart(
				partEncoding,
				part,
			)
			if err != nil {
				logrus.WithError(err).Trace("failed to decode multipart part")
				continue
			}

			// Strip newlines and carriage returns
			body = strings.ReplaceAll(body, "\r", "")
			body = strings.ReplaceAll(body, "\n", "")

			if partType == "text/html" {
				logrus.Trace("selected html body from multipart email")
				return body, nil
			}

			if partType == "text/plain" && plainText == "" {
				logrus.Trace("captured plain text body from multipart email")
				plainText = body
			}
		}

		logrus.Trace("multipart email had no html body, returning plain text fallback")
		return plainText, nil
	}

	// Single-part email.
	logrus.WithField("encoding", msg.Header.Get("Content-Transfer-Encoding")).Trace("decoding single-part email body")
	return decodePart(msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
}

func decodePart(encoding string, r io.Reader) (string, error) {
	logrus.WithField("encoding", strings.TrimSpace(encoding)).Trace("decoding email part")

	var reader io.Reader = r

	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		reader = quotedprintable.NewReader(r)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		logrus.WithError(err).Trace("failed to copy decoded email part")
		return "", err
	}

	logrus.WithField("bytes", buf.Len()).Trace("decoded email part")
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

	log.Trace("serialising email cache entry")

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithError(err).Error("failed to serialise email cache entry")
		return
	}

	path := filepath.Join(dir, inbox+".cache")
	log.WithField("path", path).Trace("writing email cache file")

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

	log.Trace("loading email cache file")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("no cache file found for inbox")
		} else {
			log.WithError(err).Error("failed to read email cache file")
		}
		return nil, err
	}

	log.WithField("bytes", len(data)).Trace("email cache loaded")
	log.Debug("email cache loaded")
	return data, nil
}

