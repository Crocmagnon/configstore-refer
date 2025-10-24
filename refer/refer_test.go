package refer_test

import (
	"testing"

	"github.com/ovh/configstore"
	"go.augendre.info/configstore-refer/refer"
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

	config, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.URL, "https://api.example.com")
	assert.Equal(t, config.User, "api-user")
	assert.Equal(t, config.Token, "some-token")
}

func TestTypes(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{}`, 1),
		configstore.NewItem("intPos", `5`, 1),
		configstore.NewItem("intNeg", `-5`, 1),
		configstore.NewItem("boolTrue", `true`, 1),
		configstore.NewItem("stringSlice", `["foo", "bar"]`, 1),
	)

	type Config struct {
		IntPos      int      `json:"-" configstore:"intPos"`
		IntNeg      int      `json:"-" configstore:"intNeg"`
		Uint        uint     `json:"-" configstore:"intPos"`
		BoolTrue    bool     `json:"-" configstore:"boolTrue"`
		StringSlice []string `json:"-" configstore:"stringSlice"`
	}

	config, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.IntPos, 5)
	assert.Equal(t, config.IntNeg, -5)
	assert.Equal(t, config.Uint, uint(5))
	assert.Equal(t, config.BoolTrue, true)
	assert.DeepEqual(t, config.StringSlice, []string{"foo", "bar"})
}

func TestNotStruct(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `some value`, 1),
	)

	config, err := refer.LoadFromStore[string](t.Context(), store, "config")
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

	config, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.Container.API.URL, "https://api.example.com")
	assert.Equal(t, config.Container.API.Token, "some-token")
}

func TestStructRefer(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{
}`, 1),
		configstore.NewItem("api-config", `{
	"url": "https://api.example.com",
	"token": "some-token"
}`, 1),
	)

	type API struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}

	type Config struct {
		API API `json:"-" configstore:"api-config"`
	}

	config, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.API.URL, "https://api.example.com")
	assert.Equal(t, config.API.Token, "some-token")
}

func TestReferKeys(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{
}`, 1),
		configstore.NewItem("api-config", `{
	"api": {
		"url": "https://api.example.com",
		"concurrency": 2,
		"creds": {
			"token": "some-token",
			"user": "user"
		}
	}
}`, 1),
	)

	type Config struct {
		URL         string `json:"-" configstore:"api-config,.api.url"`
		Token       string `json:"-" configstore:"api-config,.api.creds.token"`
		User        string `json:"-" configstore:"api-config,.api.creds.user"`
		Concurrency int    `json:"-" configstore:"api-config,.api.concurrency | tostring | tonumber"`
	}

	config, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.NilError(t, err)

	assert.Equal(t, config.URL, "https://api.example.com")
	assert.Equal(t, config.Token, "some-token")
	assert.Equal(t, config.User, "user")
	assert.Equal(t, config.Concurrency, 2)
}

func TestMissingStoreItem(t *testing.T) {
	store := configstore.NewStore()
	store.InMemory(t.Name()).Add(
		configstore.NewItem("config", `{
	"url": "https://api.example.com"
}`, 1),
		configstore.NewItem("api-token", `some-token`, 1),
	)

	type Config struct {
		URL   string `json:"url"`
		User  string `json:"-" configstore:"user"` // no "user" item in the store
		Token string `json:"-" configstore:"api-token"`
	}

	_, err := refer.LoadFromStore[Config](t.Context(), store, "config")
	assert.ErrorContains(t, err, "no item found")
	assert.ErrorContains(t, err, "user")
}
