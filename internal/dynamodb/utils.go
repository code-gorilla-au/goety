package dynamodb

import (
	"encoding/json"

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
		returnVal = v.Value
	case *types.AttributeValueMemberB:
		returnVal = v.Value
	case *types.AttributeValueMemberBOOL:
		returnVal = v.Value
	case *types.AttributeValueMemberNULL:
		returnVal = v.Value
	case *types.AttributeValueMemberM:
		var err error
		returnVal, err = FlattenAttrValue(v.Value)
		if err != nil {
			return nil, err
		}
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
