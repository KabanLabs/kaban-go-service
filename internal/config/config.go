package config

import (
	"flag"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env  string     `yaml:"env" env-default:"local"`
	Http HttpConfig `yaml:"http"`
}

type HttpConfig struct {
	Port int `yaml:"port" env-default:"8888"`
}
type WsConfig struct {
	Port int `yaml:"port" env-default:"5555"`
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
