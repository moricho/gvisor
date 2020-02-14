// Copyright 2020 The gVisor Authors.
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

// This file contains the bits of the code generator specific to marshalling
// newtypes on primitives.

package gomarshal

import (
	"fmt"
	"go/ast"
)

// marshalPrimitiveScalar writes a single primitive variable to a byte
// slice.
func (g *interfaceGenerator) marshalPrimitiveScalar(accessor, typ, bufVar string) {
	switch typ {
	case "int8", "uint8", "byte":
		g.emit("%s[0] = byte(*%s)\n", bufVar, accessor)
	case "int16", "uint16":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint16(%s[:2], uint16(*%s))\n", bufVar, accessor)
	case "int32", "uint32":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint32(%s[:4], uint32(*%s))\n", bufVar, accessor)
	case "int64", "uint64":
		g.recordUsedImport("usermem")
		g.emit("usermem.ByteOrder.PutUint64(%s[:8], uint64(*%s))\n", bufVar, accessor)
	default:
		g.emit("// Explicilty cast to the underlying type before dispatching to\n")
		g.emit("// MarshalBytes, so we don't recursively call %s.MarshalBytes\n", accessor)
		g.emit("inner := (*%s)(%s)\n", typ, accessor)
		g.emit("inner.MarshalBytes(%s[:%s.SizeBytes()])\n", bufVar, accessor)
	}
}

// unmarshalPrimitiveScalar read a single primitive variable from a byte slice.
func (g *interfaceGenerator) unmarshalPrimitiveScalar(accessor, typ, bufVar, typeCast string) {
	switch typ {
	case "int8":
		g.emit("*%s = %s(int8(%s[0]))\n", accessor, typeCast, bufVar)
	case "uint8":
		g.emit("*%s = %s(uint8(%s[0]))\n", accessor, typeCast, bufVar)
	case "byte":
		g.emit("*%s = %s(%s[0])\n", accessor, typeCast, bufVar)

	case "int16":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(int16(usermem.ByteOrder.Uint16(%s[:2])))\n", accessor, typeCast, bufVar)
	case "uint16":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(usermem.ByteOrder.Uint16(%s[:2]))\n", accessor, typeCast, bufVar)

	case "int32":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(int32(usermem.ByteOrder.Uint32(%s[:4])))\n", accessor, typeCast, bufVar)
	case "uint32":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(usermem.ByteOrder.Uint32(%s[:4]))\n", accessor, typeCast, bufVar)

	case "int64":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(int64(usermem.ByteOrder.Uint64(%s[:8])))\n", accessor, typeCast, bufVar)
	case "uint64":
		g.recordUsedImport("usermem")
		g.emit("*%s = %s(usermem.ByteOrder.Uint64(%s[:8]))\n", accessor, typeCast, bufVar)
	default:
		g.emit("// Explicilty cast to the underlying type before dispatching to\n")
		g.emit("// UnmarshalBytes, so we don't recursively call %s.UnmarshalBytes\n", accessor)
		g.emit("inner := (*%s)(%s)\n", typ, accessor)
		g.emit("inner.UnmarshalBytes(%s[:%s.SizeBytes()])\n", bufVar, accessor)
	}
}

func (g *interfaceGenerator) validatePrimitiveNewtype(t *ast.Ident) {
	switch t.Name {
	case "int8", "uint8", "byte", "int16", "uint16", "int32", "uint32", "int64", "uint64":
		// These are the only primitive types we're allow. Below, we provide
		// suggestions for some disallowed types and reject them, then attempt
		// to marshal any remaining types by invoking the marshal.Marshallable
		// interface on them. If these types don't actually implement
		// marshal.Marshallable, compilation of the generated code will fail
		// with an appropriate error message.
		return
	case "int":
		g.abortAt(t.Pos(), "Type 'int' has ambiguous width, use int32 or int64")
	case "uint":
		g.abortAt(t.Pos(), "Type 'uint' has ambiguous width, use uint32 or uint64")
	case "string":
		g.abortAt(t.Pos(), "Type 'string' is dynamically-sized and cannot be marshalled, use a fixed size byte array '[...]byte' instead")
	default:
		debugfAt(g.f.Position(t.Pos()), fmt.Sprintf("Found derived type '%s', will attempt dispatch via marshal.Marshallable.\n", t.Name))
	}
}

// emitMarshallableForPrimitiveNewtype outputs code to implement the
// marshal.Marshallable interface for a newtype on a primitive. Primitive
// newtypes are always packed, so we can omit the various fallbacks required for
// non-packed structs.
func (g *interfaceGenerator) emitMarshallableForPrimitiveNewtype(nt *ast.Ident) {
	g.emit("// SizeBytes implements marshal.Marshallable.SizeBytes.\n")
	g.emit("func (%s *%s) SizeBytes() int {\n", g.r, g.typeName())
	g.inIndent(func() {
		if size, dynamic := g.scalarSize(nt); !dynamic {
			g.emit("return %d\n", size)
		} else {
			g.emit("return (*%s)(nil).SizeBytes()\n", nt.Name)
		}
	})
	g.emit("}\n\n")

	g.emit("// MarshalBytes implements marshal.Marshallable.MarshalBytes.\n")
	g.emit("func (%s *%s) MarshalBytes(dst []byte) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.marshalPrimitiveScalar(g.r, nt.Name, "dst")
	})
	g.emit("}\n\n")

	g.emit("// UnmarshalBytes implements marshal.Marshallable.UnmarshalBytes.\n")
	g.emit("func (%s *%s) UnmarshalBytes(src []byte) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.unmarshalPrimitiveScalar(g.r, nt.Name, "src", g.typeName())
	})
	g.emit("}\n\n")

	g.emit("// Packed implements marshal.Marshallable.Packed.\n")
	g.emit("func (%s *%s) Packed() bool {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.emit("// Scalar newtypes are always packed.\n")
		g.emit("return true\n")
	})
	g.emit("}\n\n")

	g.emit("// MarshalUnsafe implements marshal.Marshallable.MarshalUnsafe.\n")
	g.recordUsedImport("safecopy")
	g.recordUsedImport("unsafe")
	g.emit("func (%s *%s) MarshalUnsafe(dst []byte) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.emit("safecopy.CopyIn(dst, unsafe.Pointer(%s))\n", g.r)
	})
	g.emit("}\n\n")

	g.emit("// UnmarshalUnsafe implements marshal.Marshallable.UnmarshalUnsafe.\n")
	g.recordUsedImport("safecopy")
	g.recordUsedImport("unsafe")
	g.emit("func (%s *%s) UnmarshalUnsafe(src []byte) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.emit("safecopy.CopyOut(unsafe.Pointer(%s), src)\n", g.r)
	})
	g.emit("}\n\n")

	g.emit("// CopyOut implements marshal.Marshallable.CopyOut.\n")
	g.recordUsedImport("marshal")
	g.recordUsedImport("usermem")
	g.emit("func (%s *%s) CopyOut(task marshal.Task, addr usermem.Addr) (int, error) {\n", g.r, g.typeName())
	g.inIndent(func() {
		// Fast serialization.
		g.emit("// Bypass escape analysis on %s. The no-op arithmetic operation on the\n", g.r)
		g.emit("// pointer makes the compiler think val doesn't depend on %s.\n", g.r)
		g.emit("// See src/runtime/stubs.go:noescape() in the golang toolchain.\n")
		g.emit("ptr := unsafe.Pointer(%s)\n", g.r)
		g.emit("val := uintptr(ptr)\n")
		g.emit("val = val^0\n\n")

		g.emit("// Construct a slice backed by %s's underlying memory.\n", g.r)
		g.emit("var buf []byte\n")
		g.emit("hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))\n")
		g.emit("hdr.Data = val\n")
		g.emit("hdr.Len = %s.SizeBytes()\n", g.r)
		g.emit("hdr.Cap = %s.SizeBytes()\n\n", g.r)

		g.emit("len, err := task.CopyOutBytes(addr, buf)\n")
		g.emit("// Since we bypassed the compiler's escape analysis, indicate that %s\n", g.r)
		g.emit("// must live until after the CopyOutBytes.\n")
		g.emit("runtime.KeepAlive(%s)\n", g.r)
		g.emit("return len, err\n")
	})
	g.emit("}\n\n")

	g.emit("// CopyIn implements marshal.Marshallable.CopyIn.\n")
	g.recordUsedImport("marshal")
	g.recordUsedImport("usermem")
	g.emit("func (%s *%s) CopyIn(task marshal.Task, addr usermem.Addr) (int, error) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.emit("// Bypass escape analysis on %s. The no-op arithmetic operation on the\n", g.r)
		g.emit("// pointer makes the compiler think val doesn't depend on %s.\n", g.r)
		g.emit("// See src/runtime/stubs.go:noescape() in the golang toolchain.\n")
		g.emit("ptr := unsafe.Pointer(%s)\n", g.r)
		g.emit("val := uintptr(ptr)\n")
		g.emit("val = val^0\n\n")

		g.emit("// Construct a slice backed by %s's underlying memory.\n", g.r)
		g.emit("var buf []byte\n")
		g.emit("hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))\n")
		g.emit("hdr.Data = val\n")
		g.emit("hdr.Len = %s.SizeBytes()\n", g.r)
		g.emit("hdr.Cap = %s.SizeBytes()\n\n", g.r)

		g.emit("len, err := task.CopyInBytes(addr, buf)\n")
		g.emit("// Since we bypassed the compiler's escape analysis, indicate that %s\n", g.r)
		g.emit("// must live until after the CopyInBytes.\n")
		g.emit("runtime.KeepAlive(%s)\n", g.r)
		g.emit("return len, err\n")
	})
	g.emit("}\n\n")

	g.emit("// WriteTo implements io.WriterTo.WriteTo.\n")
	g.recordUsedImport("io")
	g.emit("func (%s *%s) WriteTo(w io.Writer) (int64, error) {\n", g.r, g.typeName())
	g.inIndent(func() {
		g.emit("// Bypass escape analysis on %s. The no-op arithmetic operation on the\n", g.r)
		g.emit("// pointer makes the compiler think val doesn't depend on %s.\n", g.r)
		g.emit("// See src/runtime/stubs.go:noescape() in the golang toolchain.\n")
		g.emit("ptr := unsafe.Pointer(%s)\n", g.r)
		g.emit("val := uintptr(ptr)\n")
		g.emit("val = val^0\n\n")

		g.emit("// Construct a slice backed by %s's underlying memory.\n", g.r)
		g.emit("var buf []byte\n")
		g.emit("hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))\n")
		g.emit("hdr.Data = val\n")
		g.emit("hdr.Len = %s.SizeBytes()\n", g.r)
		g.emit("hdr.Cap = %s.SizeBytes()\n\n", g.r)

		g.emit("len, err := w.Write(buf)\n")
		g.emit("// Since we bypassed the compiler's escape analysis, indicate that %s\n", g.r)
		g.emit("// must live until after the Write.\n")
		g.emit("runtime.KeepAlive(%s)\n", g.r)
		g.emit("return int64(len), err\n")

	})
	g.emit("}\n\n")
}
