package utils

import (
	"crypto/rand"
	"encoding/base64"
	"math"
	"reflect"
	"unsafe"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

//
// Util - Ternary:
// A golang equivalent to JS Ternary Operator
//
// It takes a condition, and returns a result depending on the outcome
//
func Ternary(condition bool, whenTrue interface{}, whenFalse interface{}) interface{} {
	if condition {
		return whenTrue
	}

	return whenFalse
}

//
// Util - Is Power Of Two
//
func IsPowerOfTwo(n int64) bool {
	return (n != 0) && ((n & (n - 1)) == 0)
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func B2S(b []byte) string {
	/* #nosec G103 */
	return *(*string)(unsafe.Pointer(&b))
}

// S2B converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func S2B(s string) (b []byte) {
	/* #nosec G103 */
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	/* #nosec G103 */
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}

func DifferentArray(a []string, b []string) bool {
	if len(a) != len(b) {
		return true
	}
	if len(a) == 0 {
		return false
	}
	aM := make(map[string]int)
	bM := make(map[string]int)
	for _, v := range a {
		aM[v] = 1
	}
	for _, v := range b {
		bM[v] = 1
		if _, ok := aM[v]; !ok {
			return true
		}
	}
	for k := range aM {
		if _, ok := bM[k]; !ok {
			return true
		}
	}
	return false
}

func IsSliceArray(v interface{}) bool {
	k := reflect.TypeOf(v).Kind()
	return k == reflect.Slice || k == reflect.Array
}

func IsSliceArrayPointer(v interface{}) bool {
	n := reflect.TypeOf(v)
	k := n.Kind()
	if k == reflect.Ptr {
		k = n.Elem().Kind()
		return k == reflect.Slice || k == reflect.Array
	}
	return false
}

func SliceIndexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}

	return -1
}

func Contains(s []string, compare string) bool {
	for _, v := range s {
		if v == compare {
			return true
		}
	}

	return false
}

func ContainsObjectID(oid []primitive.ObjectID, compare primitive.ObjectID) bool {
	for _, v := range oid {
		if v == compare {
			return true
		}
	}

	return false
}

func IsPointer(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Ptr
}

func StringPointer(s string) *string {
	return &s
}

func Int32Pointer(i int32) *int32 {
	return &i
}

func Int64Pointer(i int64) *int64 {
	return &i
}

func BoolPointer(b bool) *bool {
	return &b
}

// Obtain the size ratio of width and height values
// For image resizing
func GetSizeRatio(og []float64, nw []float64) (int32, int32) {
	ratio := math.Min(nw[0]/og[0], nw[1]/og[1])

	var width int32 = int32(math.Floor(og[0] * ratio))
	var height int32 = int32(math.Floor(og[1] * ratio))

	return width, height
}

type Key string

const UserKey = Key("user")
const RequestCtxKey = Key("RequestCtx")
const AllRolesKey = Key("AllRoles")
