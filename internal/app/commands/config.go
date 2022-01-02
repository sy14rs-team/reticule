package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/durp/reticule/pkg/coinbasepro"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type configCmd struct {
	Create createConfigCmd `kong:"cmd,name='create',help='create a new config'"`
	Delete deleteConfigCmd `kong:"cmd,name='delete',help='delete a config'"`
	Update updateConfigCmd `kong:"cmd,name='update',help='update an existing config'"`
}

type createConfigCmd struct {
	Coinbase createReticuleConfigCmd `kong:"cmd,name='reticule',alias='r',help='create a new reticule config'"`
}

type deleteConfigCmd struct {
	Coinbase deleteReticuleConfigCmd `kong:"cmd,name='reticule',alias='r',help='delete an existing reticule config'"`
}

type updateConfigCmd struct {
	Coinbase updateReticuleConfigCmd `kong:"cmd,name='reticule',alias='cb',help='update an existing reticule config'"`
}

type reticuleConfigSet struct {
	Current string
	Configs map[string]reticuleConfig
}

type reticuleConfig struct {
	BaseURL      string
	FeedURL      string
	Auth         *coinbasepro.Auth
	ServerIP     string
	ServerPort   int
	ServerSecret string
}

type createReticuleConfigCmd struct {
	Name        string   `kong:"name='name',short='n',help='name of config',required"`
	BaseURL     *url.URL `kong:"name='base-url',short='b',default='https://api-public.sandbox.pro.coinbase.com',help='url of coinbasepro api that provided key'"`
	FeedURL     *url.URL `kong:"name='feed-url',short='f',default='wss://ws-feed-public.sandbox.pro.coinbase.com',help='url of websocket feed'"`
	Key         string   `kong:"name='key',short='k',help='coinbasepro provided api key'"`
	Passphrase  string   `kong:"name='passphrase',short='p',help='coinbasepro api passphrase'"`
	Secret      string   `kong:"name='secret',short='s',help='coinbasepro provided api secret'"`
	Use         bool     `kong:"name='use',short='u',help='set as config to use'"`
	ServerPort  int      `kong:"name='port',short='t',default='80',help='port to use in server mode'"`
	BindAddress string   `kong:"name='bind-address',short='l',default='127.0.0.1',help='ip address to use in server mode'"`
	ServerAuth  string   `kong:"name='server-auth',short='a',default='default',help='Pre-shared secret for auth in server mode'"`
}

func (c *createReticuleConfigCmd) Run(fs afero.Fs) (capture error) {
	configPath, err := configPath()
	if err != nil {
		return err
	}
	var configSet reticuleConfigSet
	f, err := fs.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0755)
	switch {
	case errors.Is(err, os.ErrNotExist):
		cfgDir := path.Dir(configPath)
		err = fs.MkdirAll(cfgDir, os.ModePerm)
		if err != nil {
			return err
		}
		fmt.Printf("creating config %q\n", configPath)
		f, err = fs.Create(configPath)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		defer func() { coinbasepro.Capture(&capture, f.Close()) }()
		b, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(b, &configSet)
		if err != nil {
			return err
		}
		if _, ok := configSet.Configs[c.Name]; ok {
			return fmt.Errorf("reticule config %q already exists, use `config update reticule` to modify an existing config", c.Name)
		}
	}
	cfg := reticuleConfig{
		BaseURL: c.BaseURL.String(),
		FeedURL: c.FeedURL.String(),
		Auth: coinbasepro.NewAuth(
			c.Key,
			c.Passphrase,
			c.Secret),
		ServerIP:     c.BindAddress,
		ServerPort:   c.ServerPort,
		ServerSecret: c.ServerAuth,
	}
	if configSet.Configs == nil {
		configSet.Configs = make(map[string]reticuleConfig)
		configSet.Current = c.Name
	}
	if c.Use {
		configSet.Current = c.Name
	}
	configSet.Configs[c.Name] = cfg
	enc := yaml.NewEncoder(f)
	err = enc.Encode(&configSet)
	if err != nil {
		return err
	}
	return enc.Close()
}

type deleteReticuleConfigCmd struct {
	Name string `kong:"name='name',short='n',help='name of config',required"`
}

func (d *deleteReticuleConfigCmd) Run(fs afero.Fs) error {
	configPath, err := configPath()
	if err != nil {
		return err
	}
	configSet, err := readConfigSet(fs, configPath)
	if err != nil {
		return err
	}
	delete(configSet.Configs, d.Name)
	return writeConfigSet(fs, configPath, configSet)
}

type updateReticuleConfigCmd struct {
	Name        string   `kong:"name='name',short='n',help='name of config',required"`
	BaseURL     *url.URL `kong:"name='base-url',short='b',help='url of coinbasepro api that provided key'"`
	FeedURL     *url.URL `kong:"name='feed-url',short='f',help='url of websocket feed'"`
	Key         string   `kong:"name='key',short='k',help='coinbasepro provided api key'"`
	Passphrase  string   `kong:"name='passphrase',short='p',help='coinbasepro api passphrase'"`
	Rename      string   `kong:"name='rename',short='r',help='new name for config'"`
	Secret      string   `kong:"name='secret',short='s',help='coinbasepro provided api secret'"`
	Use         bool     `kong:"name='use',short='s',help='set as config to use'"`
	ServerPort  int      `kong:"name='port',short='t',help='port to use in server mode'"`
	BindAddress string   `kong:"name='bind-address',short='l',help='ip address to use in server mode'"`
	ServerAuth  string   `kong:"name='server-auth',short='a',help='Pre-shared secret for auth in server mode'"`
}

func (c *updateReticuleConfigCmd) Run(fs afero.Fs) error {
	configPath, err := configPath()
	if err != nil {
		return err
	}
	configSet, err := readConfigSet(fs, configPath)
	if err != nil {
		return err
	}
	cfg, ok := configSet.Configs[c.Name]
	if !ok {
		return fmt.Errorf("reticule config %q does not exists, use `config create reticule` to create a new config", c.Name)
	}
	if c.BaseURL != nil {
		cfg.BaseURL = c.BaseURL.String()
	}
	if c.FeedURL != nil {
		cfg.FeedURL = c.FeedURL.String()
	}
	if c.Key != "" {
		cfg.Auth.Key = c.Key
	}
	if c.Passphrase != "" {
		cfg.Auth.Passphrase = c.Passphrase
	}
	if c.Secret != "" {
		cfg.Auth.Secret = c.Secret
	}
	if c.Rename != "" {
		delete(configSet.Configs, c.Name)
		c.Name = c.Rename
	}
	if c.Use {
		configSet.Current = c.Name
	}
	if c.BindAddress != "" {
		cfg.ServerIP = c.BindAddress
	}
	if c.ServerPort != 0 {
		cfg.ServerPort = c.ServerPort
	}
	if c.ServerAuth != "" {
		cfg.ServerSecret = c.ServerAuth
	}
	configSet.Configs[c.Name] = cfg
	return writeConfigSet(fs, configPath, configSet)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("no user home directory defined")
	}
	return path.Join(home, ".reticule", "reticule"), nil
}

func readConfigSet(fs afero.Fs, configPath string) (reticuleConfigSet, error) {
	var configSet reticuleConfigSet
	f, err := fs.Open(configPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return reticuleConfigSet{}, fmt.Errorf("config file %q does not exist, create a new config with `create config reticule`", configPath)
	case err != nil:
		return reticuleConfigSet{}, err
	default:
		defer func() { _ = f.Close() }()
		b, err := ioutil.ReadAll(f)
		if err != nil {
			return reticuleConfigSet{}, err
		}
		err = yaml.Unmarshal(b, &configSet)
		if err != nil {
			return reticuleConfigSet{}, err
		}
		return configSet, nil
	}
}

func writeConfigSet(fs afero.Fs, configPath string, configSet reticuleConfigSet) (capture error) {
	f, err := fs.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer func() { coinbasepro.Capture(&capture, f.Close()) }()
	enc := yaml.NewEncoder(f)
	err = enc.Encode(&configSet)
	if err != nil {
		return err
	}
	return enc.Close()
}
