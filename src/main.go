package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/ryanbays/gotp-email/internal"
	"github.com/sirupsen/logrus"
	"github.com/toorop/gin-logrus"
)

var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

const OTP_TTL = time.Hour

type EmailRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	Raw  string `json:"raw"`
}

type OTPPayload struct {
	OTP       string `json:"otp"`
	From      string `json:"from"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	logLevel, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		logLevel = "debug"
		gin.SetMode(gin.DebugMode)
	}
	ll, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatalf("invalid log level: %s", logLevel)
	}
	logrus.SetLevel(ll)

	configPath := flag.String("config", "/etc/gotp-email/gotp.conf", "Path to config file")
	rulesPath := flag.String("rules", "/etc/gotp-email/rules.json", "Path to rules JSON file")
	flag.Parse()

	logrus.WithFields(logrus.Fields{
		"config": *configPath,
		"rules":  *rulesPath,
	}).Debug("starting with config")

	if *configPath != "" {
		logrus.WithField("path", *configPath).Info("loading config file")
		internal.LoadConfig(*configPath)
	} else {
		logrus.Warn("no config path provided, using defaults")
		internal.AppConfig = internal.Config{
			CacheDir: "./cache",
			ApiKey:   "",
		}
	}

	logrus.WithField("cache_dir", internal.AppConfig.CacheDir).Debug("ensuring cache directory exists")
	err = os.MkdirAll(internal.AppConfig.CacheDir, 0755)
	if err != nil {
		logrus.WithError(err).WithField("dir", internal.AppConfig.CacheDir).Fatal("failed to create cache directory")
	}

	logrus.WithField("path", *rulesPath).Info("loading rules file")
	if err := internal.LoadRules(*rulesPath); err != nil {
		logrus.WithError(err).Fatal("failed to load rules")
	}

	r := gin.New()
	r.Use(ginlogrus.Logger(logrus.StandardLogger()), gin.Recovery())
	r.SetTrustedProxies(internal.AppConfig.TrustedProxies)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	protected := r.Group("/", apiKeyAuth(internal.AppConfig.ApiKey))
	protected.POST("/inbound-email", inbound)
	protected.GET("/otp/:inbox", getOTP)
	protected.GET("/otp/:inbox/history", history)
	protected.GET("/email-cache/:inbox", emailCache)

	logrus.Info("server listening on :8080")
	if err := r.Run(":8080"); err != nil {
		logrus.WithError(err).Fatal("server exited unexpectedly")
	}
}

func apiKeyAuth(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			logrus.Error("api key is not configured")
			c.AbortWithStatusJSON(500, gin.H{"error": "api key not configured"})
			return
		}

		if c.GetHeader("X-API-Key") != expected {
			logrus.WithField("ip", c.ClientIP()).Warn("rejected request with invalid api key")
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}

func inbound(c *gin.Context) {
	var req EmailRequest
	if err := c.BindJSON(&req); err != nil {
		logrus.WithError(err).Warn("failed to parse inbound email request")
		c.JSON(400, gin.H{"error": "bad request"})
		return
	}

	inbox := internal.ExtractInboxID(req.To)
	logrus.WithFields(logrus.Fields{
		"inbox": inbox,
		"from":  req.From,
		"to":    req.To,
	}).Debug("received inbound email")

	internal.SaveEmailCache(internal.AppConfig.CacheDir, inbox, req)
	logrus.WithField("inbox", inbox).Debug("saved email to cache")

	otp := internal.ExtractOTP(req.From, req.Raw)
	if otp == "" {
		logrus.WithFields(logrus.Fields{
			"inbox": inbox,
			"from":  req.From,
		}).Debug("no otp found in email")
		c.JSON(200, gin.H{"status": "ok", "message": "no otp found"})
		return
	}

	logrus.WithFields(logrus.Fields{
		"inbox": inbox,
		"from":  req.From,
	}).Info("otp extracted from email")

	payload := OTPPayload{
		OTP:       otp,
		From:      req.From,
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(payload)

	if err := rdb.Set(ctx, "otp:"+inbox, data, OTP_TTL).Err(); err != nil {
		logrus.WithError(err).WithField("inbox", inbox).Error("failed to store otp in redis")
	}
	if err := rdb.LPush(ctx, "otp_history:"+inbox, data).Err(); err != nil {
		logrus.WithError(err).WithField("inbox", inbox).Error("failed to push otp to history")
	}
	rdb.LTrim(ctx, "otp_history:"+inbox, 0, 10)

	c.JSON(200, gin.H{
		"status": "ok",
		"inbox":  inbox,
		"otp":    otp,
	})
}

func getOTP(c *gin.Context) {
	inbox := c.Param("inbox")
	logrus.WithField("inbox", inbox).Debug("otp lookup requested")

	val, err := rdb.Get(ctx, "otp:"+inbox).Result()
	if err != nil {
		logrus.WithField("inbox", inbox).Debug("no otp found in redis for inbox")
		c.JSON(200, gin.H{"otp": nil})
		return
	}

	var p OTPPayload
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		logrus.WithError(err).WithField("inbox", inbox).Error("failed to deserialise otp payload")
		c.JSON(500, gin.H{"error": "internal error"})
		return
	}

	logrus.WithField("inbox", inbox).Debug("otp returned successfully")
	c.JSON(200, p)
}

func history(c *gin.Context) {
	inbox := c.Param("inbox")
	logrus.WithField("inbox", inbox).Debug("otp history requested")

	items, err := rdb.LRange(ctx, "otp_history:"+inbox, 0, 10).Result()
	if err != nil {
		logrus.WithError(err).WithField("inbox", inbox).Error("failed to fetch otp history from redis")
	}

	var out []OTPPayload
	for _, i := range items {
		var p OTPPayload
		if err := json.Unmarshal([]byte(i), &p); err != nil {
			logrus.WithError(err).WithField("inbox", inbox).Warn("skipping malformed otp history entry")
			continue
		}
		out = append(out, p)
	}

	logrus.WithFields(logrus.Fields{
		"inbox": inbox,
		"count": len(out),
	}).Debug("returning otp history")
	c.JSON(200, gin.H{"history": out})
}

func emailCache(c *gin.Context) {
	inbox := c.Param("inbox")
	logrus.WithField("inbox", inbox).Debug("email cache requested")

	data, err := internal.LoadEmailCache(internal.AppConfig.CacheDir, inbox)
	if err != nil {
		logrus.WithError(err).WithField("inbox", inbox).Debug("no cached email found for inbox")
		c.JSON(200, gin.H{"email": nil})
		return
	}

	logrus.WithField("inbox", inbox).Debug("returning cached email")
	c.Data(http.StatusOK, "application/json", data)
}
