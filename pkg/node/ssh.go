package node

import (
	"fmt"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func SSHClientConfigPassword(config *types.WebSSHConfig) *model.SSHClientConfig {
	return &model.SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: model.PASSWORD,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		Password:  config.Password,
		Protocol:  config.Protocol,
	}
}

func SSHClientConfigPulicKey(config *types.WebSSHConfig) *model.SSHClientConfig {
	return &model.SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: model.PUBLICKEY,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		KeyPath:   config.PkPath,
		Protocol:  config.Protocol,
	}
}

func NewSSHClient(conf *model.SSHClientConfig) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		Timeout:         conf.Timeout,
		User:            conf.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //忽略know_hosts检查
	}

	switch conf.AuthModel {
	case model.PASSWORD:
		config.Auth = []ssh.AuthMethod{ssh.Password(conf.Password)}
	case model.PUBLICKEY:
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
