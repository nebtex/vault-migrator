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
    "sync"
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
    //Source Backend
    From *Backend `json:"from"`
    //Destination Backend
    To *Backend `json:"to"`
    //Schedule (optional)
    Schedule *string `json:"schedule"`
    //Queue Size (optional)
    QueueSize int `json:"queuesize"`
    //Number of Workers (optional)
    Workers int `json:"workers"`
}

// Recurse through 'from' backend and add keys to a queue for processing by worker processes
func populateKeyQueue(path string, from physical.Backend, keyQueue chan string) error {
    logrus.Infoln("listing keys : ", path)
    keys, err := from.List(path)
    if err != nil {
        return err
    }
    for _, key := range keys {
        if strings.HasSuffix(key, "/") {
            logrus.Infoln(": ", path+key)
            err := populateKeyQueue(path+key, from, keyQueue)
            if err != nil {
                return err
            }
            continue
        }
        logrus.Infoln("adding key to queue: ", path+key)
        keyQueue <- path + key
    }
    return nil
}

// Retrieve keys from the queue until the channel is closed
func processKeyQueue(id int, from physical.Backend, to physical.Backend, keyQueue chan string, workerWaitGroup sync.WaitGroup) {
    logrus.Infoln("Worker ", id, " starting")
    defer workerWaitGroup.Done()
    for {
        key, more := <-keyQueue
        if more {
            moveKey(key, from, to)
        } else {
            break
        }
    }
    logrus.Infoln("Worker ", id, " done")
}

func moveKey(key string, from physical.Backend, to physical.Backend) error {
    logrus.Infoln("moving key: ", key)
    entry, err := from.Get(key)
    if err != nil {
        return err
    }

    if entry != nil {
        err = to.Put(entry)
        if err != nil {
            return err
        }
    } else {
        logrus.Infoln("key not found: ", key)
    }

    return nil
}

func move(config *Config) error {
    logger := log.New("vault-migrator")
    var keyQueue = make(chan string, config.QueueSize)
    from, err := newBackend(config.From.Name, logger, config.From.Config, )
    if err != nil {
        return err
    }
    to, err := newBackend(config.To.Name, logger, config.To.Config)
    if err != nil {
        return err
    }
    logrus.Infoln("Starting ", config.Workers, " workers...")
    var workerWaitGroup sync.WaitGroup
    workerWaitGroup.Add(config.Workers)
    for id := 1; id <= config.Workers; id++ {
        go processKeyQueue(id, from, to, keyQueue, workerWaitGroup)
    }
    logrus.Infoln("Starting to move keys")
    populateKeyQueue("", from, keyQueue)
    logrus.Infoln("All keys populated, notifying workers")
    close(keyQueue)
    logrus.Infoln("Waiting for workers to finish")
    workerWaitGroup.Wait()
    logrus.Infoln("All workers finished")
    return nil
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
        if config.QueueSize <= 0 {
            // Default queue size to 10000
            config.QueueSize = 10000
        }
        if config.Workers <= 0 {
            // Default workers to 1
            config.Workers = 1
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
