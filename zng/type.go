// Package zng implements a data typing system based on the zeek type system.
// All zeek types are defined here and implement the Type interface while instances
// of values implement the Value interface.  All values conform to exactly one type.
// The package provides a fast-path for comparing a value to a byte slice
// without having to create a zeek value from the byte slice.  To exploit this,
// all values include a Comparison method that returns a Predicate function that
// takes a byte slice and a Type and returns a boolean indicating whether the
// the byte slice with the indicated Type matches the value.  The package also
// provides mechanism for coercing values in well-defined and natural ways.
package zng

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mccanne/zq/zcode"
)

var (
	ErrLenUnset     = errors.New("len(unset) is undefined")
	ErrNotContainer = errors.New("argument to len() is not a container")
	ErrNotVector    = errors.New("cannot index a non-vector")
	ErrIndex        = errors.New("vector index out of bounds")
)

// A Type is an interface presented by a zeek type.
// Types can be used to infer type compatibility and create new values
// of the underlying type.
type Type interface {
	String() string
	// New returns a Value of this Type with a value determined from the
	// zval encoding.  For records, sets, and vectors, the zval is a container
	// encoding of the of the body of values of that type.
	New(zcode.Bytes) (Value, error)
	// Parse transforms a string represenation of the type to its zval
	// encoding.  The string input is provided as a byte slice for efficiency
	// given the common use cases in the system.
	Parse([]byte) (zcode.Bytes, error)
}

var (
	TypeBool     = &TypeOfBool{}
	TypeCount    = &TypeOfCount{}
	TypeInt      = &TypeOfInt{}
	TypeDouble   = &TypeOfDouble{}
	TypeTime     = &TypeOfTime{}
	TypeInterval = &TypeOfInterval{}
	TypeString   = &TypeOfString{}
	TypePattern  = &TypeOfPattern{}
	TypePort     = &TypeOfPort{}
	TypeAddr     = &TypeOfAddr{}
	TypeSubnet   = &TypeOfSubnet{}
	TypeEnum     = &TypeOfEnum{}
	TypeUnset    = &TypeOfUnset{}
)

var typeMapMutex sync.RWMutex
var typeMap = map[string]Type{
	"bool":     TypeBool,
	"count":    TypeCount,
	"int":      TypeInt,
	"double":   TypeDouble,
	"time":     TypeTime,
	"interval": TypeInterval,
	"string":   TypeString,
	"pattern":  TypePattern,
	"regexp":   TypePattern, // zql
	"port":     TypePort,
	"addr":     TypeAddr,
	"subnet":   TypeSubnet,
	"enum":     TypeEnum,
	"unset":    TypeUnset, // zql
}

// SameType returns true if the two types are equal in that each interface
// points to the same underlying type object.  Because the zeek library
// creates each unique type only once, this pointer comparison works.  If types
// are created outside of the zeek package, then SameType will not work in general
// for them.
func SameType(t1, t2 Type) bool {
	return t1 == t2
}

// addType adds a type to the type lookup map.  It is possible that there is
// a race here when two threads try to create a new type at the same time,
// so the first one wins.  This way there cannot be types that are the same
// that have different pointers, so SameType will work correctly.
func addType(t Type) Type {
	typeMapMutex.Lock()
	defer typeMapMutex.Unlock()
	key := t.String()
	old, ok := typeMap[key]
	if ok {
		t = old
	} else {
		typeMap[key] = t
	}
	return t
}

func isIdChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.'
}

func parseWord(in string) (string, string) {
	in = strings.TrimSpace(in)
	var off int
	for ; off < len(in); off++ {
		if !isIdChar(in[off]) {
			break
		}
	}
	if off == 0 {
		return "", ""
	}
	return in[off:], in[:off]
}

// LookupType returns the Type indicated by the zeek type string.  The type string
// may be a simple type like int, double, time, etc or it may be a set
// or a vector, which are recusively composed of other types.  The set and vector
// type definitions are encoded in the same fashion as zeek stores them as type field
// in a zeek file header.  Each unique compound type object is created once and
// interned so that pointer comparison can be used to determine type equality.
func LookupType(in string) (Type, error) {
	//XXX check if rest has junk and flag an error?
	_, typ, err := parseType(in)
	return typ, err
}

// LookupVectorType returns the VectorType for the provided innerType.
func LookupVectorType(innerType Type) Type {
	return addType(&TypeVector{innerType})
}

func parseType(in string) (string, Type, error) {
	typeMapMutex.RLock()
	t, ok := typeMap[strings.TrimSpace(in)]
	typeMapMutex.RUnlock()
	if ok {
		return "", t, nil
	}
	rest, word := parseWord(in)
	if word == "" {
		return "", nil, fmt.Errorf("unknown type: %s", in)
	}
	typeMapMutex.RLock()
	t, ok = typeMap[word]
	typeMapMutex.RUnlock()
	if ok {
		return rest, t, nil
	}
	switch word {
	case "set":
		rest, t, err := parseSetTypeBody(rest)
		if err != nil {
			return "", nil, err
		}
		return rest, addType(t), nil
	case "vector":
		rest, t, err := parseVectorTypeBody(rest)
		if err != nil {
			return "", nil, err
		}
		return rest, addType(t), nil
	case "record":
		rest, t, err := parseRecordTypeBody(rest)
		if err != nil {
			return "", nil, err
		}
		return rest, addType(t), nil
	}
	return "", nil, fmt.Errorf("unknown type: %s", word)
}

// Utilities shared by compound types (ie, set and vector)

// InnerType returns the element type for set and vector types
// or nil if the type is not a set or vector.
func InnerType(typ Type) Type {
	switch typ := typ.(type) {
	case *TypeSet:
		return typ.innerType
	case *TypeVector:
		return typ.typ
	default:
		return nil
	}
}

// ContainedType returns the inner type for set and vector types in the first
// return value and the columns of its of type for record types in the second
// return value.  ContainedType returns nil for both return values if the
// type is not a set, vector, or record.
func ContainedType(typ Type) (Type, []Column) {
	switch typ := typ.(type) {
	case *TypeSet:
		return typ.innerType, nil
	case *TypeVector:
		return typ.typ, nil
	case *TypeRecord:
		return nil, typ.Columns
	default:
		return nil, nil
	}
}

func IsContainerType(typ Type) bool {
	switch typ.(type) {
	case *TypeSet, *TypeVector, *TypeRecord:
		return true
	default:
		return false
	}
}

func trimInnerTypes(typ string, raw string) string {
	// XXX handle white space, "set [..."... ?
	innerTypes := strings.TrimPrefix(raw, typ+"[")
	innerTypes = strings.TrimSuffix(innerTypes, "]")
	return innerTypes
}

// Given a predicate for comparing individual elements, produce a new
// predicate that implements the "in" comparison.  The new predicate looks
// at the type of the value being compared, if it is a set or vector,
// the original predicate is applied to each element.  The new precicate
// returns true iff the predicate matched an element from the collection.
func Contains(compare Predicate) Predicate {
	return func(e TypedEncoding) bool {
		var el TypedEncoding
		switch typ := e.Type.(type) {
		case *TypeSet:
			el.Type = typ.innerType
		case *TypeVector:
			el.Type = typ.typ
		default:
			return false
		}

		for it := e.Iter(); !it.Done(); {
			var err error
			el.Body, _, err = it.Next()
			if err != nil {
				return false
			}
			if compare(el) {
				return true
			}
		}
		return false
	}
}

func ContainerLength(e TypedEncoding) (int, error) {
	switch e.Type.(type) {
	case *TypeSet, *TypeVector:
		if e.Body == nil {
			return -1, ErrLenUnset
		}
		var n int
		for it := e.Iter(); !it.Done(); {
			if _, _, err := it.Next(); err != nil {
				return -1, err
			}
			n++
		}
		return n, nil
	default:
		return -1, ErrNotContainer
	}
}

func (e TypedEncoding) Iter() zcode.Iter {
	return zcode.Iter(e.Body)
}

func (e TypedEncoding) String() string {
	v, err := e.Type.New(e.Body)
	if err != nil {
		return fmt.Sprintf("Err stringify type %s: %s", e.Type, err)
	}
	var b strings.Builder
	b.WriteString(e.Type.String())
	b.WriteByte(':')
	if IsContainerType(e.Type) {
		b.WriteByte('[')
		b.WriteString(v.String())
		b.WriteByte(']')
	} else {
		b.WriteByte('(')
		b.WriteString(v.String())
		b.WriteByte(')')
	}
	return b.String()
}

// If the passed-in element is a vector, attempt to get the idx'th
// element, and return its type and raw representation.  Returns an
// error if the passed-in element is not a vector or if idx is
// outside the vector bounds.
func (e TypedEncoding) VectorIndex(idx int64) (TypedEncoding, error) {
	vec, ok := e.Type.(*TypeVector)
	if !ok {
		return TypedEncoding{}, ErrNotVector
	}
	if idx < 0 {
		return TypedEncoding{}, ErrIndex
	}
	for i, it := 0, e.Iter(); !it.Done(); i++ {
		zv, _, err := it.Next()
		if err != nil {
			return TypedEncoding{}, err
		}
		if i == int(idx) {
			return TypedEncoding{vec.typ, zv}, nil
		}
	}
	return TypedEncoding{}, ErrIndex
}

// Elements returns an array of TypedEncodings for the current container type.
// Returns an error if the element is not a vector or set.
func (e TypedEncoding) Elements() ([]TypedEncoding, error) {
	innerType := InnerType(e.Type)
	if innerType == nil {
		return nil, ErrNotContainer
	}
	var elements []TypedEncoding
	for it := e.Iter(); !it.Done(); {
		zv, _, err := it.Next()
		if err != nil {
			return nil, err
		}
		elements = append(elements, TypedEncoding{innerType, zv})
	}
	return elements, nil
}

// LookupTypeRecord returns a zeek.TypeRecord for the indicated columns.  If it
// already exists, the existent interface pointer is returned.  Otherwise,
// it is created and returned.
func LookupTypeRecord(columns []Column) *TypeRecord {
	s := recordString(columns)
	typeMapMutex.RLock()
	t, ok := typeMap[s]
	typeMapMutex.RUnlock()
	if ok {
		return t.(*TypeRecord)
	}
	typeMapMutex.Lock()
	defer typeMapMutex.Unlock()
	t, ok = typeMap[s]
	if ok {
		return t.(*TypeRecord)
	}
	// Make a private copy of the columns to maintain the invariant
	// that types are immutable and the columns can be retrieved from
	// the type system and traversed without any data races.
	private := make([]Column, len(columns))
	for k, p := range columns {
		private[k] = p
	}
	rec := &TypeRecord{Columns: private, Key: s}
	typeMap[s] = rec
	return rec
}

// NewValue creates a Value with the given type and value described
// as simple strings.
func NewValue(typ, val string) (Value, error) {
	t, err := LookupType(typ)
	if err != nil {
		return nil, err
	}
	zv, err := t.Parse([]byte(val))
	if err != nil {
		return nil, err
	}
	return t.New(zv)
}

// Format tranforms a zval encoding with its type encoding to a
// a human-readable (and zng text-compliant) string format
// encoded as a byte slice.
//XXX this could be more efficient
func Format(typ Type, zv zcode.Bytes) ([]byte, error) {
	val, err := typ.New(zv)
	if err != nil {
		return nil, err
	}
	return []byte(Escape([]byte(val.String()))), nil
}