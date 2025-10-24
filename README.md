# configstore-refer

`configstore-refer` helps you build a single configuration struct while keeping your `configstore` entries separate.
It uses struct tags to refer to other configstore items, with support typed values and nested json (with the help of https://github.com/itchyny/gojq).

```shell
go get go.augendre.info/configstore-refer@latest
```

See package [`refer`](/refer) for more information.

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/ovh/configstore"
    "go.augendre.info/configstore-refer/refer"
)

func main() {
	store := configstore.NewStore()
	store.InMemory("mem").Add(
		configstore.NewItem("config", `
{
	"url": "https://api.example.com"
}
`, 1),
		configstore.NewItem("api-token", `some-token`, 1),
		configstore.NewItem("api-policy", `
{
    "policy": {
        "retryNumber": 1
    }
}
`, 1),
	)

	type Config struct {
		URL         string `json:"url"`
		Token       string `json:"-" configstore:"api-token"`
		RetryNumber int    `json:"-" configstore:"api-policy,.policy.retryNumber | tostring | tonumber"`
	}

	conf, err := refer.LoadFromStore[Config](context.TODO(), store, "config")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(conf.URL)
	fmt.Println(conf.Token)
	fmt.Println(conf.RetryNumber)
}
```