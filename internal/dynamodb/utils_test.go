package dynamodb

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/odize"
)

func Test_transformJSON(t *testing.T) {
	group := odize.NewGroup(t, nil)

	err := group.
		Test("should remove Value property from a string attr", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "pk"},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, "pk", d["pk"])
		}).
		Test("should remove Value property from map", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"sk": &types.AttributeValueMemberS{Value: "sk"},
				}},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			p := d["pk"].(map[string]any)
			odize.AssertEqual(t, "sk", p["sk"])
		}).
		Test("should return number", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"number": &types.AttributeValueMemberN{Value: "100"},
				}},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			p := d["pk"].(map[string]any)
			odize.AssertEqual(t, json.Number("100"), p["number"])
		}).
		Test("should preserve large integers", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"big": &types.AttributeValueMemberN{Value: "12345678901234567890"},
				"dec": &types.AttributeValueMemberN{Value: "1.5"},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			data, err := json.Marshal(d)
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, `{"big":12345678901234567890,"dec":1.5}`, string(data))
		}).
		Test("should return null", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"null": &types.AttributeValueMemberNULL{Value: true},
				}},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			p := d["pk"].(map[string]any)
			odize.AssertEqual(t, nil, p["null"])
		}).
		Test("should return list of numbers", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberL{Value: []types.AttributeValue{
					&types.AttributeValueMemberN{Value: "100"},
					&types.AttributeValueMemberN{Value: "200"},
				}},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			p := d["pk"].([]any)
			odize.AssertEqual(t, json.Number("100"), p[0])
			odize.AssertEqual(t, json.Number("200"), p[1])
		}).
		Run()
	odize.AssertNoError(t, err)
}
