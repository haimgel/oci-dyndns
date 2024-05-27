package main

import (
	"flag"
	"github.com/haimgel/oci-dyndns/internal"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	configFileName := flag.String("config", "config.json", "Configuration file name")
	listenAddress := flag.String("listen", ":8080", "Address and port to listen to")
	flag.Parse()

	appConfig, err := internal.LoadAppConfig(configFileName)
	if err != nil {
		logger.Error("Application load error", "err", err)
		os.Exit(1)
	}
	service, err := internal.NewService(&appConfig, logger)
	if err != nil {
		logger.Error("Service creation error", "err", err)
		os.Exit(1)
	}
	err = service.Serve(listenAddress)
	if err != nil {
		logger.Error("Cannot start HTTP service", "err", err)
		os.Exit(1)
	}
}
