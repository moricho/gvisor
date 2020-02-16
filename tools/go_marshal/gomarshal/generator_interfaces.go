// Copyright 2019 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gomarshal

import (
	"fmt"
	"go/ast"
	"go/token"
)

// interfaceGenerator generates marshalling interfaces for a single type.
//
// getState is not thread-safe.
type interfaceGenerator struct {
	sourceBuffer

	// The type we're serializing.
	t *ast.TypeSpec

	// Receiver argument for generated methods.
	r string

	// FileSet containing the tokens for the type we're processing.
	f *token.FileSet

	// is records external packages referenced by the generated implementation.
	is map[string]struct{}

	// ms records Marshallable types referenced by the generated implementation
	// of t's interfaces.
	ms map[string]struct{}

	// as records embedded fields in t that are potentially not packed. The key
	// is the accessor for the field.
	as map[string]struct{}
}

// typeName returns the name of the type this g represents.
func (g *interfaceGenerator) typeName() string {
	return g.t.Name.Name
}

// newinterfaceGenerator creates a new interface generator.
func newInterfaceGenerator(t *ast.TypeSpec, fset *token.FileSet) *interfaceGenerator {
	g := &interfaceGenerator{
		t:  t,
		r:  receiverName(t),
		f:  fset,
		is: make(map[string]struct{}),
		ms: make(map[string]struct{}),
		as: make(map[string]struct{}),
	}
	g.recordUsedMarshallable(g.typeName())
	return g
}

func (g *interfaceGenerator) recordUsedMarshallable(m string) {
	g.ms[m] = struct{}{}

}

func (g *interfaceGenerator) recordUsedImport(i string) {
	g.is[i] = struct{}{}

}

func (g *interfaceGenerator) recordPotentiallyNonPackedField(fieldName string) {
	g.as[fieldName] = struct{}{}
}

func (g *interfaceGenerator) fieldAccessor(n *ast.Ident) string {
	return fmt.Sprintf("%s.%s", g.r, n.Name)
}

// abortAt aborts the go_marshal tool with the given error message, with a
// reference position to the input source. Same as abortAt, but uses g to
// resolve p to position.
func (g *interfaceGenerator) abortAt(p token.Pos, msg string) {
	abortAt(g.f.Position(p), msg)
}

// scalarSize returns the size of type identified by t. If t isn't a primitive
// type, the size isn't known at code generation time, and must be resolved via
// the marshal.Marshallable interface.
func (g *interfaceGenerator) scalarSize(t *ast.Ident) (size int, unknownSize bool) {
	switch t.Name {
	case "int8", "uint8", "byte":
		return 1, false
	case "int16", "uint16":
		return 2, false
	case "int32", "uint32":
		return 4, false
	case "int64", "uint64":
		return 8, false
	default:
		return 0, true
	}
}

func (g *interfaceGenerator) shift(bufVar string, n int) {
	g.emit("%s = %s[%d:]\n", bufVar, bufVar, n)
}

func (g *interfaceGenerator) shiftDynamic(bufVar, name string) {
	g.emit("%s = %s[%s.SizeBytes():]\n", bufVar, bufVar, name)
}

// marshalScalar writes a single scalar to a byte slice.
func (g *interfaceGenerator) marshalScalar(accessor, typ, bufVar string) {
	switch typ {
	case "int8", "uint8", "byte":
		g.emit("%s[0] = byte(%s)\n", bufVar, accessor)
		g.shift(bufVar, 1)
	case "int16", "uint16":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint16(%s[:2], uint16(%s))\n", bufVar, accessor)
		g.shift(bufVar, 2)
	case "int32", "uint32":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint32(%s[:4], uint32(%s))\n", bufVar, accessor)
		g.shift(bufVar, 4)
	case "int64", "uint64":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint64(%s[:8], uint64(%s))\n", bufVar, accessor)
		g.shift(bufVar, 8)
	default:
		g.emit("%s.MarshalBytes(%s[:%s.SizeBytes()])\n", accessor, bufVar, accessor)
		g.shiftDynamic(bufVar, accessor)
	}
}

// unmarshalScalar reads a single scalar from a byte slice.
func (g *interfaceGenerator) unmarshalScalar(accessor, typ, bufVar string) {
	switch typ {
	case "int8":
		g.emit("%s = int8(%s[0])\n", accessor, bufVar)
		g.shift(bufVar, 1)
	case "uint8":
		g.emit("%s = uint8(%s[0])\n", accessor, bufVar)
		g.shift(bufVar, 1)
	case "byte":
		g.emit("%s = %s[0]\n", accessor, bufVar)
		g.shift(bufVar, 1)

	case "int16":
		g.recordUsedImport("usermem")
		g.emit("%s = int16(usermem.ByteOrder.Uint16(%s[:2]))\n", accessor, bufVar)
		g.shift(bufVar, 2)
	case "uint16":
		g.recordUsedImport("usermem")
		g.emit("%s = usermem.ByteOrder.Uint16(%s[:2])\n", accessor, bufVar)
		g.shift(bufVar, 2)

	case "int32":
		g.recordUsedImport("usermem")
		g.emit("%s = int32(usermem.ByteOrder.Uint32(%s[:4]))\n", accessor, bufVar)
		g.shift(bufVar, 4)
	case "uint32":
		g.recordUsedImport("usermem")
		g.emit("%s = usermem.ByteOrder.Uint32(%s[:4])\n", accessor, bufVar)
		g.shift(bufVar, 4)

	case "int64":
		g.recordUsedImport("usermem")
		g.emit("%s = int64(usermem.ByteOrder.Uint64(%s[:8]))\n", accessor, bufVar)
		g.shift(bufVar, 8)
	case "uint64":
		g.recordUsedImport("usermem")
		g.emit("%s = usermem.ByteOrder.Uint64(%s[:8])\n", accessor, bufVar)
		g.shift(bufVar, 8)
	default:
		g.emit("%s.UnmarshalBytes(%s[:%s.SizeBytes()])\n", accessor, bufVar, accessor)
		g.shiftDynamic(bufVar, accessor)
		g.recordPotentiallyNonPackedField(accessor)
	}
}
