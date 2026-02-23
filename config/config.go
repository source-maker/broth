// Package config provides environment variable binding via struct tags.
// Reflection is used here as permitted by ADR-003.
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"
)

// MustBind binds environment variables to the struct fields of dst.
// Supported struct tags:
//   - `env:"ENV_VAR_NAME"` — environment variable name
//   - `default:"value"`    — default value if env var is not set
//   - `required:"true"`    — panics if env var is not set and no default
//
// Supported field types: string, int, bool, time.Duration.
func MustBind(dst any) {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		panic("config: MustBind requires a pointer to a struct")
	}
	v = v.Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		fv := v.Field(i)

		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}

		raw, ok := os.LookupEnv(envKey)
		if !ok {
			if def := field.Tag.Get("default"); def != "" {
				raw = def
				ok = true
			}
		}

		if !ok {
			if field.Tag.Get("required") == "true" {
				panic(fmt.Sprintf("config: required environment variable %s is not set", envKey))
			}
			continue
		}

		if err := setField(fv, raw); err != nil {
			panic(fmt.Sprintf("config: setting %s: %v", envKey, err))
		}
	}
}

func setField(fv reflect.Value, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Int, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(d))
		} else {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(n)
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	default:
		return fmt.Errorf("unsupported type %s", fv.Type())
	}
	return nil
}
