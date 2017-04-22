package main

import (
    log "github.com/mgutz/logxi/v1"
    "github.com/Sirupsen/logrus"
    "github.com/hashicorp/vault/physical"
    "github.com/urfave/cli"
    "os"
    "io/ioutil"
    "encoding/json"
    "fmt"
    "strings"
    "github.com/robfig/cron"
    "time"
)

//Backend is a supported storage backend by vault
type Backend struct {
    //Use the same name that is used in the vault config file
    Name   string `json:"name"`
    //Put here the configuration of your picked backend
    Config map[string]string `json:"config"`
}

//Config config.json structure
type Config struct {
    //Source
    From     *Backend `json:"from"`
    //Destination
    To       *Backend `json:"to"`
    //Schedule (optional)
    Schedule *string  `json:"schedule"`
}

func moveData(path string, from physical.Backend, to physical.Backend) error {
    keys, err := from.List(path)
    if err != nil {
        return err
    }
    for _, key := range keys {
        logrus.Infoln("moving key: ", path + key)
        if strings.HasSuffix(key, "/") {
            err := moveData(path + key, from, to)
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
        logrus.Info("all the keys have been moved ")
    }
    return nil
}

func move(config *Config) error {
    logger := log.New("vault-migrator")
    from, err := physical.NewBackend(config.From.Name, logger, config.From.Config)
    if err != nil {
        return err
    }
    to, err := physical.NewBackend(config.To.Name, logger, config.To.Config)
    if err != nil {
        return err
    }
    return moveData("", from, to)
}

func main() {
    app := cli.NewApp()
    app.Name = "vault-migrator"
    app.Usage = ""
    app.Version = version
    app.Authors = []cli.Author{{"nebtex", "publicdev@nebtex.com"}}
    app.Flags = []cli.Flag{cli.StringFlag{
        Name: "config, c",
        Value: "",
        Usage: "config file",
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
            return move(config)
        }
        cr := cron.New()
        err = cr.AddFunc(*config.Schedule, func() {
            defer func() {
                err := recover()
                if err != nil {
                    logrus.Errorln(err)
                }
            }()
            err = move(config)
            if err != nil {
                logrus.Errorln(err)
            }
        })
        if err != nil {
            return err
        }
        cr.Start()
        //make initial migration
        err = move(config)
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
