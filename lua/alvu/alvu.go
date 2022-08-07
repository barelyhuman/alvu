package alvu

import (
	"os"
	"path"

	lua "github.com/yuin/gopher-lua"
)

var api = map[string]lua.LGFunction{
	"files": GetFilesIndex,
}

// Preload adds json to the given Lua state's package.preload table. After it
// has been preloaded, it can be loaded using require:
//
//  local json = require("json")
func Preload(L *lua.LState) {
	L.PreloadModule("alvu", Loader)
}

// Loader is the module loader function.
func Loader(L *lua.LState) int {
	t := L.NewTable()
	L.SetFuncs(t, api)
	L.Push(t)
	return 1
}

// Decode lua json.decode(string) returns (table, err)
func GetFilesIndex(L *lua.LState) int {
	// path to get the index for
	str := L.CheckString(1)

	value, err := LGetFilesIndex(L, str)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(value)
	return 1
}

func LGetFilesIndex(L *lua.LState, pathToIndex string) (*lua.LTable, error) {
	indexedPaths, err := getFilesIndex(pathToIndex)
	if err != nil {
		return nil, err
	}
	arr := L.CreateTable(len(indexedPaths), 0)
	for _, item := range indexedPaths {
		arr.Append(lua.LString(item))
	}

	return arr, nil
}

func getFilesIndex(pathToIndex string) (paths []string, err error) {
	files, err := os.ReadDir(pathToIndex)
	if err != nil {
		return
	}

	for _, f := range files {
		if f.IsDir() {
			nestPath := path.Join(pathToIndex, f.Name())
			nestPaths, err := getFilesIndex(nestPath)
			if err != nil {
				return paths, err
			}
			paths = append(paths, nestPaths...)
		} else {
			paths = append(paths, f.Name())
		}
	}

	return
}
