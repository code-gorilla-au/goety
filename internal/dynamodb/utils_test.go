package dynamodb

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/odize"
)

// unknownAV satisfies types.AttributeValue — whose sole method is the
// unexported isAttributeValue() — by embedding a pointer to an SDK member
// type and promoting that witness method (a same-named method declared in
// another package alone is not enough). Being its own struct type — none of
// the ten *types.AttributeValueMember* pointer types — a type switch over
// the members falls through to the default case.
type unknownAV struct {
	*types.AttributeValueMemberS
}

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
		Test("should traverse a deeply nested map list map structure", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"root": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"children": &types.AttributeValueMemberL{Value: []types.AttributeValue{
						&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
							"name": &types.AttributeValueMemberS{Value: "one"},
						}},
						&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
							"name": &types.AttributeValueMemberS{Value: "two"},
						}},
					}},
				}},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			want := map[string]any{
				"root": map[string]any{
					"children": []any{
						map[string]any{"name": "one"},
						map[string]any{"name": "two"},
					},
				},
			}
			odize.AssertEqual(t, want, d)
		}).
		Test("should pass through set members unchanged", func(t *testing.T) {
			ss := []string{"a", "b"}
			ns := []string{"1", "2"}
			bs := [][]byte{{0x01}, {0x02, 0x03}}

			example := map[string]types.AttributeValue{
				"ss": &types.AttributeValueMemberSS{Value: ss},
				"ns": &types.AttributeValueMemberNS{Value: ns},
				"bs": &types.AttributeValueMemberBS{Value: bs},
			}

			d, err := FlattenAttrValue(example)
			odize.AssertNoError(t, err)

			odize.AssertEqual(t, ss, d["ss"])
			odize.AssertEqual(t, ns, d["ns"])
			odize.AssertEqual(t, bs, d["bs"])
		}).
		Test("should error on an unknown attribute value type", func(t *testing.T) {
			in := map[string]types.AttributeValue{"k": unknownAV{}}

			_, err := FlattenAttrValue(in)
			odize.AssertError(t, err)
			odize.AssertTrue(t, strings.Contains(err.Error(), "unknown AttributeValue type"))
		}).
		Run()
	odize.AssertNoError(t, err)
}

func Test_convertAVValue(t *testing.T) {
	group := odize.NewGroup(t, nil)

	err := group.
		Test("should wrap scalar members in AVer structs", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"s":    &types.AttributeValueMemberS{Value: "pk"},
				"n":    &types.AttributeValueMemberN{Value: "100"},
				"b":    &types.AttributeValueMemberB{Value: []byte{0x01}},
				"bool": &types.AttributeValueMemberBOOL{Value: true},
				"null": &types.AttributeValueMemberNULL{Value: true},
			}

			d, err := ConvertAVValue(example)
			odize.AssertNoError(t, err)

			str, ok := d["s"].(AVString)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, "pk", str.S)

			num, ok := d["n"].(AVNumber)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, "100", num.N)

			bin, ok := d["b"].(AVByte)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, []byte{0x01}, bin.B)

			boolean, ok := d["bool"].(AVBool)
			odize.AssertTrue(t, ok)
			odize.AssertTrue(t, boolean.BOOL)

			null, ok := d["null"].(AVNull)
			odize.AssertTrue(t, ok)
			odize.AssertTrue(t, null.NULL)
		}).
		Test("should wrap set members in AVer structs", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"ss": &types.AttributeValueMemberSS{Value: []string{"a", "b"}},
				"ns": &types.AttributeValueMemberNS{Value: []string{"1", "2"}},
				"bs": &types.AttributeValueMemberBS{Value: [][]byte{{0x01}, {0x02, 0x03}}},
			}

			d, err := ConvertAVValue(example)
			odize.AssertNoError(t, err)

			strSet, ok := d["ss"].(AVStringSet)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, []string{"a", "b"}, strSet.SS)

			numSet, ok := d["ns"].(AVNumberSet)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, []string{"1", "2"}, numSet.NS)

			byteSet, ok := d["bs"].(AVByteSet)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, [][]byte{{0x01}, {0x02, 0x03}}, byteSet.BS)
		}).
		Test("should traverse a deeply nested map list map structure", func(t *testing.T) {
			example := map[string]types.AttributeValue{
				"root": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"children": &types.AttributeValueMemberL{Value: []types.AttributeValue{
						&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
							"name": &types.AttributeValueMemberS{Value: "one"},
						}},
						&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
							"name": &types.AttributeValueMemberS{Value: "two"},
						}},
					}},
				}},
			}

			d, err := ConvertAVValue(example)
			odize.AssertNoError(t, err)

			root, ok := d["root"].(AVMap)
			odize.AssertTrue(t, ok)

			children, ok := root.M["children"].(AVList)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, 2, len(children.L))

			first, ok := children.L[0].(AVMap)
			odize.AssertTrue(t, ok)
			firstName, ok := first.M["name"].(AVString)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, "one", firstName.S)

			second, ok := children.L[1].(AVMap)
			odize.AssertTrue(t, ok)
			secondName, ok := second.M["name"].(AVString)
			odize.AssertTrue(t, ok)
			odize.AssertEqual(t, "two", secondName.S)
		}).
		Test("should error on an unknown attribute value type", func(t *testing.T) {
			in := map[string]types.AttributeValue{"k": unknownAV{}}

			_, err := ConvertAVValue(in)
			odize.AssertError(t, err)
			odize.AssertTrue(t, strings.Contains(err.Error(), "unknown AttributeValue type"))
		}).
		Run()
	odize.AssertNoError(t, err)
}
