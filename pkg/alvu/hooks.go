package alvu

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/barelyhuman/alvu/transformers"
	lua "github.com/yuin/gopher-lua"

	luaAlvu "github.com/barelyhuman/alvu/lua/alvu"
	ghttp "github.com/cjoudrey/gluahttp"
	stringsLib "github.com/vadv/gopher-lua-libs/strings"
	yamlLib "github.com/vadv/gopher-lua-libs/yaml"
	luajson "layeh.com/gopher-json"
)

type HookSource struct {
	luaState      *lua.LState
	filename      string
	ForSingleFile bool
	ForFile       string
}

type Hooks struct {
	ac               AlvuConfig
	collection       []*HookSource
	forSpecificFiles map[string][]*HookSource
}

type HookedFile struct {
	transformers.TransformedFile
	content []byte
	data    map[string]interface{}
	extras  map[string]interface{}
}

func (h *Hooks) Load() {
	hookFiles := []string{}
	folderInfo, err := os.Stat(h.ac.HookDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		readHookDirError(err, h.ac.HookDir, h.ac.logger)
	}

	file, err := os.Open(folderInfo.Name())
	readHookDirError(err, h.ac.HookDir, h.ac.logger)
	childs, err := file.Readdirnames(1)
	readHookDirError(err, h.ac.HookDir, h.ac.logger)
	if len(childs) == 0 {
		return
	}

	filepath.WalkDir(h.ac.HookDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			h.ac.logger.Error(fmt.Sprintf("Issue reading %v, with error: %v", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".lua" {
			return nil
		}
		hookFiles = append(hookFiles, filepath.Join(h.ac.RootPath, path))
		return nil
	})

	h.forSpecificFiles = map[string][]*HookSource{}
	for _, filename := range hookFiles {
		hookSource := h.readHookFile(filename, h.ac.RootPath, h.ac.logger)
		if hookSource.ForSingleFile {
			if h.forSpecificFiles[hookSource.ForFile] == nil {
				h.forSpecificFiles[hookSource.ForFile] = []*HookSource{}
			}
			h.forSpecificFiles[hookSource.ForFile] = append(h.forSpecificFiles[hookSource.ForFile], hookSource)
		} else {
			h.collection = append(h.collection, hookSource)
		}
	}
}

func (h *Hooks) runLifeCycleHooks(hookType string) error {
	keys := make([]string, 0, len(h.forSpecificFiles))
	for k := range h.forSpecificFiles {
		keys = append(keys, k)
	}

	localCollection := []*HookSource{}
	localCollection = append(localCollection, h.collection...)

	for _, v := range keys {
		localCollection = append(localCollection, h.forSpecificFiles[v]...)
	}

	sort.Slice(localCollection, func(i, j int) bool {
		return strings.Compare(localCollection[i].filename, localCollection[j].filename) == -1
	})

	for i := range localCollection {
		s := localCollection[i]
		hookFunc := s.luaState.GetGlobal(hookType)

		if hookFunc == lua.LNil {
			continue
		}

		if err := s.luaState.CallByParam(lua.P{
			Fn:      hookFunc,
			NRet:    0,
			Protect: true,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) readHookFile(filename string, basepath string, logger Logger) *HookSource {
	lState := lua.NewState()
	luaAlvu.Preload(lState)
	luajson.Preload(lState)
	yamlLib.Preload(lState)
	stringsLib.Preload(lState)
	lState.PreloadModule("http", ghttp.NewHttpModule(&http.Client{}).Loader)
	if err := lState.DoFile(filename); err != nil {
		logger.Error(fmt.Sprintf("Failed to execute hook: %v, with error: %v\n", filename, err))
		panic("")
	}
	if basepath == "." {
		lState.SetGlobal("workingdir", lua.LString(""))
	} else {
		lState.SetGlobal("workingdir", lua.LString(basepath))
	}
	forFile := lState.GetGlobal("ForFile")
	forFileValue := forFile.String()
	return &HookSource{
		filename:      filename,
		luaState:      lState,
		ForSingleFile: forFileValue != "nil",
		ForFile:       forFileValue,
	}
}

func (h *Hooks) ProcessFile(file transformers.TransformedFile) (hookedFile HookedFile) {
	hookedFile.TransformedFile = file
	fileData, _ := os.ReadFile(file.TransformedFile)

	hookInput := struct {
		Name       string `json:"name"`
		SourcePath string `json:"source_path"`
		// DestPath         string                 `json:"dest_path"`
		// Meta             map[string]interface{} `json:"meta"`
		WriteableContent string `json:"content"`
		// HTMLContent      string                 `json:"html"`
	}{
		Name:             strings.TrimPrefix(file.SourcePath, filepath.Join(h.ac.RootPath, "pages")),
		SourcePath:       file.SourcePath,
		WriteableContent: string(fileData),
	}

	hookJsonInput, _ := json.Marshal(hookInput)

	localCollection := []*HookSource{}

	filePathSplits := strings.Split(file.SourcePath, string(filepath.Separator))
	nonRootPath := filepath.Join(filePathSplits[1:]...)

	if len(h.forSpecificFiles[nonRootPath]) > 0 {
		localCollection = append(localCollection, h.forSpecificFiles[nonRootPath]...)
	}
	localCollection = append(localCollection, h.collection...)

	sort.Slice(localCollection, func(i, j int) bool {
		return strings.Compare(localCollection[i].filename, localCollection[j].filename) == -1
	})

	for i := range localCollection {
		hook := localCollection[i]
		hookFunc := hook.luaState.GetGlobal("Writer")

		if hookFunc == lua.LNil {
			continue
		}

		if err := hook.luaState.CallByParam(lua.P{
			Fn:      hookFunc,
			NRet:    1,
			Protect: true,
		}, lua.LString(hookJsonInput)); err != nil {
			h.ac.logger.Error(fmt.Sprintf("Failed to execute  %v's Writer on %v, with err: %v", hook.filename, file.SourcePath, err))
			panic("")
		}

		ret := hook.luaState.Get(-1)

		var fromPlug map[string]interface{}

		err := json.Unmarshal([]byte(ret.String()), &fromPlug)
		if err != nil {
			h.ac.logger.Error(fmt.Sprintf("Invalid return value in hook %v", hook.filename))
			return
		}

		if fromPlug["content"] != nil {
			stringVal := fmt.Sprintf("%s", fromPlug["content"])
			hookedFile.content = []byte(stringVal)
		}

		// if fromPlug["name"] != nil {
		// 	hookedFile.content = []byte(fmt.Sprintf("%v", fromPlug["name"]))
		// }

		if fromPlug["data"] != nil {
			hookedFile.data = mergeMapWithCheck(hookedFile.data, fromPlug["data"])
		}

		if fromPlug["extras"] != nil {
			hookedFile.extras = mergeMapWithCheck(hookedFile.extras, fromPlug["data"])
		}

		hook.luaState.Pop(1)
	}
	return
}

func readHookDirError(err error, directory string, logger Logger) {
	if err == nil {
		return
	}
	logger.Error(
		fmt.Sprintf("Failed to read the hooks dir: %v, with error: %v\n", directory, err),
	)
}

func mergeMapWithCheck(maps ...any) (source map[string]interface{}) {
	source = map[string]interface{}{}
	for _, toCheck := range maps {
		if pairs, ok := toCheck.(map[string]interface{}); ok {
			for k, v := range pairs {
				source[k] = v
			}
		}
	}
	return source
}
