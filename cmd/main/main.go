package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	eos "github.com/eoscanada/eos-go"
	shipclient "github.com/eosswedenorg-go/antelope-ship-client"
	"github.com/eosswedenorg-go/pid"
	"github.com/eosswedenorg/thalos/api/message"
	_ "github.com/eosswedenorg/thalos/api/message/json"
	_ "github.com/eosswedenorg/thalos/api/message/msgpack"
	api_redis "github.com/eosswedenorg/thalos/api/redis"
	"github.com/eosswedenorg/thalos/app"
	"github.com/eosswedenorg/thalos/app/abi"
	"github.com/eosswedenorg/thalos/app/config"
	. "github.com/eosswedenorg/thalos/app/log"
	"github.com/go-redis/redis/v8"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/telegram"
	"github.com/pborman/getopt/v2"
	log "github.com/sirupsen/logrus"
)

// ---------------------------
//  Global variables
// ---------------------------

var conf *config.Config

var shClient *shipclient.Stream

var running bool = false

func readerLoop() {
	running = true
	recon_cnt := 0

	for running {
		recon_cnt++
		log.Infof("Connecting to ship at: %s (Try %d)", conf.Ship.Url, recon_cnt)
		if err := shClient.Connect(conf.Ship.Url); err != nil {
			log.WithError(err).Error("Failed to connect")

			if recon_cnt >= 3 {
				msg := fmt.Sprintf("Failed to connect to ship at '%s'", conf.Ship.Url)
				if err := notify.Send(context.Background(), conf.Name, msg); err != nil {
					log.WithError(err).Error("Failed to send notification")
				}
				recon_cnt = 0
			}

			log.Info("Trying again in 5 seconds ....")
			time.Sleep(5 * time.Second)
			continue
		}

		if err := shClient.SendBlocksRequest(); err != nil {
			log.WithError(err).Error("Failed to send block request")
			continue
		}

		recon_cnt = 0
		log.Infof("Connected, Start: %d, End: %d", shClient.StartBlock, shClient.EndBlock)
		log.WithError(shClient.Run()).Error("Failed to read from ship")
	}
}

func run() {
	// Spawn reader loop in another thread.
	go readerLoop()

	// Create interrupt channel.
	signals := make(chan os.Signal, 1)

	// Register signal channel to receive signals from the os.
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt
	sig := <-signals
	log.WithField("signal", sig).Info("Signal received")

	// Cleanly close the connection by sending a close message.
	err := shClient.Shutdown()
	if err != nil {
		log.WithError(err).Info("failed to send close message to ship server")
	}

	running = false
}

func init() {
	// Initialize logger
	formatter := log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.0000",
	}

	log.SetFormatter(&formatter)
}

func main() {
	var err error
	var chainInfo *eos.InfoResp

	showHelp := getopt.BoolLong("help", 'h', "display this help text")
	showVersion := getopt.BoolLong("version", 'v', "display this help text")
	configFile := getopt.StringLong("config", 'c', "./config.yml", "Config file to read", "file")
	pidFile := getopt.StringLong("pid", 'p', "", "Where to write process id", "file")
	logFile := getopt.StringLong("log", 'l', "", "Path to log file", "file")

	getopt.Parse()

	if *showHelp {
		getopt.Usage()
		return
	}

	if *showVersion {
		fmt.Println("v0.0.0")
		return
	}

	// Write PID file
	if len(*pidFile) > 0 {
		log.Infof("Writing pid to: %s", *pidFile)
		err = pid.Save(*pidFile)
		if err != nil {
			log.WithError(err).Fatal("failed to write pid file")
			return
		}
	}

	// Parse config
	conf, err = config.Load(*configFile)
	if err != nil {
		log.WithError(err).Fatal("failed to read config file")
		return
	}

	// If log file is given on the commandline, override config values.
	if len(*logFile) > 0 {
		conf.Log.Directory = path.Dir(*logFile)
		conf.Log.Filename = path.Base(*logFile)
	}

	if len(conf.Log.Filename) > 0 {
		writer, err := NewRotatingFileFromConfig(conf.Log)
		if err != nil {
			log.WithError(err).Fatal("Failed to open log")
			return
		}
		log.WithFields(log.Fields{
			"maxfilesize": conf.Log.MaxFileSize,
			"maxage":      conf.Log.MaxTime,
			"directory":   conf.Log.GetDirectory(),
			"filename":    conf.Log.GetFilename(),
		}).Info("Logging to file: ", conf.Log.GetFilePath())
		log.SetOutput(writer)
	}

	// Init telegram notification service
	telegram, err := telegram.New(conf.Telegram.Id)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize telegram")
		return
	}

	telegram.AddReceivers(conf.Telegram.Channel)

	// Register services in notification manager
	notify.UseServices(telegram)

	// Connect to redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     conf.Redis.Addr,
		Password: conf.Redis.Password,
		DB:       conf.Redis.DB,
	})

	err = rdb.Ping(context.Background()).Err()
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to redis")
		return
	}

	log.Printf("Get chain info from api at: %s", conf.Api)
	eosClient := eos.New(conf.Api)
	chainInfo, err = eosClient.GetInfo(context.Background())
	if err != nil {
		log.WithError(err).Fatal("Failed to get info")
		return
	}

	if conf.Ship.StartBlockNum == shipclient.NULL_BLOCK_NUMBER {
		if conf.Ship.IrreversibleOnly {
			conf.Ship.StartBlockNum = uint32(chainInfo.LastIrreversibleBlockNum)
		} else {
			conf.Ship.StartBlockNum = uint32(chainInfo.HeadBlockNum)
		}
	}

	shClient = shipclient.NewStream(func(s *shipclient.Stream) {
		s.StartBlock = conf.Ship.StartBlockNum
		s.EndBlock = conf.Ship.EndBlockNum
		s.IrreversibleOnly = conf.Ship.IrreversibleOnly
	})

	// Get codec
	codec, err := message.GetCodec(conf.MessageCodec)
	if err != nil {
		log.WithError(err).Fatal("Failed to load codec")
		return
	}

	processor := app.SpawnProccessor(
		shClient,
		api_redis.NewPublisher(rdb, api_redis.Namespace{
			Prefix:  conf.Redis.Prefix,
			ChainID: chainInfo.ChainID.String(),
		}),
		abi.NewAbiManager(rdb, eosClient, conf.Redis.CacheID),
		codec,
	)

	// Run the application
	run()

	// Close the processor properly
	processor.Close()
}
