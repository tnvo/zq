package errors

import (
	"github.com/mccanne/zq/pkg/test"
	"github.com/mccanne/zq/pkg/zeek"
)

const inputDuplicateFields = `
#0:record[foo:record[foo:string,bar:string]]
0:[["1";"2";]]
#1:record[foo:record[foo:string,foo:string]]
1:[["1";"2";]]
`

var DuplicateFields = test.Internal{
	Name:         "duplicatefields",
	Query:        "*",
	Input:        test.Trim(inputDuplicateFields),
	OutputFormat: "zng",
	ExpectedErr:  zeek.ErrDuplicateFields,
}