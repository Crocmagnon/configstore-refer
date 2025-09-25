package refer

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/ovh/configstore"
)

func GetFromStore[T any](store *configstore.Store, key string) (T, error) {
	item, err := get[T](store, key)
	if err != nil {
		return *new(T), fmt.Errorf("getting item from configstore: %w", err)
	}

	err = lookup(store, &item)
	if err != nil {
		return item, fmt.Errorf("looking up referenced items: %w", err)
	}

	return item, nil
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

		tagParts := strings.Split(tag, ",")
		if len(tagParts) == 0 {
			continue
		}
		if len(tagParts) > 2 {
			return fmt.Errorf("configstore tag contains more than two parts: %s", tag)
		}

		sliceKey := tagParts[0]
		query := ""
		if len(tagParts) == 2 {
			query = tagParts[1]
		}

		f := valueOfParent.Field(i)
		if !f.CanSet() {
			continue
		}

		if query == "" {
			item, err := configstore.Filter().
				Store(store).
				Slice(sliceKey).
				Unmarshal(func() interface{} { return reflect.New(typeOfField).Interface() }).
				Squash().
				MustGetFirstItem().
				Unmarshaled()
			if err != nil {
				return fmt.Errorf("getting referenced item from configstore: %w", err)
			}

			val := reflect.ValueOf(item).Elem().Interface() // deref pointer

			f.Set(reflect.ValueOf(val))
		} else {
			item, err := get[map[string]any](store, sliceKey)
			if err != nil {
				return fmt.Errorf("getting referenced item from configstore: %w", err)
			}

			qry, err := gojq.Parse(query)
			if err != nil {
				return fmt.Errorf("parsing query %q: %w", query, err)
			}

			iter := qry.Run(item) // or query.RunWithContext
			for {
				v, hasNext := iter.Next()
				if !hasNext {
					break
				}

				if err, ok := v.(error); ok {
					var haltErr *gojq.HaltError
					if errors.As(err, &haltErr) && haltErr.Value() == nil {
						break
					}
					return fmt.Errorf("iterating over query results: %w", err)
				}

				f.Set(reflect.ValueOf(v))
			}
		}
	}
	return nil
}

func get[T any](store *configstore.Store, key string) (T, error) {
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

	return *item.(*T), nil
}
