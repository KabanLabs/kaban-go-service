package config

import (
	"flag"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env  string     `yaml:"env" env-default:"local"`
	Ws   WsConfig   `yaml:"ws"`
	Http HttpConfig `yaml:"http"`
}

type HttpConfig struct {
	EnabledCors bool `yaml:"enable_cors" env-default:"false"`
}

type WsConfig struct {
	Port          int  `yaml:"port" env-default:"5555"`
	WSMaxMessage  int  `yaml:"ws-max-message-size" env-default:"1048576"`
	WSSendBuffer  int  `yaml:"ws-send-buffer-size" env-default:"256"`
	LogDataStream bool `yaml:"log-data-stream" env-default:"false"`
}

func MustLoad() *Config {
	path := fetchConfigPath()

	if path == "" {
		panic("config path is empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config file does not exists: " + path)
	}

	var config Config

	if err := cleanenv.ReadConfig(path, &config); err != nil {
		panic("failed to read config file: " + err.Error())
	}

	return &config
}

// fetchConfigPath fetches config path from cmd flag or env vars
func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
