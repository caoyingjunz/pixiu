package services

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type Conf struct {
	Host     string `yaml:host`
	User     string `yaml:user`
	Password string `yaml:password`
}

func GetConf() Conf {

	file, err := os.Open("config/config.yml")
	if err != nil {
		panic(err)
	}
	bytes, err := ioutil.ReadAll(file)

	if err != nil {
		panic(err)
	}

	cfg := Conf{}
	err = yaml.Unmarshal(bytes, &cfg)
	if err != nil {
		panic(err)
	}
	return cfg
}
