package cmd

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/InazumaV/V2bX/conf"
	vCore "github.com/InazumaV/V2bX/core"
	"github.com/InazumaV/V2bX/limiter"
	"github.com/InazumaV/V2bX/node"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	config string
	watch  bool
)

var serverCommand = cobra.Command{
	Use:   "sjy",
	Short: "开始启动--->",
	Run:   serverHandle,
	Args:  cobra.NoArgs,
}

func init() {
	serverCommand.PersistentFlags().
		StringVarP(&config, "config", "c",
			"/root/config.json", "config file path")
	serverCommand.PersistentFlags().
		BoolVarP(&watch, "watch", "w",
			true, "watch file path change")
	command.AddCommand(&serverCommand)
}

func serverHandle(_ *cobra.Command, _ []string) {
	showVersion()
	c := conf.New()
	err := c.LoadFromPath(config)
	if err != nil {
		log.WithField("err", err).Error("加载配置文件失败")
		return
	}
	switch c.LogConfig.Level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
	if c.LogConfig.Output != "" {
		w := &lumberjack.Logger{
			Filename:   c.LogConfig.Output,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
		log.SetOutput(w)
	}
	limiter.Init()
	log.Info("数据传输开始****")
	vc, err := vCore.NewCore(c.CoresConfig)
	if err != nil {
		log.WithField("err", err).Error("new core failed")
		return
	}
	err = vc.Start()
	if err != nil {
		log.WithField("err", err).Error("Start core failed")
		return
	}
	defer vc.Close()
	log.Info("内核 ", vc.Type(), " *")
	nodes := node.New()
	err = nodes.Start(c.NodeConfig, vc)
	if err != nil {
		log.WithField("err", err).Error("运行节点失败")
		return
	}
	log.Info("节点启动")
	xdns := os.Getenv("XRAY_DNS_PATH")
	sdns := os.Getenv("SING_DNS_PATH")
	if watch {
		err = c.Watch(config, xdns, sdns, func() {
			nodes.Close()
			err = vc.Close()
			if err != nil {
				log.WithField("err", err).Error("重启节点失败")
				return
			}
			vc, err = vCore.NewCore(c.CoresConfig)
			if err != nil {
				log.WithField("err", err).Error("新核心失败")
				return
			}
			err = vc.Start()
			if err != nil {
				log.WithField("err", err).Error("Start core failed")
				return
			}
			log.Info("Core ", vc.Type(), " restarted")
			err = nodes.Start(c.NodeConfig, vc)
			if err != nil {
				log.WithField("err", err).Error("Run nodes failed")
				return
			}
			log.Info("Nodes restarted")
			runtime.GC()
		})
		if err != nil {
			log.WithField("err", err).Error("start watch failed")
			return
		}
	}
	// clear memory
	runtime.GC()
	// wait exit signal
	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
		<-osSignals
	}
}
