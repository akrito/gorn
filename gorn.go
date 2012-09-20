package main

import (
	"github.com/ugorji/go-msgpack"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Cache struct {
	Paths   map[string]Path
	History []string
}

type Path struct {
	Dir   string
	Execs []string
	Mtime int64
}

func main() {
	// Where's the cache?
	home := os.Getenv("HOME")
	cacheName := home + "/.cache/gorn.msgpack"
	// Read the cache
	in, _ := os.Open(cacheName)
	dec := msgpack.NewDecoder(in, nil)
	var cache Cache
	dec.Decode(&cache)

	// Check timestamps of everything on $PATH. If the timestamp is newer,
	// regenerate that path
	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, ":")
	for _, path := range paths {
		// TODO: compensate for missing paths
		fi, _ := os.Stat(path)
		mtime := fi.ModTime().Unix()
		if cache.Paths[path].Mtime != mtime {
			// Regenerate path
			if len(cache.Paths) == 0 {
				cache.Paths = make(map[string]Path, 64)
			}
			cache.Paths[path] = regenerate(path)
		}
	}

	candidates := make(map[string]string)
	// Populate history map
	historyMap := make(map[string]int)
	for i, exec := range cache.History {
		historyMap[exec] = i
	}
	// For executables in the paths dictionary
	for _, path := range cache.Paths {
		for _, exec := range path.Execs {
			// if it's not in previous input
			if _, ok := historyMap[exec]; !ok {
				// add it to candidates
				candidates[exec] = exec
			}
		}
	}

	var input []string
	// print previous input in order ...
	for _, exec := range cache.History {
		input = append(input, exec)
	}
	// print candidates in any order
	for exec := range candidates {
		input = append(input, exec)
	}
	inputJoined := strings.Join(input, "\n")
	reader := strings.NewReader(inputJoined)

	// get dmenu output
	dmenu := exec.Command("dmenu", os.Args[1:]...)
	dmenu.Stdin = reader
	dmenuBytes, _ := dmenu.Output()
	dmenuOut := strings.TrimSpace(string(dmenuBytes))

	// run it, without a shell
	progParts := strings.Split(dmenuOut, " ")
	path, err := exec.LookPath(progParts[0])
	if err != nil {
		log.Fatal("executable not found in path")
	}
	prog := exec.Command(path, progParts[1:]...)
	prog.Start()

	// add to beginning of list
	newHistory := []string{dmenuOut}
	// if dmenu output in previous input
	if i, ok := historyMap[dmenuOut]; ok {
		// remove it
		before := cache.History[:i]
		after := cache.History[i+1:]
		cache.History = append(before, after...)
	}
	cache.History = append(newHistory, cache.History...)

	// serialize previous input list and write
	// serialize paths and write
	out, _ := os.Create(cacheName)
	enc := msgpack.NewEncoder(out)
	enc.Encode(&cache)
}

func regenerate(pathname string) Path {
	var p Path
	p.Dir = pathname
	fi, _ := os.Stat(pathname)
	p.Mtime = fi.ModTime().Unix()

	fileinfos, _ := ioutil.ReadDir(pathname)
	for _, fi := range fileinfos {
		// Is it an executable?
		if fi.IsDir() == false && fi.Mode()&0111 != 0 {
			p.Execs = append(p.Execs, fi.Name())
		}
	}
	return p
}
