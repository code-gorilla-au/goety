package dynamodb

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// JSONStringify - returns a json string representation of the given value
func JSONStringify(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}

	return string(data)
}

// FlattenAttrList - flattens the given attribute value list.
func FlattenAttrList(data []map[string]types.AttributeValue) ([]map[string]any, error) {
	transformed := []map[string]any{}

	for _, item := range data {
		transformedItem, err := FlattenAttrValue(item)
		if err != nil {
			return nil, err
		}

		transformed = append(transformed, transformedItem)
	}

	return transformed, nil
}

// FlattenAttrValue - flattens the given attribute value map.
// Removes the "Value" attribute from the AttributeValueMember struct and returns the value as a map[string]any.
func FlattenAttrValue(data map[string]types.AttributeValue) (map[string]any, error) {
	transformed := map[string]any{}

	for key, value := range data {
		transformedValue, err := extractAttrValue(value)
		if err != nil {
			return nil, err
		}

		transformed[key] = transformedValue
	}

	return transformed, nil
}

// collectAttrMap walks a member map, converting every value with convert
// and returning a new map holding the converted values, keyed the same way.
func collectAttrMap(
	members map[string]types.AttributeValue,
	convert func(types.AttributeValue) (any, error),
) (map[string]any, error) {
	out := make(map[string]any, len(members))
	for key, value := range members {
		converted, err := convert(value)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

// collectAttrList walks a member list, converting every value with convert
// and returning a new slice holding the converted values, in order.
func collectAttrList(
	items []types.AttributeValue,
	convert func(types.AttributeValue) (any, error),
) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, value := range items {
		converted, err := convert(value)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func extractAttrValue(value types.AttributeValue) (any, error) {
	var returnVal any
	switch v := value.(type) {
	case *types.AttributeValueMemberS:
		returnVal = v.Value
	case *types.AttributeValueMemberN:
		returnVal = json.Number(v.Value)
	case *types.AttributeValueMemberB:
		returnVal = v.Value
	case *types.AttributeValueMemberBOOL:
		returnVal = v.Value
	case *types.AttributeValueMemberNULL:
		returnVal = nil
	case *types.AttributeValueMemberM:
		result, err := collectAttrMap(v.Value, extractAttrValue)
		if err != nil {
			return nil, err
		}
		returnVal = result
	case *types.AttributeValueMemberL:
		result, err := collectAttrList(v.Value, extractAttrValue)
		if err != nil {
			return nil, err
		}
		returnVal = result
	case *types.AttributeValueMemberSS:
		returnVal = v.Value
	case *types.AttributeValueMemberNS:
		returnVal = v.Value
	case *types.AttributeValueMemberBS:
		returnVal = v.Value
	default:
		return nil, fmt.Errorf("unknown AttributeValue type: %T", value)
	}

	return returnVal, nil
}

func ConvertAVValues(data []map[string]types.AttributeValue) ([]map[string]AVer, error) {
	transformed := []map[string]AVer{}

	for _, item := range data {
		transformedItem, err := ConvertAVValue(item)
		if err != nil {
			return nil, err
		}

		transformed = append(transformed, transformedItem)
	}

	return transformed, nil
}

func ConvertAVValue(data map[string]types.AttributeValue) (map[string]AVer, error) {
	transformed := map[string]AVer{}

	for key, value := range data {
		transformedValue, err := convertAVValue(value)
		if err != nil {
			return nil, err
		}

		transformed[key] = transformedValue
	}

	return transformed, nil
}

func convertAVValue(value types.AttributeValue) (AVer, error) {
	var returnVal AVer
	switch v := value.(type) {
	case *types.AttributeValueMemberS:
		returnVal = AVString{
			S: v.Value,
		}
	case *types.AttributeValueMemberN:
		returnVal = AVNumber{
			N: v.Value,
		}
	case *types.AttributeValueMemberB:
		returnVal = AVByte{
			B: v.Value,
		}
	case *types.AttributeValueMemberBOOL:
		returnVal = AVBool{
			BOOL: v.Value,
		}
	case *types.AttributeValueMemberNULL:
		returnVal = AVNull{
			NULL: v.Value,
		}
	case *types.AttributeValueMemberM:
		result, err := collectAttrMap(v.Value, func(av types.AttributeValue) (any, error) {
			return convertAVValue(av)
		})
		if err != nil {
			return nil, err
		}
		returnVal = AVMap{M: result}
	case *types.AttributeValueMemberL:
		result, err := collectAttrList(v.Value, func(av types.AttributeValue) (any, error) {
			return convertAVValue(av)
		})
		if err != nil {
			return nil, err
		}
		returnVal = AVList{L: result}
	case *types.AttributeValueMemberSS:
		returnVal = AVStringSet{
			SS: v.Value,
		}
	case *types.AttributeValueMemberNS:
		returnVal = AVNumberSet{
			NS: v.Value,
		}
	case *types.AttributeValueMemberBS:
		returnVal = AVByteSet{
			BS: v.Value,
		}
	default:
		return nil, fmt.Errorf("unknown AttributeValue type: %T", value)
	}

	return returnVal, nil
}
