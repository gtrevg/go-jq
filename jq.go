/*
  Package jq provides functions to resolve paths of the form fld1/fld2/42/fld/4 to be resolved
  in nested structures as returned by e.g. json.Unmarshal.

  TODO the decision of returning nil (not found) or an error is not entirely consistent yet.
*/
package jq

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Q queries the root object with the path composed of the indices.
//
// If the index is empty, it returns root.
//
// Otherwise, if root is a struct or a map with a string key type
// and the index is a string, it will return Q applied to the corresponding
// field or map value with the remainder of the index.
//
// Otherwise, if root is an array or slice, or a map with integer key type,
// and the index is an integer type or a string that parses as an integer type
// it will return Q applied to the corresponding value with the
// remainder of the index.
//
// If the value is not present, Q returns nil, but if the
// index has the wrong type for the root element it will return an error.
func Q(root interface{}, index ...interface{}) interface{} {
	if len(index) == 0 {
		return root
	}

	switch v := reflect.ValueOf(root); v.Kind() {
	case reflect.Struct:
		switch i := reflect.ValueOf(index[0]); i.Kind() {
		case reflect.String:
			r := v.FieldByName(strings.Title(i.String())) // get the corresponding exported field only
			if r.IsValid() {
				return Q(r.Interface(), index[1:]...)
			}
			return nil
		}
		return fmt.Errorf("cannot use %v (type %T) as struct field name", index[0], index[0])

	case reflect.Map:
		switch k := v.Type().Key(); k.Kind() {
		case reflect.String:
			switch i := reflect.ValueOf(index[0]); i.Kind() {
			case reflect.String:
				if vv := v.MapIndex(i); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			default:
				return fmt.Errorf("cannot use %v (type %T) as map key of type %s", index[0], index[0], k)
			}

		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			switch i := reflect.ValueOf(index[0]); i.Kind() {
			case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if vv := v.MapIndex(i.Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			case reflect.String:
				idx, err := strconv.ParseUint(i.String(), 0, 64)
				if err != nil {
					return fmt.Errorf("cannot parse %v (type %T) as map key of type %s: %v)", index[0], index[0], k, err)
				}
				if vv := v.MapIndex(reflect.ValueOf(idx).Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			default:
				return fmt.Errorf("cannot use %v (type %T) as map key of type %s", index[0], index[0], k)
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			switch i := reflect.ValueOf(index[0]); i.Kind() {
			case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if vv := v.MapIndex(i.Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			case reflect.String:
				idx, err := strconv.ParseInt(i.String(), 0, 64)
				if err != nil {
					return fmt.Errorf("cannot parse %v (type %T) as map key of type %s: %v)", index[0], index[0], k, err)
				}
				if vv := v.MapIndex(reflect.ValueOf(idx).Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			default:
				return fmt.Errorf("cannot use %v (type %T) as map key of type %s", index[0], index[0], k)
			}

		default:
			return fmt.Errorf("map key type %s not supported", k)
		}

	case reflect.Array, reflect.Slice:
		switch i := reflect.ValueOf(index[0]); i.Kind() {
		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if ii := i.Uint(); ii < uint64(v.Len()) {
				return Q(v.Index(int(ii)).Interface(), index[1:]...)
			}
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if ii := i.Int(); 0 <= ii && ii < int64(v.Len()) {
				return Q(v.Index(int(ii)).Interface(), index[1:]...)
			}
			return nil
		case reflect.String:
			idx, err := strconv.ParseInt(i.String(), 0, 64)
			if err != nil {
				return fmt.Errorf("cannot parse %v (type %T) as array index: %v)", index[0], index[0], err)
			}
			if 0 <= idx && idx < int64(v.Len()) {
				return Q(v.Index(int(idx)).Interface(), index[1:]...)
			}
			return nil
		default:
			return fmt.Errorf("cannot use %v (type %T) as array index", index[0], index[0])
		}
	}
	return fmt.Errorf("type %T does not support indexing", root)
}

// QQ splits the single argument 'index' on slashes and calls Q with the resulting index array
func QQ(root interface{}, index string) interface{} {
	var pp []interface{}
	if index != "" {
		parts := strings.Split(index, "/")
		for _, v := range parts {
			pp = append(pp, v)
		}
	}
	return Q(root, pp...)
}

// String returns the string found at path or the empty string in all other cases.
func String(root interface{}, index ...interface{}) string {
	switch vv := Q(root, index...).(type) {
	case string:
		return vv
	case json.Number:
		return vv.String()
	}
	return ""
}

// Int returns the integer found at path or 0 in all other cases.
// Integers of any size and sign are cast to plain int, with possible loss of information.
// Int also handles the json.Number type that may be returned by json.Unmarshal.
func Int(root interface{}, index ...interface{}) int {
	switch vv := Q(root, index...).(type) {
	case int:
		return vv
	case int8:
		return int(vv)
	case int16:
		return int(vv)
	case int32:
		return int(vv)
	case int64:
		return int(vv)
	case uint:
		return int(vv)
	case uint8:
		return int(vv)
	case uint16:
		return int(vv)
	case uint32:
		return int(vv)
	case uint64:
		return int(vv)
	case json.Number:
		n, err := vv.Int64()
		if err != nil {
			return 0
		}
		return int(n)
	}
	return 0
}

var zeroTime time.Time

// Time returns the string found at path, parsed as an RFC3339 formatted date
// and time "2006-01-02T15:04:05Z07:00" with optional fractional second or the
// zero time in all other cases.
func Time(root interface{}, index ...interface{}) time.Time {
	s, ok := Q(root, index...).(string)
	if !ok {
		return zeroTime
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return zeroTime
}
