# ZNG Specification

> Note: This specification is ALPHA and a work in progress.
> Zq's implementation of ZNG is tracking this spec and as it changes,
> the zq output format is subject to change.  In this branch,
> zq attempts to implement everything herein excepting:
>
> * the bytes type is not yet implemented,
> * ordering hints are not generated or taken advantage of,
> * it is not yet possible to validate enums against a set of allowed values.
>
> Also, we are contemplating reducing the number of primitive types, e.g.,
> the number of variations in integer types.

ZNG is a format for structured data values, ideally suited for streams
of heterogeneously typed records, e.g., structured logs, where filtering and
analytics may be applied to a stream in parts without having to fully deserialize
every value.

ZNG has both a text form simply called "ZNG",
comprised of a sequence of newline-delimited UTF-8 strings,
as well as a binary form called "BZNG".

ZNG is richly typed and thinner on the wire than JSON.
Like [newline-delimited JSON (NDJSON)](http://ndjson.org/),
the ZNG text format represents a sequence of data objects
that can be parsed line by line.
ZNG strikes a balance between the narrowly typed but flexible NDJSON format and
a more structured approach like
[Apache Avro](https://avro.apache.org).

ZNG is type rich and embeds all type information in the stream while having a
binary serialization format that allows "lazy parsing" of fields such that
only the fields of interest in a stream need to be deserialized and interpreted.
Unlike Avro, ZNG embeds its schemas in the data stream and thereby admits
an efficient multiplexing of heterogeneous data types by prepending to each
data value a simple integer identifier to reference its type.   

ZNG requires no external schema definitions as its type system
constructs schemas on the fly from within the stream using composable,
dynamic type definitions.  Given this, there is no need for
a schema registry service, though ZNG can be readily adapted to systems like
[Apache Kafka](https://kafka.apache.org/) which utilize such registries,
by having a connector translate the schemas implied in the
ZNG stream into registered schemas and vice versa.

ZNG is a superset of JSON in that any JSON input
can be mapped onto ZNG and recovered by decoding
that ZNG back into JSON.

The ZNG design [was motivated by](./zeek-compat.md)
and is compatible with the
[Zeek log format](https://docs.zeek.org/en/stable/examples/logs/).
As far as we know, the Zeek log format pioneered the concept of
embedding the schema of the log lines within the log file itself using
meta-records, and ZNG merely modernizes this original approach.

## 1. The ZNG data model

ZNG encodes a sequence of one or more typed data values to comprise a stream.
The stream of values is interleaved with control messages
that provide type definitions and other metadata.  The type of
a particular data value is specified by its "type identifier", or type ID,
which is an integer representing either a "primitive type" or a
"container type".

The ZNG type system comprises the standard set of primitive types like integers,
floating point, strings, byte arrays, etc. as well as container types
like records, arrays, and sets arranged from the primitive types.

For example, a ZNG stream representing the single string "hello world"
looks like this:
```
9:hello, world
```
Here, the type ID is the integer "9" representing the string type
(defined in [Typedefs](#typedefs)) and the data value "hello, world"
is an instance of string.

ZNG gets more interesting when different data types are interleaved in the stream.
For example,
```
9:hello, world
4:42
9:there's a fly in my soup!
9:no, there isn't.
4:3
```
where type ID 4 represents an integer.  This encoding represents the sequence of
values:
```
"hello, world"
42
"there's a fly in my soup!"
"no, there isn't."
3
```
ZNG streams are often comprised as a sequence of records, which works well to provide
an efficient representation of structured logs.  In this case, a new type ID is
needed to define the schema for each distinct record.  To define a new
type, the "#" syntax is used.  For example,
logs from the open-source Zeek system might look like this
```
#24:record[_path:string,ts:time,uid:string,id:record[orig_h:ip,orig_p:port,resp_h:ip,resp_p:port]...
#25:record[_path:string,ts:time,fuid:string,tx_hosts:set[ip]...
24:[conn;1425565514.419939;CogZFI3py5JsFZGik;[192.168.1.1:;80/tcp;192.168.1.2;8080;]...
25:[files;1425565514.419987;Fj8sRF1gdneMHN700d;[52.218.49.89;52.218.48.169;]...
```
Note that the value encoding need not refer to the field names and types as that is
completely captured by the type ID.  Values merely encode the value
information consistent with the referenced type ID.

## 2. ZNG Binary Format (BZNG)

The BZNG binary format is based on machine-readable data types with an
encoding methodology inspired by Avro and
[Protocol Buffers](https://developers.google.com/protocol-buffers).

A BZNG stream comprises a sequence of interleaved control messages and value messages
that are serialized into a stream of bytes.

Each message is prefixed with a single-byte header code.  The upper bit of
the header code indicates whether the message is a control message (1)
or a value message (0).

### 2.1 Control Messages

The lower 7 bits of a control header byte define the control code.
Control codes 0 through 5 are reserved for BZNG:

| Code | Message Type      |
|------|-------------------|
|  `0` | record definition |
|  `1` | array definition  |
|  `2` | set definition    |
|  `3` | union definition  |
|  `4` | type alias        |
|  `5` | ordering hint     |

All other control codes are available to higher-layer protocols to carry
application-specific payloads embedded in the ZNG stream.

Any such application-specific payloads not known by
a ZNG data receiver shall be ignored.

The body of an application-specific control message is any UTF-8 string.
These payloads are guaranteed to be preserved
in order within the stream and presented to higher layer components through
any ZNG streaming API.  In this way, senders and receivers of ZNG can embed
protocol directives as ZNG control payloads rather than defining additional
encapsulating protocols.  See the
[zng-over-http](zng-over-http.md) protocol for an example.

### 2.1.1 Typedefs

Following a header byte of 0x80-0x83 is a "typedef".  A typedef binds
"the next available" integer type ID to a type encoding.  Type IDs
begin at the value 23 and increase by one for each typedef. These bindings
are scoped to the stream in which the typedef occurs.

Type IDs for the "primitive types" need not be defined with typedefs and
are predefined as follows:

<table>
<tr><td>

| Type       | ID |
|------------|------|
| `bool`     |   0  |
| `byte`     |   1  |
| `int16`    |   2  |
| `uint16`   |   3  |
| `int32`    |   4  |
| `uint32`   |   5  |
| `int64`    |   6  |
| `uint64`   |   7  |
| `float64`  |   8  |
| `string`   |   9  |

</td><td>

| Type       | ID |
|------------|------|
| `bytes`    |  10  |
| `bstring`  |  11  |
| `enum`     |  12  |
| `ip`       |  13  |
| `port`     |  14  |
| `net`      |  15  |
| `time`     |  16  |
| `duration` |  17  |
| `null`     |  18  |
| &nbsp;     |      |

</td></tr> </table>

A typedef is encoded as a single byte indicating the container type ID following by
the type encoding.  This creates a binding between the implied type ID
(i.e., 23 plus the count of all previous typedefs in the stream) and the new
type definition.

The type ID is encoded as a `uvarint`, an encoding used throughout the BZNG format.

> Inspired by Protocol Buffers,
> a `uvarint` is an unsigned, variable-length integer encoded as a sequence of
> bytes consisting of N-1 bytes with bit 7 clear and the Nth byte with bit 7 set,
> whose value is the base-128 number composed of the digits defined by the lower
> 7 bits of each byte from least-significant digit (byte 0) to
> most-significant digit (byte N-1).

#### 2.1.1.1 Record Typedef

A record typedef creates a new type ID equal to the next stream type ID
with the following structure:
```
----------------------------------------------------------
|0x80|<nfields>|<field1><type-id-1><field2><type-id-2>...|
----------------------------------------------------------
```
Record types consist of an ordered set of columns where each column consists of
a name and a typed value.  Unlike JSON, the ordering of the columns is significant
and must be preserved through any APIs that consume, process, and emit ZNG records.

A record type is encoded as a count of fields, i.e., `<nfields>` from above,
followed by the field definitions,
where a field definition is a field name followed by a type ID, i.e.,
`<field1>` followed by `<type-id-1>` etc. as indicated above.

The field names in a record must be unique.

The `<nfields>` is encoded as a `uvarint`.

The field name is encoded as a UTF-8 string defining a "ZNG identifier"
The UTF-8 string
is further encoded as a "counted string", which is `uvarint` encoding
of the length of the string followed by that many bytes of UTF-8 encoded
string data.

N.B.: The rules for ZNG identifiers follow the same rules as
[JavaScript identifiers](https://tc39.es/ecma262/#prod-IdentifierName).

The type ID follows the field name and is encoded as a `uvarint`.

A record may not contain zero columns.

#### 2.1.1.2 Array Typedef

An array type is encoded as simply the type code of the elements of
the array encoded as a `uvarint`:
```
----------------
|0x81|<type-id>|
----------------
```

#### 2.1.1.3 Set Typedef

A set type is encoded as a type count followed by the type ID of the
elements of the set, each encoded as a `<uvarint>`:
```
-------------------------
|0x82|<ntypes>|<type-id>|
-------------------------
```

`<ntypes>` must be 1.

`<type-id>` must be a primitive type ID.

#### 2.1.1.4 Union Typedef

A union typedef creates a new type ID equal to the next stream type ID
with the following structure:
```
-----------------------------------------
|0x83|<ntypes>|<type-id-1><type-id-2>...|
-----------------------------------------
```
A union type consists of an ordered set of types
encoded as a count of the number of types, i.e., `<ntypes>` from above,
followed by the type IDs comprising the types of the union.
The type IDs of a union must be unique.

The `<ntypes>` and the type IDs are all encoded as `uvarint`.

`<ntypes>` cannot be 0.

#### 2.1.1.5 Alias Typedef

A type alias defines a new type ID that binds a new type name
to a previously existing type ID.  This is useful for systems like Zeek,
where there are customary type names that are well-known to users of the
Zeek system and are easily mapped onto a BZNG type having a different name.
By encoding the aliases in the format, there is no need to configure mapping
information across different systems using the format, as the type aliases
are communicated to the consumer of a BZNG stream.

A type alias is encoded as follows:
```
----------------------
|0x84|<name><type-id>|
----------------------
```
where `<name>` is an identifier representing the new type name with a new type ID
allocated as the next available type ID in the stream that refers to the
existing type ID ``<type-id>``.  ``<type-id>`` is encoded as a `uvarint` and `<name>`
is encoded as a `uvarint` representing the length of the name in bytes,
followed by that many bytes of UTF-8 string.

### 2.1.2 Ordering Hint

An ordering hint provides a means to indicate that data in the stream
is sorted a certain way.

The hint is encoded as follows:
```
---------------------------------------
|0x84|<len>|[+-]<field>,[+-]<field>,...
---------------------------------------
```
where the payload of the message is a length-counted UTF-8 string.
`<len>` is a `uvarint` indicating the length in bytes of the UTF-8 string
describing the ordering hint.

In the hint string, `[+-]` indicates either `+` or `-` and `<field>` refers
to the top-level field name in a record of any subsequent record value encountered
from thereon in the stream with the field names specified.
The hint guarantees that all subsequent value lines will
appear sorted in the file or stream, in ascending order in the case of `+` and
descending order in the case of `-`, according to the field provided.
If more than one sort
field is provided, then the values are guaranteed to be sorted by each
subsequent key for values that have previous keys of equal value.

It is an error for any such values to appear that contradicts the most
recent ordering directives.

### 2.2 BZNG Value Messages

Following a header byte with bit 7 zero is a `typed value`
with a `uvarint7` encoding its length.

> A `uvarint7` is the same as a `uvarint` except only 7 bits instead of 8
> are available in the first byte.  Its value is equal to the lower 6-bits if bit 6
> of the first byte is 1; otherwise it is that value plus the value of the
> subsequent `uvarint` times 64.

A `typed value` is encoded as either a `uvarint7` (in a top-level value message)
or `uvarint` (for any other values)
encoding the length in bytes of the type ID and value followed by
the body of the typed value comprising that many bytes.
Within the body of the typed value,
the type ID is encoded as a `uvarint` and the value is encoded
as a byte array whose length is equal to the body length less the
length in bytes of the type ID.
```
------------------------
|uvarint7|type-id|value|
------------------------
```

A typed value with a `value` of length N and the type indicated
is interpreted as follows:

| Type       | N        |              Value                               |
|------------|----------|--------------------------------------------------|
| `bool`     | 1        |  one byte 0 (false) or 1 (true)                  |
| `byte`     | 1        |  the byte                                        |
| `int16`    | variable |  signed int of length N                          |
| `uint16`   | variable |  unsigned int of length N                        |
| `int32`    | variable |  signed int of length N                          |
| `uint32`   | variable |  unsigned int of length N                        |
| `int64`    | variable |  signed int of length N                          |
| `uint64`   | variable |  unsigned int of length N                        |
| `float64`  | 8        |  8 bytes of IEEE 64-bit format                   |
| `string`   | variable |  UTF-8 byte sequence of string                   |
| `bytes`    | variable |  bytes of value                                  |
| `bstring`  | variable |  UTF-8 byte sequence with `\x` escapes           |
| `enum `    | variable |  UTF-8 bytes of enum string                      |
| `ip`       | 4 or 16  |  4 or 16 bytes of IP address                     |
| `net`      | 8 or 32  |  8 or 32 bytes of IP prefix and subnet mask      |
| `time`     | 8        |  8 bytes of signed nanoseconds from epoch        |
| `duration` | 8        |  8 bytes of signed nanoseconds duration          |
| `null`     | 0        |  No value, always represents an undefined value  |

All multi-byte sequences representing machine words are serialized in
little-endian format.

> Note: The bstring type is an unusual type representing a hybrid type
> mixing a UTF-8 string with embedded binary data.   This type is
> useful in systems like Zeek where data is pulled off the network
> while expecting a string, but there can be embedded binary data due to
> bugs, malicious attacks, etc.  It is up to the receiver to determine
> with out-of-band information or inference whether the data is ultimately
> arbitrary binary data or a valid UTF-8 string.

A union value is encoded as a container with two elements. The first
element is the uvarint encoding of the index determining the type of
the value in reference to the union type, and the second element is
the value encoded according to that type.

Array, set, and record types are variable length and are encoded
as a sequence of elements:

| Type     |          Value                       |
|----------|--------------------------------------|
| `array`  | concatenation of elements            |
| `set`    | normalized concatenation of elements |
| `record` | concatenation of elements            |

Since N, the byte length of any of these container values, is known,
there is no need to encode a count of the
elements present.  Also, since the type ID is implied by the typedef
of any container type, each value is encoded without its type ID.

The concatenation of elements is encoded as a sequence of "tag-counted" values.
A tag carries both the length information of the corresponding value as well
a "container bit" to differentiate between primitive values and container values
without having to refer to the implied type.  This admits an efficient implementation
for traversing the values, inclusive of recursive traversal of container values,
whereby the inner loop need not consult and interpret the type ID of each element.

The tag encodes the length N of the value and indicates whether
it is a primitive value or a container value.
The length is offset by 1 whereby length of 0 represents an unset value
analogous to null in JSON.
The container bit is 1 for container values and 0 for primitive values.
The tag is defined as
```
2*(N+1) + the container bit
```
and is encoded as a `uvarint`.

For example, tag value 0 is an unset primitive value and tag value 1
is an unset container value.  Tag value 2 is a length zero primitive value,
e.g., it could represent empty string.  Tag value 3 is a length 1 primitive value,
e.g., it would represent the boolean "true" if followed by byte value 1
in the context of type ID 0 (i.e., the type ID for boolean).

Following the tag encoding is the value encoded in N bytes as described above.

For sets, the concatenation of elements must be normalized so that the
sequence of bytes encoding each element's tag-counted value is
lexicographically greater than that of the preceding element.

## 3. ZNG Text Format

The ZNG text format is a human-readable form that follows directly from the BZNG
binary format.  A ZNG file/stream is encoded with UTF-8.
All subsequent references to characters and strings in this section refer to
the Unicode code points that result when the stream is decoded.
If a ZNG stream includes data that is not valid UTF-8, the stream is invalid.

A stream of control messages and values messages is represented
as a sequence of lines each terminated by a newline.
Any newlines embedded in string-typed values must be escaped,
i.e., via `\u{0a}` or `\x0a`.

A line that begins with `#` is a control message and all other lines
are values.

### 3.1 ZNG Control Messages

ZNG control messages have one of four forms defined below.

Any line beginning with `#` that does not conform with the syntax described here
is an error.
When errors are encountered parsing ZNG, an implementation should return a
corresponding error and allow ZNG parsing to proceed if desired.

### 3.1.1 Type Binding

A type binding has the following form:
```
#<type-tag>:<type-string>
```
Here, `<type-tag>` is a string decimal integer and `<type-string>`
is a string defining a type according to the ZNG type syntax creates a binding
between the indicated tag and the indicated type.

### 3.1.2 Type Alias

A type alias has the following form:
```
#<type-name>:<type-string>
```
Here, `<type-name>` is an identifier and `<type-string>`
is a string defining a type according to the ZNG type syntax creates a binding
between the indicated tag and the indicated type.
This form defines an alias mapping the identifier to the indicated type.
`<type-name>` is an identifier with semantics as defined in Section 2.1.1.4.

It is an error to define an alias that has the same name as a primitive type.
It is also an error to redefine a previously defined alias with a
type that differs from the original definition.

### 3.1.3 Application-specific Payload

An application-specific payload has the following form:
```
#!<control code>:<payload>
```
Here, `<control code>` is a decimal integer in the range 6-127 and `<payload>`
is any UTF-8 string with escaped newlines.

### 3.1.4 Ordering Hint
An ordering hint has the form:
```
#[+-]<field>,[+-]<field>,...
```
where the string present after the colon has the same semantics as
those described in Section 2.1.2.

### Type Grammar

Given the above textual definitions and the underlying BZNG specification, a
grammar describing the textual type encodings is:
```
<stype> := bool | byte | int16 | uint16 | int32 | uint32 | int64 | uint64 | float64
         | string | bytes | bstring | enum | ip | net | time | duration | null
         | <alias-name>

<ctype> :=  array [ <stype> ]
          | record [ <columns> ]
          | record [ ]
          | union [ <stype-list> ]
          | set [ <stype> ]


<stype-list> :=    <stype>
                 | <stype-list> , <stype>

<columns> :=      <column>
                | <columns> , <column>

<column> := <id> : <stype>

<alias-name> := <id>

<id> := <id_start> <id_continue>*

<id_start> := [A-Za-z_$]

<id_continue> := <id_start> | [0-9]
```

A reference implementation of this type system is embedded in
[zq/zng](../).

### 3.2 ZNG Values

A ZNG value is encoded on a line as typed value, which is encoded as
an integer type code followed by `:`, which is in turn followed
by a value encoding.

Here is a pseudo-grammar for typed values:
```
<typed-value> := <tag> : <elem>
<tag> :=  0
        | [1-9][0-9]*
<elem> :=
          <terminal>
          <tag> : <terminal>
        | [ <list-elem>* ]
<list-elem> := <elem> ;
<terminal> := <char>*
```

A terminal value is encoded as a string of characters terminated
by a semicolon (which must be escaped if it appears in a string-typed value).
If the terminal value is of a union type, it is prefixed with the index of the value type in reference to the union type and a colon.

Container values (i.e., sets, arrays, or records) are encoded as
* an open bracket,
* zero or more encoded values terminated with semicolon, and
* a close bracket.

Any value can be specified as "unset" with the ASCII character `-`.
This is typically used to represent columns of records where not all
columns have been set in a given record value, though any type can be
validly unset.  A value that is not to be interpreted as "unset"
but is the single-character string `-`, must be escaped (e.g., `\x2d`).

Note that this syntax can be scanned and parsed independent of the
actual type definition indicated by the descriptor.  It is a semantic error
if the parsed value does not match the indicated type in terms of number and
sub-structure of value elements present and their interpretation as a valid
string of the specified type.

It is an error for a value to reference a type ID that has not been previously
defined by a typedef scoped to the stream in which the value appears.

#### 3.2.1 Character Escape Rules

Any Unicode code point may be represented in a `string` value using
the same `\u` syntax as Javascript.  Specifically:
* The sequence `\uhhhh` where each `h` is a hexadecimal digit represents
  the Unicode code point corresponding to the given
  4-digit (hexadecimal) number, or:
* `\u{h*}` where there are from 1 to 6 hexadecimal digits inside the
  brackets represents the Unicode code point corresponding to the given
  hexadecimal number.

`\u` followed by anything that does not conform to the above syntax
is not a valid escape sequence.
The behavior of an implementation that encounters such
invalid sequences in a `string` type is undefined.

Any character in a `bstring` value may be escaped from the ZNG formatting rules
using the hex escape syntax, i.e., `\xhh` where `h` is a hexadecimal digit.
This allows binary data that does not conform to a valid UTF-8 character encoding
to be embedded in the `bstring` data type.
`\x` followed by anything other than two hexadecimal digits is not a valid
escape sequence. The behavior of an implementation that encounters such
invalid sequences in a `bstring` type is undefined.
Additionally, the backslash character itself (U+3B) may be represented
by a sequence of two consecutive backslash characters.  In other words,
the bstrings `\\` and `\x3b` are equivalent and both represent a single
backslash character.

These special characters must be escaped if they appear within a
`string` or `bstring` type: `;`, `\`, newline (Unicode U+0A).
These characters are invalid in all other data types.

Likewise, these characters must be escaped if they appear as the first character
of a value:
```
[ ]
```
In addition, `-` must be escaped if representing the single ASCII byte equal
to `-` as opposed to representing an unset value.

#### 3.2.2 Value Syntax

Each UTF-8 string field parsed from a value line is interpreted according to the
type descriptor of the line.
The formats for each type is as follows:

Type | Format
---- | ------
`bool` | a single character `T` or `F`
`byte` | two-characters of hexadecimal digit
`int16` | decimal string representation of any signed, 16-bit integer
`uint16` | decimal string representation of any unsigned, 16-bit integer
`int32` | decimal string representation of any signed, 32-bit integer
`uint32` | decimal string representation of any unsigned, 32-bit integer
`int64` | decimal string representation of any signed, 64-bit integer
`uint64` | decimal string representation of any unsigned, 64-bit integer
`float64` | a decimal representation of a 64-bit IEEE floating point literal as defined in JavaScript
`string` | a UTF-8 string
`bytes` | a sequence of bytes encoded as base64
`bstring` | a UTF-8 string with `\x` escapes of non-UTF binary data
`enum` | a string representing an enumeration value defined outside the scope of ZNG
`ip` | a string representing an IP address in [IPv4 or IPv6 format](https://tools.ietf.org/html/draft-main-ipaddr-text-rep-02#section-3)
`net` | a string in CIDR notation representing an IP address and prefix length as defined in RFC 4632 and RFC 4291.
`time` | signed dotted decimal notation of seconds
`duration` | signed dotted decimal notation of seconds
`null` | must be the literal value -

## 4. Examples

Here are some simple examples to get the gist of the ZNG text format.

Primitive types look like this and do not need typedefs:
```
bool
string
int
```
Container types look like this and do need typedefs:
```
#0:array[int]
#1:set[bool,string]
#2:record[x:float64,y:float64]
```
Container types can be embedded in other container types by referencing
an earlier-defined type alias:
```
#REC:record[a:string,b:string,c:int]
#SET:set[string]
#99:record[v:REC,s:SET,r:REC,s2:SET]
```
This ZNG defines a tag for the primitive string type and defines a record
and references the types accordingly in three values;
```
#0:string
#1:record[a:string,b:string]
0:hello, world;
1:[hello;world;]
0:this is a semicolon: \x3b;
```
which represents a stream of the following three values:
```
string("hello, world")
record(a:"hello",b:"world")
string("this is a semicolon: ;")
```
Note that the tag integers occupy their own numeric space indepedent of
any underlying BZNG type IDs.

The semicolon terminator is important.  Consider this ZNG depicting
sets of strings:
```
#0:set[string]
0:[hello,world;]
0:[hello;world;]
0:[]
0:[;]
```
In this example:
* the first value is a `set` of one `string`
* the second value is a `set` of two `string` values, `hello` and `world`,
* the third value is an empty `set`, and
* the fourth value is a `set` containing one `string` of zero length.

In this way, an empty `set` and a `set` containing only a zero-length `string` can be distinguished.

This scheme allows containers to be embedded in containers, e.g., a
`record` inside of a `record` like this:
```
#LL:record[compass:string,degree:float64]
#26:record[city:string,lat:LL,long:LL]
26:[NYC;[N;40.7128;][W;74.0060;]]
```
An unset value indicates a field of a `record` that wasn't set by the encoder:
```
26:[North Pole;[N;90;]-;]
```
e.g., the North Pole has a latitude but no meaningful longitude.

## 5. Related Links

* [Zeek ASCII logging](https://docs.zeek.org/en/stable/examples/logs/)
* [Binary logging in Zeek](https://www.zeek.org/development/projects/binary-logging.html)
* [Hadoop sequence file](https://cwiki.apache.org/confluence/display/HADOOP2/SequenceFile)
* [Avro](https://avro.apache.org)
* [Parquet](https://en.wikipedia.org/wiki/Apache_Parquet)
* [Protocol Buffers](https://developers.google.com/protocol-buffers)
* [MessagePack](https://msgpack.org/index.html)
* [gNMI](https://github.com/openconfig/reference/tree/master/rpc/gnmi)
