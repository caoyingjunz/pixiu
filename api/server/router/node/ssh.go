package node

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type AuthModel int8

const (
	PASSWORD AuthModel = iota + 1
	PUBLICKEY
)

type SSHClientConfig struct {
	AuthModel AuthModel
	Host      string
	Port      int
	User      string
	Password  string
	KeyPath   string
	Protocol  string
	Timeout   time.Duration
}

func SSHClientConfigPassword(config *WebSSHConfig) *SSHClientConfig {
	return &SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: PASSWORD,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		Password:  config.Password,
		Protocol:  config.Protocol,
	}
}

func SSHClientConfigPulicKey(config *WebSSHConfig) *SSHClientConfig {
	return &SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: PUBLICKEY,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		KeyPath:   config.PkPath,
		Protocol:  config.Protocol,
	}
}

func NewSSHClient(conf *SSHClientConfig) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		Timeout:         conf.Timeout,
		User:            conf.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 忽略know_hosts检查
	}
	switch conf.AuthModel {
	case PASSWORD:
		config.Auth = []ssh.AuthMethod{ssh.Password(conf.Password)}
	case PUBLICKEY:
		signer, err := getKey(conf.KeyPath)
		if err != nil {
			return nil, err
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	c, err := ssh.Dial(conf.Protocol, fmt.Sprintf("%s:%d", conf.Host, conf.Port), config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func getKey(keyPath string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}
