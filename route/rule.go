package route

import (
	"errors"
	"fmt"
	"strings"

	"github.com/layou233/zbproxy/v3/adapter"
	"github.com/layou233/zbproxy/v3/common"
	"github.com/layou233/zbproxy/v3/common/set"
	"github.com/layou233/zbproxy/v3/config"
)

const (
	parameterListPrefix = "list:"
	typeCustomPrefix    = "custom:"
)

var ErrRuleTypeNotFound = errors.New("rule type is not found")

type Rule interface {
	Config() *config.Rule
	Match(metadata *adapter.Metadata) bool
}

func NewRule(config *config.Rule, listMap map[string]set.StringSet, ruleRegistry map[string]CustomRuleInitializer) (Rule, error) {
	switch config.Type {
	case "always":
		return &RuleAlways{config}, nil
	case "and":
		return NewLogicalAndRule(config, listMap, ruleRegistry)
	case "or":
		return NewLogicalOrRule(config, listMap, ruleRegistry)
	case "SourceIPVersion":
		return NewSourceIPVersionRule(config)
	case "SourceIP":
		return NewSourceIPRule(config, listMap)
	case "MinecraftHostname":
		return NewMinecraftHostnameRule(config, listMap)
	case "MinecraftPlayerName":
		return NewMinecraftPlayerNameRule(config, listMap)
	}
	if len(ruleRegistry) > 0 && strings.HasPrefix(config.Type, typeCustomPrefix) {
		typeName := strings.TrimPrefix(config.Type, typeCustomPrefix)
		ruleInitializer, found := ruleRegistry[typeName]
		if !found {
			return nil, fmt.Errorf("unknown custom rule type: %s", typeName)
		}
		return ruleInitializer(config, listMap)
	}
	return nil, common.Cause("type ["+config.Type+"]: ", ErrRuleTypeNotFound)
}