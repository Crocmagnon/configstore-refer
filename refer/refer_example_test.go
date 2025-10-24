package refer_test

import (
	"context"
	"fmt"
	"log"

	"github.com/ovh/configstore"
	"go.augendre.info/configstore-refer/refer"
)

func ExampleLoadFromStore() {
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
	// Output:
	// https://api.example.com
	// some-token
	// 1
}
