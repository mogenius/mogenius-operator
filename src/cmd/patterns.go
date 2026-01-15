package cmd

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/core"
	"mogenius-operator/src/logging"
	schemaPkg "mogenius-operator/src/schema"
	"mogenius-operator/src/shutdown"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type patternsArgs struct {
	Output string `help:"" default:"json"`
}

func RunPatterns(args *patternsArgs, logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	configModule.Validate()

	systems := InitializeSystems(logManagerModule, configModule, cmdLogger, valkeyLogChannel)
	defer shutdown.ExecuteShutdownHandlers()

	socketApi := systems.socketApi
	socketApi.AssertPatternsUnique()
	patternConfig := systems.socketApi.PatternConfigs()

	var outputData string
	output := strings.TrimSpace(args.Output)
	switch output {
	case "json":
		outputData = patternsToJson(patternConfig)
	case "yaml":
		outputData = patternsToYaml(patternConfig)
	case "typescript":
		outputData = patternsToTypescript(patternConfig, socketApi)
	default:
		return fmt.Errorf("invalid output format")
	}

	fmt.Print(outputData)

	return nil
}

func patternsToJson(patternConfig map[string]core.PatternConfig) string {
	data, err := json.Marshal(patternConfig)
	assert.Assert(err == nil, err)

	return string(data)
}

func patternsToYaml(patternConfig map[string]core.PatternConfig) string {
	// this is going through json to inherit the json tag config
	jsonData := patternsToJson(patternConfig)

	var data any
	err := json.Unmarshal([]byte(jsonData), &data)
	assert.Assert(err == nil, err)

	bytes, err := yaml.Marshal(data)
	assert.Assert(err == nil, err)

	return string(bytes)
}

func patternsToTypescript(patternConfig map[string]core.PatternConfig, socketApi core.SocketApi) string {
	// prepare data
	normalizedPatternToRawPatternMap := map[string]string{}
	normalizedPatternList := []string{}
	normalizedPatternConfigMap := map[string]core.PatternConfig{}

	// normalize data
	for pattern, config := range patternConfig {
		normalizedPattern := socketApi.NormalizePatternName(pattern)
		normalizedPatternList = append(normalizedPatternList, normalizedPattern)
		normalizedPatternToRawPatternMap[normalizedPattern] = pattern
		normalizedPatternConfigMap[normalizedPattern] = config
	}
	slices.Sort(normalizedPatternList)

	// render
	tsbuffer := ""
	tsbuffer = appendPatternEnum(tsbuffer, normalizedPatternList, normalizedPatternConfigMap, normalizedPatternToRawPatternMap)
	tsbuffer = appendPatternMappings(tsbuffer, normalizedPatternList, normalizedPatternConfigMap, normalizedPatternToRawPatternMap)
	tsbuffer = appendRequestResponseTypeDefinitions(tsbuffer, normalizedPatternList, normalizedPatternConfigMap)
	tsbuffer = appendStructLayoutDefinitions(tsbuffer, normalizedPatternList, normalizedPatternConfigMap)
	tsbuffer = appendTypeMappingDefinition(tsbuffer, normalizedPatternList, normalizedPatternConfigMap)

	return tsbuffer
}

func appendPatternEnum(
	buff string,
	normalizedPatternList []string,
	normalizedPatternConfigMap map[string]core.PatternConfig,
	normalizedPatternToRawPatternMap map[string]string,
) string {
	buff = buff + "//===============================================================\n"
	buff = buff + "//===================== Pattern Enumeration =====================\n"
	buff = buff + "//===============================================================\n"
	buff = buff + "\n"

	buff = buff + "export enum Pattern {\n"
	for _, normalizedPattern := range normalizedPatternList {
		actualPattern, ok := normalizedPatternToRawPatternMap[normalizedPattern]
		assert.Assert(ok)
		config, ok := normalizedPatternConfigMap[normalizedPattern]
		assert.Assert(ok)
		if config.Deprecated {
			msg := ""
			if config.DeprecatedMessage != "" {
				msg = " " + config.DeprecatedMessage
			}
			buff = buff + "  /** @deprecated" + msg + " */\n"
		}
		buff = buff + fmt.Sprintf(`  %s = "%s",`+"\n", normalizedPattern, actualPattern)
	}
	buff = buff + "}\n"
	buff = buff + "\n"

	return buff
}

func appendPatternMappings(
	buff string,
	normalizedPatternList []string,
	normalizedPatternConfigMap map[string]core.PatternConfig,
	normalizedPatternToRawPatternMap map[string]string,
) string {
	buff = buff + "//===============================================================\n"
	buff = buff + "//====================== Pattern Mappings =======================\n"
	buff = buff + "//===============================================================\n"
	buff = buff + "\n"

	buff = buff + "export const StringToPattern = {\n"
	for _, normalizedPattern := range normalizedPatternList {
		actualPattern, ok := normalizedPatternToRawPatternMap[normalizedPattern]
		assert.Assert(ok)
		config, ok := normalizedPatternConfigMap[normalizedPattern]
		assert.Assert(ok)
		if config.Deprecated {
			msg := ""
			if config.DeprecatedMessage != "" {
				msg = " " + config.DeprecatedMessage
			}
			buff = buff + "  /** @deprecated" + msg + " */\n"
		}
		buff = buff + `  "` + actualPattern + `": Pattern.` + normalizedPattern + `,` + "\n"
	}
	buff = buff + "};\n"
	buff = buff + "\n"

	buff = buff + "export const PatternToString = {\n"
	for _, normalizedPattern := range normalizedPatternList {
		actualPattern, ok := normalizedPatternToRawPatternMap[normalizedPattern]
		assert.Assert(ok)
		config, ok := normalizedPatternConfigMap[normalizedPattern]
		assert.Assert(ok)
		if config.Deprecated {
			msg := ""
			if config.DeprecatedMessage != "" {
				msg = " " + config.DeprecatedMessage
			}
			buff = buff + "  /** @deprecated" + msg + " */\n"
		}
		buff = buff + `  [Pattern.` + normalizedPattern + `]: "` + actualPattern + `",` + "\n"
	}
	buff = buff + "};\n"
	buff = buff + "\n"

	return buff
}

func appendRequestResponseTypeDefinitions(buff string, normalizedPatternList []string, patterns map[string]core.PatternConfig) string {
	buff = buff + "//===============================================================\n"
	buff = buff + "//================= Request and Response Types ==================\n"
	buff = buff + "//===============================================================\n"
	buff = buff + "\n"

	for _, pattern := range normalizedPatternList {
		config, ok := patterns[pattern]
		assert.Assert(ok)

		buff = buff + schemaAsTypescriptType(pattern+"_REQUEST", config, config.RequestSchema)
		buff = buff + schemaAsTypescriptType(pattern+"_RESPONSE", config, config.ResponseSchema)
	}

	buff = buff + "\n"

	return buff
}

func appendStructLayoutDefinitions(buff string, normalizedPatternList []string, patternConfigs map[string]core.PatternConfig) string {
	buff = buff + "//===============================================================\n"
	buff = buff + "//===================== Struct Definitions ======================\n"
	buff = buff + "//===============================================================\n"
	buff = buff + "\n"

	for _, normalizedPattern := range normalizedPatternList {
		config, ok := patternConfigs[normalizedPattern]
		assert.Assert(ok)

		if config.RequestSchema != nil {
			normalizedPatternWithScope := normalizedPattern + "_REQUEST"
			requestStructNames := mapKeysSorted(config.RequestSchema.StructLayouts)
			for _, structRef := range requestStructNames {
				layout := config.RequestSchema.StructLayouts[structRef]
				tsTypeName := structRefToTypescriptName(normalizedPatternWithScope, structRef)
				buff = buff + "export type " + tsTypeName + " = " + structLayoutToTypescriptType(normalizedPatternWithScope, config.RequestSchema, &layout) + ";\n"
			}
		}

		if config.ResponseSchema != nil {
			normalizedPatternWithScope := normalizedPattern + "_RESPONSE"
			responseStructNames := mapKeysSorted(config.ResponseSchema.StructLayouts)
			for _, structRef := range responseStructNames {
				layout := config.ResponseSchema.StructLayouts[structRef]
				tsTypeName := structRefToTypescriptName(normalizedPatternWithScope, structRef)
				buff = buff + "export type " + tsTypeName + " = " + structLayoutToTypescriptType(normalizedPatternWithScope, config.ResponseSchema, &layout) + ";\n"
			}
		}
	}

	buff = buff + "\n"

	return buff
}

func structLayoutToTypescriptType(normalizedPatternWithScope string, schema *schemaPkg.Schema, layout *schemaPkg.StructLayout) string {
	buff := ""

	buff = buff + "{"
	idx := 0
	propertyNames := mapKeysSorted(layout.Properties)
	for _, name := range propertyNames {
		property := layout.Properties[name]
		buff = buff + `"` + name + `": ` + toTypescriptType(normalizedPatternWithScope, schema, property, 0)
		idx++
		if idx < len(layout.Properties) {
			buff = buff + ","
		}
	}
	buff = buff + "}"

	return buff
}

func schemaAsTypescriptType(normalizedPatternWithScope string, patternConfig core.PatternConfig, schema *schemaPkg.Schema) string {
	buff := ""

	if schema == nil {
		buff = buff + "/**\n"
		buff = buff + " * api schema has not been defined by the operator\n"
		buff = buff + " */\n"
		buff = buff + "export type " + normalizedPatternWithScope + " = any;\n"
		buff = buff + "\n"
		return buff
	}

	if schema.TypeInfo.Type == schemaPkg.SchemaTypeFunction {
		return buff
	}

	// doc comment area
	buff = buff + "/**\n"
	buff = buff + " * #### Source\n"
	buff = buff + " *\n"
	buff = buff + " * ```yaml\n"
	buff = buff + prefixMultilineString(schema.Yaml(), " * ")
	buff = buff + " * ```\n"
	buff = buff + " *\n"
	if patternConfig.Deprecated {
		deprecatedMessage := ""
		if patternConfig.DeprecatedMessage != "" {
			deprecatedMessage = ` The pattern "` + normalizedPatternWithScope + `" is deprecated: ` + patternConfig.DeprecatedMessage
		}
		buff = buff + " * @deprecated" + deprecatedMessage + "\n"
	}
	buff = buff + " */\n"

	buff = buff + "export type " + normalizedPatternWithScope + " = " + toTypescriptType(normalizedPatternWithScope, schema, schema.TypeInfo, 0) + ";\n"

	buff = buff + "\n"

	return buff
}

// recursively translate a schemaPkg.TypeInfo to typescript
func toTypescriptType(normalizedPatternWithScope string, schema *schemaPkg.Schema, typeInfo *schemaPkg.TypeInfo, depth int) string {
	assert.Assert(depth <= 4096, "recursion limit reached")
	assert.Assert(schema != nil)
	assert.Assert(typeInfo != nil)

	nullableTypeAppendix := ""
	if typeInfo.Pointer {
		nullableTypeAppendix = "|undefined"
	}

	buff := ""

	switch typeInfo.Type {
	case schemaPkg.SchemaTypeBoolean:
		buff = buff + "boolean" + nullableTypeAppendix
	case schemaPkg.SchemaTypeInteger, schemaPkg.SchemaTypeUnsignedInteger, schemaPkg.SchemaTypeFloat:
		buff = buff + "number" + nullableTypeAppendix
	case schemaPkg.SchemaTypeString:
		buff = buff + "string" + nullableTypeAppendix
	case schemaPkg.SchemaTypeArray:
		buff = buff + toTypescriptType(normalizedPatternWithScope, schema, typeInfo.ElementType, depth+1) + "[]" + nullableTypeAppendix
	case schemaPkg.SchemaTypeMap:
		buff = buff + "Record<" + toTypescriptType(normalizedPatternWithScope, schema, typeInfo.KeyType, depth+1) + ", " + toTypescriptType(normalizedPatternWithScope, schema, typeInfo.ValueType, depth+1) + ">"
	case schemaPkg.SchemaTypeAny:
		buff = buff + "any"
	case schemaPkg.SchemaTypeStruct:
		buff = buff + structRefToTypescriptName(normalizedPatternWithScope, typeInfo.StructRef) + nullableTypeAppendix
	default:
		assert.Assert(false, "UNREACHABLE", typeInfo.Type)
	}

	return buff
}

func appendTypeMappingDefinition(buff string, normalizedPatternList []string, normalizedPatternConfigMap map[string]core.PatternConfig) string {
	buff = buff + "//===============================================================\n"
	buff = buff + "//==================== Pattern Type Mapping =====================\n"
	buff = buff + "//===============================================================\n"
	buff = buff + "\n"

	buff = buff + "export interface IPatternConfig {\n"
	for _, normalizedPattern := range normalizedPatternList {
		config, ok := normalizedPatternConfigMap[normalizedPattern]
		assert.Assert(ok)
		if config.Deprecated {
			msg := ""
			if config.DeprecatedMessage != "" {
				msg = " " + config.DeprecatedMessage
			}
			buff = buff + "  /** @deprecated" + msg + " */\n"
		}
		buff = buff + "  [Pattern." + normalizedPattern + "]: {\n"
		buff = buff + "    Request: " + normalizedPattern + "_REQUEST;" + "\n"
		buff = buff + "    Response: " + normalizedPattern + "_RESPONSE;" + "\n"
		buff = buff + "  };\n"
	}
	buff = buff + "};\n"
	buff = buff + "\n"

	buff = buff + "export type PatternKey = keyof IPatternConfig;\n"
	buff = buff + "\n"
	buff = buff + "export function PatternInfo<K extends PatternKey>(_: K): IPatternConfig[K] {\n"
	buff = buff + "  return {} as IPatternConfig[K];\n"
	buff = buff + "};\n"

	return buff
}

func prefixMultilineString(input string, prefix string) string {
	output := ""

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		output = output + prefix + scanner.Text() + "\n"
	}

	return output
}

func structRefToTypescriptName(normalizedPatternWithScope string, structref string) string {
	structref = strings.ToUpper(structref)
	structref = strings.ReplaceAll(structref, "/", "_")
	structref = strings.ReplaceAll(structref, ".", "_")
	structref = strings.ReplaceAll(structref, "-", "_")
	structref = strings.ReplaceAll(structref, ",", "_")
	structref = strings.ReplaceAll(structref, "[", "")
	structref = strings.ReplaceAll(structref, "]", "")
	structref = strings.ReplaceAll(structref, "*", "")
	structref = strings.ReplaceAll(structref, "·", "")
	return normalizedPatternWithScope + "__" + structref
}

func mapKeysSorted[K cmp.Ordered, V any](data map[K]V) []K {
	dataKeys := []K{}
	for key := range data {
		dataKeys = append(dataKeys, key)
	}
	slices.Sort(dataKeys)
	return dataKeys
}
