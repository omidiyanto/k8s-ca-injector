package config

import (
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// SetupConfig initializes viper config and returns the config object and a
// configured logger. The caller should use the returned logger everywhere so
// logs are consistent across packages.
func SetupConfig() (*viper.Viper, *logrus.Logger) {
	lg := logrus.New()

	cfg := viper.New()
	cfg.AddConfigPath(".")
	cfg.AddConfigPath("$HOME/.config")
	cfg.AddConfigPath("/etc/ca-injector")

	cfg.SetConfigName("ca-injector")

	cfg.AutomaticEnv()
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	cfg.SetDefault("tls.key", "/cert/tls.key")
	cfg.SetDefault("tls.crt", "/cert/tls.crt")
	cfg.SetDefault("tls.ca.key", "ca.crt")
	cfg.SetDefault("shutdown.timeout", 10*time.Second)

	if err := cfg.ReadInConfig(); err != nil {
		lg.WithError(err).Error("could not read initial config")
	}

	cfg.OnConfigChange(func(_ fsnotify.Event) {
		if err := cfg.ReadInConfig(); err != nil {
			lg.WithError(err).Warn("could not reload config")
		}
	})
	if os.Getenv("KUBERNETES_SERVICE_PORT") != "" {
		lg.SetFormatter(&logrus.JSONFormatter{})
	}

	go cfg.WatchConfig()

	return cfg, lg
}
