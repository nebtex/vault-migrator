# vault migrator

migrate or backup vault data between two physical backends. in one operation or in a cron job.


tested with:

* vault v0.7
* consul to dynamodb
* dynamodb to consul

# usage

create a `config.json` file with this structure

```go
package main

type Backend struct {
    Name   string `json:"name"`
    Config map[string]string `json:"config"`
}

type Config struct {
    //Source
    From     *Backend `json:"from"`
    //Destination
    To       *Backend `json:"to"`
    //(optional) schedule a cron job
    Schedule *string  `json:"schedule"`
}
```
## examples:


1. from dynamodb to consul

```json
{
  "to": {
    "name": "consul",
      "config": {
        "address": "127.0.0.7:8500",
        "path": "vault",
        "token": "xxxx-xxxx-xxxx-xxxx-xxxxxxxxx"
     }
  },
    "from": {
        "name": "dynamodb",
        "config": {
          "ha_enabled": true,
          "table": "vault",
          "write_capacity": 1,
          "access_key": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
          "secret_key": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
        }
    },
  "schedule": "@daily"
}
```

full list of storage backends and configuration options: [Vault Storage Backend](https://www.vaultproject.io/docs/configuration/storage/index.html)

this will backup each 24 hours your dinamodb vault data to a consul backend instance. 


`schedule` is optional for more documentation please check [robfig/cron](https://godoc.org/github.com/robfig/cron)