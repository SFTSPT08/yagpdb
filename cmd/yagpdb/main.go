package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/web"
)

var (
	flagRunBot    bool
	flagRunWeb    bool
	flagRunReddit bool
	flagRunStats  bool

	flagAction string

	flagRunEverything bool
	flagLogTimestamp  bool
	flagAddr          string
	flagConfig        string

	BotSession *discordgo.Session
	RedisPool  *pool.Pool
)

func init() {
	flag.BoolVar(&flagRunBot, "bot", false, "Set to run discord bot")
	flag.BoolVar(&flagRunWeb, "web", false, "Set to run webserver")
	flag.BoolVar(&flagRunReddit, "reddit", false, "Set to run reddit bot")
	flag.BoolVar(&flagRunStats, "stats", false, "Set to update stats")
	flag.BoolVar(&flagRunEverything, "all", false, "Set to everything (discord bot, webserver and reddit bot)")

	flag.BoolVar(&flagLogTimestamp, "ts", false, "Set to include timestamps in log")
	flag.StringVar(&flagAddr, "addr", ":5000", "Address for webserver to listen on")
	flag.StringVar(&flagConfig, "conf", "config.json", "Path to config file")
	flag.StringVar(&flagAction, "a", "", "Run a action and exit, available actions: connected")
	flag.Parse()
}

func main() {

	log.AddHook(common.ContextHook{})

	if flagLogTimestamp {
		web.LogRequestTimestamps = true
	}

	if !flagRunBot && !flagRunWeb && !flagRunReddit && !flagRunEverything && flagAction == "" {
		log.Error("Didnt specify what to run, see -h for more info")
		return
	}

	log.Info("YAGPDB is initializing...")
	config, err := common.LoadConfig(flagConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed loading config")
	}

	BotSession, err = discordgo.New(config.BotToken)
	if err != nil {
		log.WithError(err).Fatal("Failed initilizing bot session")
	}

	BotSession.MaxRestRetries = 3
	//BotSession.LogLevel = discordgo.LogInformational

	RedisPool, err = pool.NewCustomPool("tcp", config.Redis, 100, common.RedisDialFunc)
	if err != nil {
		log.WithError(err).Fatal("Failed initilizing redis pool")
	}

	if flagAction != "" {
		runAction(flagAction)
		return
	}

	common.RedisPool = RedisPool
	common.Conf = config
	common.BotSession = BotSession
	// common.Pastebin = &pastebin.Pastebin{DevKey: config.PastebinDevKey}

	// Setup plugins
	commands.RegisterPlugin()
	serverstats.RegisterPlugin()
	notifications.RegisterPlugin()
	customcommands.RegisterPlugin()
	reddit.RegisterPlugin()
	moderation.RegisterPlugin()
	reputation.RegisterPlugin()
	aylien.RegisterPlugin()
	streaming.RegisterPlugin()

	// Setup plugins for bot, but run later if enabled
	bot.Setup()

	// RUN FORREST RUN
	if flagRunWeb || flagRunEverything {
		web.ListenAddress = flagAddr
		go web.Run()
	}

	if flagRunBot || flagRunEverything {
		go bot.Run()
	}

	if flagRunReddit || flagRunEverything {
		go reddit.RunReddit()
	}

	if flagRunStats || flagRunEverything {
		go serverstats.UpdateStatsLoop()
	}

	select {}
}

func runAction(str string) {
	log.Info("Running action", str)
	client, err := RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed to get redis connection")
		return
	}
	defer RedisPool.CarefullyPut(client, &err)

	switch str {
	case "connected":
		err = common.RefreshConnectedGuilds(BotSession, client)
	default:
		log.Error("Unknown action")
		return
	}

	if err != nil {
		log.WithError(err).Error("Error running action")
	} else {
		log.Info("Sucessfully ran action", str)
	}
}
