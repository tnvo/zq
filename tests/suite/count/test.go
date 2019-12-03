package count

import (
	"github.com/mccanne/zq/tests/test"
)

func init() {
	test.Add(test.Detail{
		Name:     "count",
		Query:    "* | count()",
		Input:    input,
		Format:   "table",
		Expected: expected,
	})
}

const input = `
#0:record[_path:string,foo:string]
0:[conn;1;]
0:[conn;2;]
0:[conn;3;]
0:[conn;4;]
0:[conn;5;]
0:[conn;6;]
0:[conn;7;]
0:[conn;8;]
0:[conn;9;]
0:[conn;10;]
`

const expected = `
COUNT
10`