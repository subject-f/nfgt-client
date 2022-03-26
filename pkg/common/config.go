package common

import (
	"path"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
)

type Config struct {
	SshKey        string `yaml:"config.git.ssh_key"`
	SshPassphrase string `yaml:"config.git.passphrase"`
	RemoteUrl     string `yaml:"config.git.remote_url"`
	RemoteName    string `yaml:"config.git.remote_name"`

	SyncInterval int `yaml:"config.sync_interval"`
}

var defaultValues = map[string]interface{}{
	"config.git.passphrase":  "",
	"config.git.remote_name": "origin",
	"config.sync_interval":   10,
}

func ParseConfig(filepath string) Config {
	Infof("Initializing config from %v\n", filepath)

	k := koanf.New(path.Dir(filepath))

	k.Load(confmap.Provider(defaultValues, "."), nil)

	var config Config

	if err := k.Load(file.Provider(path.Base(filepath)), yaml.Parser()); err != nil {
		Fatalf("Error loading config: %v", err)
	}

	k.UnmarshalWithConf("", &config, koanf.UnmarshalConf{
		Tag:       "yaml",
		FlatPaths: true,
	})

	return config
}
