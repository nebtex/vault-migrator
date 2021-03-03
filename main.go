package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	vaultcli "github.com/hashicorp/vault/cli"
	vaultcommand "github.com/hashicorp/vault/command"
	"github.com/hashicorp/vault/physical"
	log "github.com/mgutz/logxi/v1"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var backendFactories map[string]physical.Factory

func init() {
	// fish the backend factories out of the vault CLI, since that is inexplicably where
	// this map is assembled
	vaultCommands := vaultcli.Commands(nil)
	cmd, err := vaultCommands["server"]()
	if err != nil {
		logrus.Fatal("'vault server' init failed", err)
	}
	serverCommand, ok := cmd.(*vaultcommand.ServerCommand)
	if !ok {
		logrus.Fatal("'vault server' did not return a ServerCommand")
	}
	backendFactories = serverCommand.PhysicalBackends
}

func newBackend(kind string, logger log.Logger, conf map[string]string) (physical.Backend, error) {
	if factory := backendFactories[kind]; factory == nil {
		return nil, fmt.Errorf("no Vault backend is named %+q", kind)
	} else {
		return factory(conf, logger)
	}
}

//Backend is a supported storage backend by vault
type Backend struct {
	//Use the same name that is used in the vault config file
	Name string `json:"name"`
	//Put here the configuration of your picked backend
	Config map[string]string `json:"config"`
}

//Config config.json structure
type Config struct {
	//Source
	From *Backend `json:"from"`
	//Destination
	To *Backend `json:"to"`
	//Schedule (optional)
	Schedule *string `json:"schedule"`
}

func copyData(path string, from physical.Backend, to physical.Backend) error {
	keys, err := from.List(path)
	if err != nil {
		return err
	}
	for _, key := range keys {
		logrus.Infoln("copying key: ", path+key)
		if strings.HasSuffix(key, "/") {
			err := copyData(path+key, from, to)
			if err != nil {
				return err
			}
			continue
		}
		entry, err := from.Get(path + key)
		if err != nil {
			return err
		}
		if entry == nil {
			continue
		}
		err = to.Put(entry)

		if err != nil {
			return err
		}
	}
	if path == "" {
		logrus.Info("all the keys have been copied ")
	}
	return nil
}

func copy(config *Config) error {
	logger := log.New("vault-migrator")

	from, err := newBackend(config.From.Name, logger, config.From.Config)
	if err != nil {
		return err
	}
	to, err := newBackend(config.To.Name, logger, config.To.Config)
	if err != nil {
		return err
	}
	return copyData("", from, to)
}

func main() {
	app := cli.NewApp()
	app.Name = "vault-migrator"
	app.Usage = ""
	app.Version = version
	app.Authors = []cli.Author{{"nebtex", "publicdev@nebtex.com"}}
	app.Flags = []cli.Flag{cli.StringFlag{
		Name:   "config, c",
		Value:  "",
		Usage:  "config file",
		EnvVar: "VAULT_MIGRATOR_CONFIG_FILE",
	}}

	app.Action = func(c *cli.Context) error {
		configFile := c.String("config")
		configRaw, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		config := &Config{}
		err = json.Unmarshal(configRaw, config)
		if err != nil {
			return err
		}
		if config.From == nil {
			return fmt.Errorf("%v", "Please define a source (key: from)")
		}
		if config.To == nil {
			return fmt.Errorf("%v", "Please define a destination (key: to)")
		}
		if config.Schedule == nil {
			return copy(config)
		}
		cr := cron.New()
		err = cr.AddFunc(*config.Schedule, func() {
			defer func() {
				err := recover()
				if err != nil {
					logrus.Errorln(err)
				}
			}()
			err = copy(config)
			if err != nil {
				logrus.Errorln(err)
			}
		})
		if err != nil {
			return err
		}
		cr.Start()
		//make initial migration
		err = copy(config)
		if err != nil {
			return err
		}
		for {
			time.Sleep(time.Second * 60)
			logrus.Info("Waiting the next schedule")

		}

	}
	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
