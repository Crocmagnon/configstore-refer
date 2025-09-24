package refer

import (
	"fmt"
	"reflect"

	"github.com/ovh/configstore"
)

func GetFromStore[T any](store *configstore.Store, key string) (T, error) {
	item, err := configstore.Filter().
		Store(store).
		Slice(key).
		Unmarshal(func() interface{} { return new(T) }).
		Squash().
		MustGetFirstItem().
		Unmarshaled()
	if err != nil {
		return *new(T), fmt.Errorf("getting item from configstore: %w", err)
	}

	typedItem := item.(*T)

	err = lookup(store, typedItem)
	if err != nil {
		return *typedItem, fmt.Errorf("looking up referenced items: %w", err)
	}

	return *typedItem, nil
}

func lookup(store *configstore.Store, parent any) error {
	typeOfParent := reflect.TypeOf(parent)
	if typeOfParent.Kind() == reflect.Pointer {
		typeOfParent = typeOfParent.Elem()
	}
	if typeOfParent.Kind() == reflect.Pointer {
		typeOfParent = typeOfParent.Elem()
	}
	if typeOfParent.Kind() != reflect.Struct {
		return nil
	}

	valueOfParent := reflect.ValueOf(parent)
	if valueOfParent.Kind() == reflect.Pointer {
		valueOfParent = valueOfParent.Elem()
	}
	if valueOfParent.Kind() == reflect.Pointer {
		valueOfParent = valueOfParent.Elem()
	}

	for i := range typeOfParent.NumField() {
		typeField := typeOfParent.Field(i)

		if !typeField.IsExported() {
			continue
		}

		typeOfField := typeField.Type
		if typeOfField.Kind() == reflect.Pointer {
			typeOfField = typeOfField.Elem()
		}

		if typeOfField.Kind() == reflect.Struct {
			// TODO handle error
			lookup(store, valueOfParent.Field(i).Addr().Interface())
			continue
		}

		tag, ok := typeField.Tag.Lookup("configstore")
		if !ok || tag == "" {
			continue
		}

		item, err := store.GetItemValue(tag)
		if err != nil {
			return fmt.Errorf("getting referenced item from configstore: %w", err)
		}

		f := valueOfParent.Field(i)
		if f.CanSet() {
			f.Set(reflect.ValueOf(item))
		}
	}
	return nil
}
