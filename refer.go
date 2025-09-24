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
	for typeOfParent.Kind() == reflect.Pointer {
		typeOfParent = typeOfParent.Elem()
	}
	if typeOfParent.Kind() != reflect.Struct {
		return nil
	}

	valueOfParent := reflect.ValueOf(parent)
	for valueOfParent.Kind() == reflect.Pointer {
		valueOfParent = valueOfParent.Elem()
	}

	for i := range typeOfParent.NumField() {
		typeField := typeOfParent.Field(i)

		if !typeField.IsExported() {
			continue
		}

		typeOfField := typeField.Type
		for typeOfField.Kind() == reflect.Pointer {
			typeOfField = typeOfField.Elem()
		}

		tag := typeField.Tag.Get("configstore")

		if typeOfField.Kind() == reflect.Struct && tag == "" {
			if err := lookup(store, valueOfParent.Field(i).Addr().Interface()); err != nil {
				return err
			}
			continue
		}

		if tag == "" {
			continue
		}

		f := valueOfParent.Field(i)
		if !f.CanSet() {
			continue
		}

		item, err := configstore.Filter().
			Store(store).
			Slice(tag).
			Unmarshal(func() interface{} { return reflect.New(typeOfField).Interface() }).
			Squash().
			MustGetFirstItem().
			Unmarshaled()
		if err != nil {
			return fmt.Errorf("getting referenced item from configstore: %w", err)
		}

		val := reflect.ValueOf(item).Elem().Interface()

		f.Set(reflect.ValueOf(val))
	}
	return nil
}
