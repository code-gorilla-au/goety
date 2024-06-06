package dynamodb

import (
	"encoding/json"
	"strconv"

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

func extractAttrValue(value types.AttributeValue) (any, error) {
	var returnVal any
	switch v := value.(type) {
	case *types.AttributeValueMemberS:
		returnVal = v.Value
	case *types.AttributeValueMemberN:
		parsed, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			return nil, err
		}
		returnVal = parsed
	case *types.AttributeValueMemberB:
		returnVal = v.Value
	case *types.AttributeValueMemberBOOL:
		returnVal = v.Value
	case *types.AttributeValueMemberNULL:
		returnVal = nil
	case *types.AttributeValueMemberM:
		var err error
		result := map[string]any{}
		for key, value := range v.Value {
			result[key], err = extractAttrValue(value)
			if err != nil {
				return nil, err
			}
		}
		returnVal = result
	case *types.AttributeValueMemberL:
		result := []any{}
		for _, item := range v.Value {
			transformedItem, err := extractAttrValue(item)
			if err != nil {
				return nil, err
			}

			result = append(result, transformedItem)
		}
		returnVal = result
	case *types.AttributeValueMemberSS:
		returnVal = v.Value
	case *types.AttributeValueMemberNS:
		returnVal = v.Value
	case *types.AttributeValueMemberBS:
		returnVal = v.Value
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
		var err error
		result := map[string]any{}
		for key, value := range v.Value {
			result[key], err = convertAVValue(value)
			if err != nil {
				return nil, err
			}
		}
		returnVal = AVMap{
			M: result,
		}
	case *types.AttributeValueMemberL:
		result := []any{}
		for _, item := range v.Value {
			transformedItem, err := convertAVValue(item)
			if err != nil {
				return nil, err
			}

			result = append(result, transformedItem)
		}
		returnVal = AVList{
			L: result,
		}
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
	}

	return returnVal, nil
}
