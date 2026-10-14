package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerUtils(L *lua.LState) {
	L.SetGlobal("print", L.NewFunction(e.print))
	L.SetGlobal("hash", L.NewFunction(luaHash))
	L.SetGlobal("ipairs", ipairs(L, L.GetGlobal("ipairs")))
}

func ipairs(L *lua.LState, stock lua.LValue) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		v := L.CheckAny(1)
		if v.Type() != lua.LTUserData {
			L.Push(stock)
			L.Push(v)
			L.Call(1, 3)
			return 3
		}
		n := L.ObjLen(v)
		L.Push(L.NewFunction(func(L *lua.LState) int {
			i := L.CheckInt(2) + 1
			if i > n {
				return 0
			}
			L.Push(lua.LNumber(i))
			L.Push(L.GetTable(v, lua.LNumber(i)))
			return 2
		}))
		L.Push(v)
		L.Push(lua.LNumber(0))
		return 3
	})
}

func (e *Engine) print(L *lua.LState) int {
	parts := make([]string, L.GetTop())
	for i := range parts {
		parts[i] = L.ToStringMeta(L.Get(i + 1)).String()
	}
	e.log.Info(strings.Join(parts, " "), "at", L.Where(1))
	return 0
}

func luaHash(L *lua.LState) int {
	sum := sha256.Sum256([]byte(L.CheckString(1)))
	L.Push(lua.LString(hex.EncodeToString(sum[:8])))
	return 1
}
