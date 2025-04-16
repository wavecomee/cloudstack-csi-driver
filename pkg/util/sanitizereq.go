package util

import "reflect"

// SanitizeRequest takes a request object and returns a copy of the request with
// the "Secrets" field cleared.
func SanitizeRequest(req interface{}) interface{} {
	v := reflect.ValueOf(&req).Elem()
	e := reflect.New(v.Elem().Type()).Elem()

	e.Set(v.Elem())

	f := reflect.Indirect(e).FieldByName("Secrets")

	if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
		f.Set(reflect.MakeMap(f.Type()))
		v.Set(e)
	}

	return req
}
