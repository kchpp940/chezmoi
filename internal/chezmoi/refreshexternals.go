package chezmoi

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

type RefreshExternals int

const (
	RefreshExternalsAuto RefreshExternals = iota
	RefreshExternalsAlways
	RefreshExternalsNever
)

var (
	refreshExternalsWellKnownStrings = map[string]RefreshExternals{
		"always": RefreshExternalsAlways,
		"auto":   RefreshExternalsAuto,
		"never":  RefreshExternalsNever,
	}

	RefreshExternalsFlagCompletionFunc = FlagCompletionFunc(slices.Sorted(maps.Keys(refreshExternalsWellKnownStrings)))
)

func (re *RefreshExternals) Set(s string) error {
	if value, ok := refreshExternalsWellKnownStrings[strings.ToLower(s)]; ok {
		*re = value
		return nil
	}
	switch value, err := ParseBool(s); {
	case err != nil:
		return err
	case value:
		*re = RefreshExternalsAlways
		return nil
	default:
		*re = RefreshExternalsNever
		return nil
	}
}

func (re RefreshExternals) String() string {
	switch re {
	case RefreshExternalsAlways:
		return "always"
	case RefreshExternalsAuto:
		return "auto"
	case RefreshExternalsNever:
		return "never"
	default:
		panic(fmt.Sprintf("%d: invalid RefreshExternals value", re))
	}
}

func (re RefreshExternals) Type() string {
	return "always|auto|never"
}

func StringToRefreshExternalsHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t != reflect.TypeOf(RefreshExternals(0)) {
			return data, nil
		}
		var result RefreshExternals
		if err := result.Set(data.(string)); err != nil {
			return nil, err
		}
		return result, nil
	}
}
