package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"reflect"
)

// Digest fingerprints every mutable native state field, including private
// controller counters. Immutable source banks are identified by the replay's
// source fingerprint and are not hashed on each gameplay checkpoint.
func (e *Engine) Digest() [32]byte {
	h := sha256.New()
	v := reflect.ValueOf(e).Elem()
	for index := 0; index < v.NumField(); index++ {
		name := v.Type().Field(index).Name
		if name == "Data" || name == "initialSurfaceEvents" {
			continue
		}
		digestValue(h, v.Field(index))
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

func digestValue(h hash.Hash, v reflect.Value) {
	var encoded [8]byte
	write := func(value uint64) { binary.LittleEndian.PutUint64(encoded[:], value); h.Write(encoded[:]) }
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			write(1)
		} else {
			write(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		write(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		write(v.Uint())
	case reflect.String:
		s := v.String()
		write(uint64(len(s)))
		h.Write([]byte(s))
	case reflect.Array, reflect.Slice:
		write(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			digestValue(h, v.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			digestValue(h, v.Field(i))
		}
	case reflect.Pointer:
		if v.IsNil() {
			write(0)
		} else {
			write(1)
			digestValue(h, v.Elem())
		}
	}
}
