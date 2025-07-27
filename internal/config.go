package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type OciConfig struct {
	Tenancy              string `json:"tenancy"`
	User                 string `json:"user"`
	Region               string `json:"region"`
	Fingerprint          string `json:"fingerprint"`
	PrivateKey           string `json:"privateKey"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
}

type AppConfig struct {
	OciConfig OciConfig `json:"oci"`
	Zone      string    `json:"zone"`
	Host      string    `json:"host"`
	Hosts     []string  `json:"hosts"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
}

func LoadAppConfig(fileName *string) (AppConfig, error) {
	appConfig := AppConfig{}

	jsonFile, err := os.Open(*fileName)
	if err != nil {
		return appConfig, fmt.Errorf("unable to open config file: %v", err)
	}
	// noinspection GoUnhandledErrorResult
	defer jsonFile.Close()
	jsonBytes, err := io.ReadAll(jsonFile)
	if err != nil {
		return appConfig, fmt.Errorf("unable to read config file: %v", err)
	}
	if err := json.Unmarshal(jsonBytes, &appConfig); err != nil {
		return appConfig, fmt.Errorf("cannot parse the config file: %v", err)
	}
	if appConfig.Host != "" {
		if len(appConfig.Hosts) == 0 {
			appConfig.Hosts = []string{appConfig.Host}
		} else {
			return appConfig, fmt.Errorf("either 'host' or 'hosts' must be set, not both")
		}
	}
	if len(appConfig.Hosts) == 0 {
		return appConfig, fmt.Errorf("at least one host must be set")
	}
	return appConfig, nil
}
