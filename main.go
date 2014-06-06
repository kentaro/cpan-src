package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/howeyc/fsnotify"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	module := os.Args[1]

	if module == "" {
		fmt.Println("usage: $ cpan-src <Foo::Bar>")
		os.Exit(1)
	}

	if !hasGhq() {
		fmt.Println("you need to install ghq in advance")
		os.Exit(1)
	}

	if !hasCpanm() {
		fmt.Println("you need to install cpanm in advance")
		os.Exit(1)
	}

	dir := perlModuleInstallDir()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create watcher: %s", err)
	}

	done := &sync.WaitGroup{}

	go func() {
		done.Add(1)
		defer done.Done()

		cmd := exec.Command("cpanm", module)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()

		if err != nil {
			log.Printf("an error occurred while executing cpanm: %s", err)
		}
	}()

	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				handleDirectory(ev, dir, done)
			case err := <-watcher.Error:
				log.Println(err)
			}
		}
	}()

	err = watcher.Watch(dir)
	if err != nil {
		log.Panicf("failed to start watching dir: %s", err)
	}
	log.Printf("started watching: %s\n", dir)

	done.Wait()
	watcher.Close()
}

func hasCpanm() (result bool) {
	cmd := exec.Command("which", "cpanm")
	err := cmd.Run()
	return err == nil
}

func hasGhq() (result bool) {
	cmd := exec.Command("which", "ghq")
	err := cmd.Run()
	return err == nil
}

func perlModuleInstallDir() (dir string) {
	var buf bytes.Buffer
	cmd := exec.Command("perl", "-MConfig", "-e", `print $Config{sitearchexp}`)
	cmd.Stdout = &buf
	err := cmd.Run()

	if err != nil {
		log.Panicf("failed to retrieve the install dir of Perl modules: %s", err)
	}

	dir = buf.String() + "/.meta"
	return
}

func handleDirectory(ev *fsnotify.FileEvent, baseDir string, done *sync.WaitGroup) {
	done.Add(1)

	fileInfo, err := os.Stat(ev.Name)

	if err != nil {
		log.Printf("failed to retrieve file info: %s", err)
		return
	}

	if !fileInfo.IsDir() {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create watcher: %s", err)
	}

	go func() {
		for {
			select {
			case fileEv := <-watcher.Event:
				handleFile(fileEv, done)
			case err := <-watcher.Error:
				log.Println(err)
			}
		}
	}()

	err = watcher.Watch(ev.Name)
	if err != nil {
		log.Panicf("failed to start watching dir: %s", err)
	}

	defer func() {
		done.Done()
		watcher.Close()
	}()

	log.Println("event: ", ev.Name)
}

func handleFile(ev *fsnotify.FileEvent, done *sync.WaitGroup) {
	log.Printf("file found: %s\n", ev.Name)
	if strings.HasSuffix(ev.Name, "MYMETA.json") {
		done.Add(1)
		defer done.Done()

		handleJSON(ev.Name)
	}
}

var meta struct {
	Resources Resources `json:"resources"`
}

type Resources struct {
	Repository Repository `json:"repository"`
}

type Repository struct {
	Url string `json:"url"`
}

func handleJSON(fileName string) {
	file, err := os.Open(fileName)

	if err != nil {
		log.Printf("failed to open MYMETA.json: %s", err)
		return
	}

	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&meta); err != nil {
		log.Printf("failed to parse MYMETA.json: %s", err)
	}

	fmt.Printf("%v", meta)
	return
}
