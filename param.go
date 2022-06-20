package httprouter

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Param struct {
	key   string
	value interface{}
}

type Params []Param

// return value by name
func (ps Params) ByName(name string) interface{} {
	for _, p := range ps {
		if p.key == name {
			return p.value
		}
	}
	return nil
}

func (ps Params) GetString(name string) string {
	return ps.ByName(name).(string)
}

func (ps Params) GetInt(name string) int {
	return ps.ByName(name).(int)
}

func (ps Params) GetBytes(name string) []byte {
	return ps.ByName(name).([]byte)
}

// DecodeQuery decode a query to a params
// like Foo=foo&Bar=bar
func DecodeQuery(query string) (Params, error) {
	ps := make(Params, 0)
	pairStrArr := strings.Split(query, "&")
	for _, pairStr := range pairStrArr {

		pairs := strings.Split(pairStr, "=")
		// len of pairs must equal with 2
		if len(pairs) != 2 {

			return nil, errors.Errorf("error format query: %s", pairStr)
		}

		// value like 1,2,3,,4, get [1,2,3,4]
		if strings.Contains(pairs[1], ",") {
			valueSlice := strings.Split(pairs[1], ",")
			vals := []interface{}{}
			for _, value := range valueSlice {

				// to an appropriate type value
				value := toAppropriateType(strings.Trim(value, " "))
				if value != "" {
					vals = append(vals, value)
				}
			}
			ps = append(ps, Param{key: pairs[0], value: vals})

			return ps, nil
		}

		// to an appropriate type value
		val := toAppropriateType(strings.Trim(pairs[1], " "))
		ps = append(ps, Param{key: pairs[0], value: val})
	}

	return ps, nil
}

// toAppropriateType convert value of interface{} to a appropriate type
// int, float32, float64 and bool are returned as it is
// string may convert to int, float64, bool and slice of them.
func toAppropriateType(val interface{}) interface{} {
	typ := reflect.TypeOf(val).Kind()
	if typ == reflect.String {
		str := val.(string)

		// if bool
		valBool, err := strconv.ParseBool(str)
		if err == nil {
			return valBool
		}

		// if int
		valInt, err := strconv.Atoi(str)
		if err == nil {
			return valInt
		}

		// if float
		valFloat, err := strconv.ParseFloat(str, 64)
		if err == nil {
			return valFloat
		}

		// may be a slice of something?
		if strings.Contains(str, ",") {
			strSlice := strings.Split(str, ",")
			valSlice := []interface{}{}
			for _, strstr := range strSlice {
				valSlice = append(valSlice, toAppropriateType(strstr))
			}
			return valSlice
		}

	}
	// as it is
	return val
}
