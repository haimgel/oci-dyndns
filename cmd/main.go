package main

import (
	"flag"
	"github.com/haimgel/oci-dyndns/internal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func createLogger() *zap.SugaredLogger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build(zap.AddStacktrace(zap.ErrorLevel), zap.WithCaller(false))
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}

func main() {
	logger := createLogger()
	defer func() { _ = logger.Sync() }()

	configFileName := flag.String("config", "config.json", "Configuration file name")
	listenAddress := flag.String("listen", ":8080", "Address and port to listen to")
	flag.Parse()

	appConfig, err := internal.LoadAppConfig(configFileName)
	if err != nil {
		logger.Fatal(err)
	}
	service, err := internal.NewService(&appConfig, logger)
	if err != nil {
		logger.Fatal(err)
	}
	err = service.Serve(listenAddress)
	if err != nil {
		logger.Fatal(err)
	}
}
