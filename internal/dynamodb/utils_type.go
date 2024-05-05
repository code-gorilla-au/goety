package dynamodb

import (
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
		Test("should remove Value property", func(t *testing.T) {
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
		Run()
	odize.AssertNoError(t, err)
}
