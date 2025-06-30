package alvu

import (
	"os"
	"path"

	dotenv "github.com/joho/godotenv"
	lua "github.com/yuin/gopher-lua"
)

var api = map[string]lua.LGFunction{
	"files":   GetFilesIndex,
	"get_env": GetEnv,
}

// Preload adds json to the given Lua state's package.preload table. After it
// has been preloaded, it can be loaded using require:
//
//	local json = require("json")
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
		fullPath := path.Join(pathToIndex, f.Name())
		info, lerr := os.Lstat(fullPath)
		if lerr != nil {
			return paths, lerr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if f.IsDir() {
			nestPath := fullPath
			nestPaths, err := getFilesIndex(nestPath)
			if err != nil {
				return paths, err
			}
			withPathPrefix := []string{}
			for _, _path := range nestPaths {
				withPathPrefix = append(withPathPrefix, path.Join(f.Name(), _path))
			}
			paths = append(paths, withPathPrefix...)
		} else {
			paths = append(paths, f.Name())
		}
	}

	return
}

func GetEnv(L *lua.LState) int {
	// path to get the env from
	str := L.CheckString(1)
	// key to get from index
	key := L.CheckString(2)
	value := LGetEnv(L, str, key)
	L.Push(value)
	return 1
}

func LGetEnv(L *lua.LState, fromFile string, str string) lua.LString {
	dotenv.Load(fromFile)
	val := os.Getenv(str)
	return lua.LString(val)
}
