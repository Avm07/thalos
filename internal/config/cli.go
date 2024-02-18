package config

import (
	"path"
	"time"

	shipclient "github.com/eosswedenorg-go/antelope-ship-client"
	"github.com/spf13/pflag"
)

func GetFlags() *pflag.FlagSet {
	flags := pflag.FlagSet{}

	// Cli only flags
	flags.StringP("config", "c", "./config.yml", "Config file to read")
	flags.StringP("level", "L", "info", "Log level to use")
	flags.StringP("pid", "p", "", "Where to write process id")
	flags.BoolP("no-state-cache", "n", false, "Force the application to take start block from config/api")

	// Generic
	flags.StringP("url", "u", "", "Url to antelope api")
	flags.String("codec", "json", "Codec used to send messages")

	// Redis
	flags.String("redis-addr", "127.0.0.1:6379", "host:port to redis server")
	flags.String("redis-user", "", "Redis username")
	flags.String("redis-password", "", "Redis password")
	flags.Int("redis-db", 0, "Redis database")
	flags.String("redis-prefix", "ship", "Redis channel prefix")

	// Telegram
	flags.String("telegram-id", "", "Id of telegram bot")
	flags.Int64("telegram-channel", 0, "Telegram channel to send notifications to")

	// Log
	flags.StringP("log", "l", "", "Path to log file (default: print to stdout/stderr)")
	flags.String("log-max-filesize", "10mb", "Max filesize for logfile to rotate")
	flags.Duration("log-max-time", time.Hour*24, "Max time for logfile to rotate")

	// Ship
	flags.String("ship-url", "ws://127.0.0.1:8080", "Url to ship node")
	flags.Uint32("start-block", shipclient.NULL_BLOCK_NUMBER, "Start to stream from this block")
	flags.Uint32("end-block", shipclient.NULL_BLOCK_NUMBER, "Stop streaming when this block is reached")

	flags.Lookup("start-block").DefValue = "config value, cache, head from api"
	flags.Lookup("end-block").DefValue = "none"

	flags.Bool("irreversible-only", false, "Only stream irreversible blocks from ship")
	flags.Int("max-msg-in-flight", 10, "Maximum messages that can be sent from SHIP without acknowledgement")
	flags.String("chain", "", "ChainID used in channel namespace, can be any string (default from api)")

	return &flags
}

func overrideCliFlags(cfg *Config, flags *pflag.FlagSet) error {
	logFile, _ := flags.GetString("log")
	if len(logFile) > 0 {
		cfg.Log.Directory = path.Dir(logFile)
		cfg.Log.Filename = path.Base(logFile)
	}
	return nil
}
