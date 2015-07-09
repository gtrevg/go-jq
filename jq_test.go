package jq

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

var ee = errors.New("") // dummy error to flag expecting error

const testS = `{
    "foo": 1,
    "bar": 2,
    "test": "Hello, world!",
    "baz": 123.1,
    "array": [
        {"foo": 1},
        {"bar": 2},
        {"baz": 3}
    ],
    "subobj": {
        "foo": 1,
        "subarray": [1,2,3],
        "subsubobj": {
            "bar": 2,
            "baz": 3,
            "array": ["hello", "world"]
        }
    },
    "bool": true
}`

var testObj interface{}

var testStruct struct {
	Foo, Bar int
	Test     string
	Baz      float64
	Array    []struct {
		Foo, Bar, Baz int
	}
	Subobj struct {
		Foo       int
		Subarray  []int
		Subsubobj struct {
			Bar, Baz int
			Array    []string
		}
	}
}

func init() {
	if err := json.Unmarshal([]byte(testS), &testObj); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(testS), &testStruct); err != nil {
		panic(err)
	}
}

func TestQ(t *testing.T) {
	for _, tc := range []struct {
		root   interface{}
		path   []interface{}
		expect interface{}
	}{
		{"foo", nil, "foo"},
		{0, nil, 0},
		{0, []interface{}{0}, ee},
		{0, []interface{}{""}, ee},
		{[]int{1, 2, 3}, nil, []int{1, 2, 3}},
		{[]int{1, 2, 3}, []interface{}{"0"}, 1},
		{[]int{1, 2, 3}, []interface{}{0}, 1},
		{[]int{1, 2, 3}, []interface{}{-1}, nil},
		{[]int{1, 2, 3}, []interface{}{4}, nil},
		{[]int{1, 2, 3}, []interface{}{uint(0)}, 1},
		{[]int{1, 2, 3}, []interface{}{uint(4)}, nil},
		{[]int{1, 2, 3}, []interface{}{nil}, ee},
		{map[string]int{"0": 1, "1": 2}, nil, map[string]int{"0": 1, "1": 2}},
		{map[string]int{"0": 1, "1": 2}, []interface{}{"0"}, 1},
		{map[string]int{"0": 1, "1": 2}, []interface{}{"ffoo"}, nil},
		{map[string]int{"0": 1, "1": 2}, []interface{}{0}, ee},
		{map[int]string{0: "1", 1: "2"}, []interface{}{0}, "1"},
		{map[int]string{0: "1", 1: "2"}, []interface{}{4}, nil},
		{map[int]string{0: "1", 1: "2"}, []interface{}{"boo"}, ee},
		{map[int8]string{0: "1", 1: "2"}, []interface{}{0}, "1"},
		{map[int8]string{0: "1", 1: "2"}, []interface{}{4}, nil},
		{map[int8]string{0: "1", 1: "2"}, []interface{}{"boo"}, ee},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{0}, "1"},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{"0"}, "1"},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{4}, nil},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{-4}, nil},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{"-4"}, ee},
		{map[uint]string{0: "1", 1: "2"}, []interface{}{"boo"}, ee},
		{testObj, nil, testObj},
		{testObj, []interface{}{"foo"}, 1.}, // json.Unmarshal turns all numbers into floats
		{testObj, []interface{}{"baz"}, 123.1},
		{testObj, []interface{}{"array"}, []interface{}{map[string]interface{}{"foo": 1.}, map[string]interface{}{"bar": 2.}, map[string]interface{}{"baz": 3.}}},
		{testObj, []interface{}{"array", 0}, map[string]interface{}{"foo": 1.}},
		{testObj, []interface{}{"array", 0, "foo"}, 1.},
		{testObj, []interface{}{"array", 0, "bar"}, nil},
		{testObj, []interface{}{"subobj", "subarray"}, []interface{}{1., 2., 3.}},
		{testObj, []interface{}{"subobj", "subarray", 0}, 1.},
		{testStruct, nil, testStruct},
		{testStruct, []interface{}{"foo"}, 1},
		{testStruct, []interface{}{"test"}, "Hello, world!"},
		{testStruct, []interface{}{"test/bla"}, nil},
		{testStruct, []interface{}{"baz"}, 123.1},
		{testStruct, []interface{}{"array", 0, "foo"}, 1},
		{testStruct, []interface{}{"array", 0, "bar"}, 0}, // not set from json
		{testStruct, []interface{}{"array", 0, 0}, ee},    // wrong type of key
		{testStruct, []interface{}{"array", 0, "notexist"}, nil},
		{testStruct, []interface{}{"subobj", "subarray"}, []int{1, 2, 3}},
		{testStruct, []interface{}{"subobj", "subarray", 0}, 1},
		{testStruct, []interface{}{"subobj", "subarray", -1}, nil},
	} {
		if _, ok := tc.expect.(error); ok {
			v := Q(tc.root, tc.path...)
			if _, ok := v.(error); !ok {
				t.Errorf("%#v [%v]: expected error, got %v (%T) ", tc.root, tc.path, v, v)
			}
			continue
		}

		if v := Q(tc.root, tc.path...); !reflect.DeepEqual(v, tc.expect) {
			if e, ok := v.(error); ok {
				t.Errorf("%#v [%v]:  expected %v, got error: %q", tc.root, tc.path, tc.expect, e)
			} else {
				t.Errorf("%#v [%v]:  expected %v, got %v (%T)", tc.root, tc.path, tc.expect, v, v)
			}
		}
	}
}

func TestQQ(t *testing.T) {
	for _, tc := range []struct {
		root   interface{}
		path   string
		expect interface{}
	}{
		{"foo", "", "foo"},
		{0, "", 0},
		{0, " ", ee},
		{[]int{1, 2, 3}, "", []int{1, 2, 3}},
		{[]int{1, 2, 3}, "0", 1},
		{[]int{1, 2, 3}, "-1", nil},
		{[]int{1, 2, 3}, "4", nil},
		{map[string]int{"0": 1, "1": 2}, "", map[string]int{"0": 1, "1": 2}},
		{map[string]int{"0": 1, "1": 2}, "0", 1},
		{map[string]int{"0": 1, "1": 2}, "ffoo", nil},
		{map[int]string{0: "1", 1: "2"}, "0", "1"},
		{map[int]string{0: "1", 1: "2"}, "4", nil},
		{map[int]string{0: "1", 1: "2"}, "boo", ee},
		{map[int8]string{0: "1", 1: "2"}, "0", "1"},
		{map[int8]string{0: "1", 1: "2"}, "4", nil},
		{map[int8]string{0: "1", 1: "2"}, "boo", ee},
		{map[uint]string{0: "1", 1: "2"}, "0", "1"},
		{map[uint]string{0: "1", 1: "2"}, "4", nil},
		{map[uint]string{0: "1", 1: "2"}, "-4", ee},
		{map[uint]string{0: "1", 1: "2"}, "boo", ee},
		{testObj, "", testObj},
		{testObj, "foo", 1.}, // json.Unmarshal turns all numbers into floats
		{testObj, "baz", 123.1},
		{testObj, "array", []interface{}{map[string]interface{}{"foo": 1.}, map[string]interface{}{"bar": 2.}, map[string]interface{}{"baz": 3.}}},
		{testObj, "array/0", map[string]interface{}{"foo": 1.}},
		{testObj, "array/0/foo", 1.},
		{testObj, "array/0/bar", nil},
		{testObj, "subobj/subarray", []interface{}{1., 2., 3.}},
		{testObj, "subobj/subarray/1", 2.},
		{testObj, "subobj/nosuchkey/1", nil},
		{testStruct, "foo", 1},
		{testStruct, "baz", 123.1},
		{testStruct, "array/0/foo", 1},
		{testStruct, "array/0/bar", 0}, // not set from json
		{testStruct, "array/0/0", nil},  // wrong type of key, but strconv doesnt mind
		{testStruct, "array/0/notexist", nil},
		{testStruct, "subobj/subarray", []int{1, 2, 3}},
		{testStruct, "subobj/subsubobj/array/1", "world"},
		{testStruct, "subobj/subarray/0", 1},
		{testStruct, "subobj/subarray/-1", nil},
	} {
		if _, ok := tc.expect.(error); ok {
			v := QQ(tc.root, tc.path)
			if _, ok := v.(error); !ok {
				t.Errorf("%#v [%q]: expected error, got %v (%T) ", tc.root, tc.path, v, v)
			}
			continue
		}

		if v := QQ(tc.root, tc.path); !reflect.DeepEqual(v, tc.expect) {
			if e, ok := v.(error); ok {
				t.Errorf("%#v [%q]:  expected %v, got error: %q", tc.root, tc.path, tc.expect, e)
			} else {
				t.Errorf("%#v [%q]:  expected %v, got %v (%T)", tc.root, tc.path, tc.expect, v, v)
			}
		}
	}
}

func TestString(t *testing.T) {
	if v := String(testStruct, "subobj","subsubobj","array", "1"); v != "world" {
		t.Errorf("%#v [%q]:  expected %v, got %v (%T)", testStruct, "subobj/subsubobj/array/1", "world", v, v)
	}
}

func TestInt(t *testing.T) {
	if v := Int(testStruct, "subobj","subsubobj","bar"); v != 2 {
		t.Errorf("%#v [%q]:  expected %v, got %v (%T)", testStruct, "subobj/subsubobj/bar", 1, v, v)
	}
}

