// Package refer helps you build a single configuration struct
// while keeping your [github.com/ovh/configstore] entries separate.
// It uses struct tags to refer to other configstore items, with support for typed values and nested json
// with the help of [github.com/itchyny/gojq].
package refer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/ovh/configstore"
)

// LoadFromStore gets an item from the given configstore.Store
// and follows `configstore` tags on fields of the given T type.
//
// Tags must be in the following format: `configstore:"store-key[,expression]"`.
//
// `store-key` refers to another entry in the given store.
// LoadFromStore will attempt to load an item by that name and set it as the value of the struct field.
// If no item is found, an error is returned.
//
// `expression` is optional and assumes the entry referred to by store-key can be unmarshalled as map[string]any.
// It will be parsed by [github.com/itchyny/gojq.Parse] to retrieve a specific entry from the map
// and set it as the value of the struct field instead of the whole configstore item.
//
// Example:
//
//	type Config struct {
//		URL         string `json:"url"`
//		Token       string `json:"-"   configstore:"api-token"`
//		RetryNumber int    `json:"-"   configstore:"api-policy,.retryNumber | tostring | tonumber"`
//	}
func LoadFromStore[T any](ctx context.Context, store *configstore.Store, key string) (T, error) {
	item, err := GetFromStoreAs[T](store, key)
	if err != nil {
		return *new(T), fmt.Errorf("getting item from configstore: %w", err)
	}

	err = lookup(ctx, store, &item)
	if err != nil {
		return item, fmt.Errorf("looking up referenced items: %w", err)
	}

	return item, nil
}

// GetFromStoreAs returns a single typed item from the given store.
func GetFromStoreAs[T any](store *configstore.Store, key string) (T, error) {
	item, err := GetFromStore(store, key, new(T))
	if err != nil {
		return *new(T), err
	}

	return *item.(*T), nil //nolint:forcetypeassert // safe cast as per configstore package
}

// GetFromStore returns a single item from the given store, unmarshalled using the given type.
// unmarshalAs must be a pointer.
func GetFromStore(store *configstore.Store, key string, unmarshalAs any) (interface{}, error) {
	item, err := configstore.Filter().
		Store(store).
		Slice(key).
		Unmarshal(func() interface{} { return unmarshalAs }).
		Squash().
		MustGetFirstItem().
		Unmarshaled()
	if err != nil {
		return nil, fmt.Errorf("getting item from configstore: %w", err)
	}

	return item, nil
}

func lookup(ctx context.Context, store *configstore.Store, parent any) error {
	valueOfParent := derefValue(reflect.ValueOf(parent))
	if valueOfParent.Kind() != reflect.Struct {
		return nil
	}

	typeOfParent := valueOfParent.Type()

	for i := range typeOfParent.NumField() {
		err := handleField(ctx, store, typeOfParent, valueOfParent, i)
		if err != nil {
			return fmt.Errorf("handling field %d: %w", i, err)
		}
	}

	return nil
}

var (
	// ErrInvalidTag is returned when the configstore tag is malformed.
	ErrInvalidTag = errors.New("invalid tag")

	// ErrUnassignable is returned when the referred type is not assignable to the field type.
	// This can happen when the referred type is a json object and the field type is a struct.
	// It can only happen when using `expression` in the tag.
	ErrUnassignable = errors.New("unassignable type")
)

func handleField(
	ctx context.Context,
	store *configstore.Store,
	typeOfParent reflect.Type,
	valueOfParent reflect.Value,
	fieldIndex int,
) error {
	typeField := typeOfParent.Field(fieldIndex)
	if !typeField.IsExported() {
		return nil
	}

	valueField := valueOfParent.Field(fieldIndex)
	if !valueField.CanSet() {
		return nil
	}

	typeOfField := derefType(typeField.Type)

	tag := typeField.Tag.Get("configstore")
	if tag == "" {
		if typeOfField.Kind() != reflect.Struct {
			return nil
		}

		err := lookup(ctx, store, valueOfParent.Field(fieldIndex).Addr().Interface())
		if err != nil {
			return err
		}

		return nil
	}

	const maxTagParts = 2

	tagParts := strings.Split(tag, ",")
	switch len(tagParts) {
	case 1, maxTagParts:
		break // break out of the switch
	default:
		return fmt.Errorf(
			"%w: configstore tag doesn't contain exactly one or two parts: %s",
			ErrInvalidTag,
			tag,
		)
	}

	sliceKey := tagParts[0]

	query := ""
	if len(tagParts) == maxTagParts {
		query = tagParts[1]
	}

	if query == "" {
		// Can't use GetFromStoreAs[T] here because generics imply that the compiler can guess the type at compile time,
		// and we're using reflection which implies getting the type at runtime.
		item, err := GetFromStore(store, sliceKey, reflect.New(typeOfField).Interface())
		if err != nil {
			return err
		}

		valueField.Set(derefValue(reflect.ValueOf(item)))
	} else {
		err := handleWithQuery(ctx, store, sliceKey, query, valueField)
		if err != nil {
			return fmt.Errorf("handling with query: %w", err)
		}
	}

	return nil
}

func handleWithQuery(
	ctx context.Context,
	store *configstore.Store,
	sliceKey string,
	query string,
	valueField reflect.Value,
) error {
	item, err := GetFromStoreAs[map[string]any](store, sliceKey)
	if err != nil {
		return err
	}

	qry, err := gojq.Parse(query)
	if err != nil {
		return fmt.Errorf("parsing query %q: %w", query, err)
	}

	iter := qry.RunWithContext(ctx, item)

	value, hasNext := iter.Next()
	if !hasNext {
		return nil
	}

	if err, ok := value.(error); ok {
		var haltErr *gojq.HaltError
		if errors.As(err, &haltErr) && haltErr.Value() == nil {
			return nil
		}

		return fmt.Errorf("iterating over query results: %w", err)
	}

	valueOf := reflect.ValueOf(value)

	if valueOf.IsValid() {
		if !valueOf.Type().AssignableTo(valueField.Type()) {
			return fmt.Errorf(
				"%w: %s is not assignable to %s",
				ErrUnassignable,
				valueOf.Type(),
				valueField.Type(),
			)
		}

		valueField.Set(valueOf)
	}

	return nil
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

func derefValue(t reflect.Value) reflect.Value {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}
