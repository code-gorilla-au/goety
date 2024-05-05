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
		switch v := value.(type) {
		case *types.AttributeValueMemberS:
			transformed[key] = v.Value
		case *types.AttributeValueMemberN:
			transformed[key] = v.Value
		case *types.AttributeValueMemberB:
			transformed[key] = v.Value
		case *types.AttributeValueMemberBOOL:
			transformed[key] = v.Value
		case *types.AttributeValueMemberNULL:
			transformed[key] = v.Value
		case *types.AttributeValueMemberM:
			var err error
			transformed[key], err = FlattenAttrValue(v.Value)
			if err != nil {
				return nil, err
			}
		case *types.AttributeValueMemberL:
			result := []any{}
			for _, item := range v.Value {
				transformedItem, err := FlattenAttrValue(map[string]types.AttributeValue{"L": item})
				if err != nil {
					return nil, err
				}

				result = append(result, transformedItem)
			}
			transformed[key] = result
		case *types.AttributeValueMemberSS:
			transformed[key] = v.Value
		case *types.AttributeValueMemberNS:
			transformed[key] = v.Value
		case *types.AttributeValueMemberBS:
			transformed[key] = v.Value
		}

	}

	return transformed, nil
}
