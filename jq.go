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

type quantifier int

const (
	ALL quantifier = iota
)

func (q quantifier) String() string {
	switch q {
	case ALL:
		return "ALL"
	}
	return fmt.Sprintf("<quantifier %d>", int(q))
}

func isSigned(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

// Q recursively queries the root object with the path composed of the indices.
//
// If index has no elements, it returns root.
//
// If root is a struct or a map with a string key type
// and the first element of index is a string,
// it will return Q applied to the corresponding field or map value
// with the remainder of the index.
//
// If root is an array or slice, or a map with integer key type,
// and the first element of index is an integer type or a string that parses as an integer type
// it will return Q applied to the corresponding value with the
// remainder of the index.
//
// If the first element of index is the special value ALL, Q returns multiple values as follows:
// If root is a struct or a map Q returns a map of the field names or keys
// of root to the result of Q applied to the corresponding values with the remainder of the index,
// but any errors are omitted from the result set.
// If root is a slice or array, Q returns a slice of the results of Q
// applied to the elements of root with the remainder of the index.
//
// If the value is not present, Q returns nil, but if the
// index has the wrong type for the root element it will return an error.
func Q(root interface{}, index ...interface{}) interface{} {
	if len(index) == 0 {
		return root
	}

	if i, ok := index[0].(quantifier); ok && i == ALL {
		switch v := reflect.ValueOf(root); v.Kind() {
		case reflect.Struct:
			m := make(map[string]interface{})
			for ii := 0; ii < v.NumField(); ii++ {
				f := v.Type().Field(ii)
				if f.PkgPath != "" { // not exported
					continue
				}
				r := v.Field(ii)
				if !r.IsValid() {
					continue
				}
				rr := Q(r.Interface(), index[1:]...)
				// Fields will typically vary in type, and many of them may not be indexable
				// like the rest of the query requires.  It seems more convenient for the user
				// to just filter these elements out here.
				if _, ok := rr.(error); ok {
					continue
				}
				m[f.Name] = rr
			}
			return m

		case reflect.Map:
			k := v.Type().Key()
			var dum []interface{} // dont know how else to make typeof interface.
			m := reflect.MakeMap(reflect.MapOf(k, reflect.TypeOf(dum).Elem()))
			for _, kk := range v.MapKeys() {
				vv := v.MapIndex(kk)
				rr := Q(vv.Interface(), index[1:]...)
				if rr == nil {
					continue
				}
				// see above
				if _, ok := rr.(error); ok {
					continue
				}
				m.SetMapIndex(kk, reflect.ValueOf(rr))
			}
			return m.Interface()

		case reflect.Array, reflect.Slice:
			var a []interface{}
			for ii := 0; ii < v.Len(); ii++ {
				r := v.Index(ii)
				if r.IsValid() {
					a = append(a, Q(r.Interface(), index[1:]...))
				}
			}
			return a
		}
		return fmt.Errorf("type %T does not support retrieving ALL", root)
	}

	if v, ok := index[0].(quantifier); ok {
		panic(fmt.Errorf("unsupported %s", v))
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
			}
			return fmt.Errorf("cannot use %v (type %T) as map key of type %s", index[0], index[0], k)

		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			switch i := reflect.ValueOf(index[0]); i.Kind() {
			case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if vv := v.MapIndex(i.Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			case reflect.String:
				var idxv reflect.Value
				if isSigned(k.Kind()) {
					idx, err := strconv.ParseInt(i.String(), 0, 64)
					if err != nil {
						return fmt.Errorf("cannot parse %v (type %T) as map key of type %s: %v)", index[0], index[0], k, err)
					}
					idxv = reflect.ValueOf(idx)
				} else {
					idx, err := strconv.ParseUint(i.String(), 0, 64)
					if err != nil {
						return fmt.Errorf("cannot parse %v (type %T) as map key of type %s: %v)", index[0], index[0], k, err)
					}
					idxv = reflect.ValueOf(idx)
				}
				if vv := v.MapIndex(idxv.Convert(k)); vv.IsValid() {
					return Q(vv.Interface(), index[1:]...)
				}
				return nil
			}
			return fmt.Errorf("cannot use %v (type %T) as map key of type %s", index[0], index[0], k)
		}
		return fmt.Errorf("map key type %s not supported", v.Type().Key())

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
		}
		return fmt.Errorf("cannot use %v (type %T) as array index", index[0], index[0])
	}

	return fmt.Errorf("type %T does not support indexing", root)
}

// QQ splits the single argument 'index' on slashes and calls Q with the resulting index array.
// an index element named "*" will be mapped to the jq.ALL value.
func QQ(root interface{}, index string) interface{} {
	var pp []interface{}
	if index != "" {
		parts := strings.Split(index, "/")
		for _, v := range parts {
			if v == "*" {
				pp = append(pp, ALL)
			} else {
				pp = append(pp, v)
			}
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

// Bool returns the truth value according to javascript rules.
func Bool(root interface{}, index ...interface{}) bool {
	switch vv := Q(root, index...).(type) {
	case string:
		return vv != ""
	case bool:
		return vv
	case int:
		return vv != 0
	case int8:
		return vv != 0
	case int16:
		return vv != 0
	case int32:
		return vv != 0
	case int64:
		return vv != 0
	case uint:
		return vv != 0
	case uint8:
		return vv != 0
	case uint16:
		return vv != 0
	case uint32:
		return vv != 0
	case uint64:
		return vv != 0
	case json.Number:
		n, err := vv.Int64()
		if err != nil {
			return false
		}
		return n != 0
	}
	return false
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
// time object found at that path, or the zero time in all other cases.
func Time(root interface{}, index ...interface{}) time.Time {
	switch v := Q(root, index...).(type) {
	case string:
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	case time.Time:
		return v
	}
	return zeroTime
}
