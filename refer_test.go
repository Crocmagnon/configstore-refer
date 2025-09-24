package refer_test

import (
	"testing"

	refer "codeberg.org/Crocmagnon/configstore-refer"
	"github.com/ovh/configstore"
	"gotest.tools/v3/assert"
)

func TestSimple(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{
	"url": "https://api.example.com"
}`, 1),
		configstore.NewItem("api-token", `some-token`, 1),
		configstore.NewItem("user", `api-user`, 1),
	)

	type Config struct {
		URL   string `json:"url"`
		User  string `json:"-" configstore:"user"`
		Token string `json:"-" configstore:"api-token"`
	}

	config, err := refer.GetFromStore[Config](store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.URL, "https://api.example.com")
	assert.Equal(t, config.User, "api-user")
	assert.Equal(t, config.Token, "some-token")
}

func TestNotStruct(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `some value`, 1),
	)

	config, err := refer.GetFromStore[string](store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config, "some value")
}

func TestNested(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{
	"container": {
		"api": {
			"url": "https://api.example.com"
		}
	}
}`, 1),
		configstore.NewItem("api-token", `some-token`, 1),
	)

	type API struct {
		URL   string `json:"url"`
		Token string `json:"-" configstore:"api-token"`
	}

	type Container struct {
		API API `json:"api"`
	}

	type Config struct {
		Container Container `json:"container"`
	}

	config, err := refer.GetFromStore[Config](store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.Container.API.URL, "https://api.example.com")
	assert.Equal(t, config.Container.API.Token, "some-token")
}
