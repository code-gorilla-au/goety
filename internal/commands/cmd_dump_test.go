package commands

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/code-gorilla-au/odize"
)

func TestParseAttributeNames(t *testing.T) {
	group := odize.NewGroup(t, nil)

	cases := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "should return no names for empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "should parse multiple pairs",
			input: "#pk=pk,#sk=sk",
			want:  map[string]string{"#pk": "pk", "#sk": "sk"},
		},
		{
			name:  "should trim whitespace around keys and values",
			input: " #pk = pk , #sk = sk ",
			want:  map[string]string{"#pk": "pk", "#sk": "sk"},
		},
		{
			name:  "should keep equals signs inside a value",
			input: "#pk=a=b=c",
			want:  map[string]string{"#pk": "a=b=c"},
		},
		{
			name:    "should error when an entry has no separator",
			input:   "#pk",
			wantErr: true,
		},
		{
			name:    "should error when a key is empty",
			input:   "=pk",
			wantErr: true,
		},
		{
			name:  "should keep the last value for duplicate keys",
			input: "#pk=first,#pk=last",
			want:  map[string]string{"#pk": "last"},
		},
	}

	for _, tc := range cases {
		group.Test(tc.name, func(t *testing.T) {
			names, err := parseAttributeNames(tc.input)
			if tc.wantErr {
				odize.AssertError(t, err)
				return
			}

			odize.AssertNoError(t, err)
			odize.AssertEqual(t, tc.want, names)
		})
	}

	odize.AssertNoError(t, group.Run())
}

func TestParseAttributeValues(t *testing.T) {
	group := odize.NewGroup(t, nil)

	cases := []struct {
		name    string
		input   string
		want    map[string]types.AttributeValue
		wantErr bool
	}{
		{
			name:  "should return no values for empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "should parse multiple pairs",
			input: ":pk=1,:sk=2",
			want: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "1"},
				":sk": &types.AttributeValueMemberS{Value: "2"},
			},
		},
		{
			name:  "should trim whitespace around keys and values",
			input: " :pk = 1 , :sk = 2 ",
			want: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "1"},
				":sk": &types.AttributeValueMemberS{Value: "2"},
			},
		},
		{
			name:  "should keep equals signs inside a value",
			input: ":pk=YWJj==",
			want: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "YWJj=="},
			},
		},
		{
			name:    "should error when an entry has no separator",
			input:   ":pk",
			wantErr: true,
		},
		{
			name:    "should error when a key is empty",
			input:   "=1",
			wantErr: true,
		},
		{
			name:  "should keep the last value for duplicate keys",
			input: ":pk=first,:pk=last",
			want: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "last"},
			},
		},
	}

	for _, tc := range cases {
		group.Test(tc.name, func(t *testing.T) {
			values, err := parseAttributeValues(tc.input)
			if tc.wantErr {
				odize.AssertError(t, err)
				return
			}

			odize.AssertNoError(t, err)
			odize.AssertEqual(t, tc.want, values)
		})
	}

	odize.AssertNoError(t, group.Run())
}
