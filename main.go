package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func PipeReaderOutputToChan(r io.ReadCloser, label string, c chan<- string) {
	scanner := bufio.NewScanner(r)
	go func() {
		for scanner.Scan() {
			t := scanner.Text()
			c <- fmt.Sprintf("%v: %v", label, t)
			time.Sleep(1)
		}
	}()
}

func OutputLog(c <-chan string) {
	for {
		fmt.Println(<-c)
	}
}

type Proc struct {
	name string
	cmd  string
}

func ParseProcfile(path string) []Proc {
	var procs []Proc
	f, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(f), "\n")
	for _, l := range lines {
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		xs := strings.SplitN(l, ":", 2)
		name := xs[0]
		cmd := xs[1]
		procs = append(procs, Proc{name, cmd})
	}
	return procs
}

func cancelOnTermination(cancel context.CancelFunc, wg *sync.WaitGroup) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer wg.Done()
		log.Printf("received SIGTERM %v\n", <-s)
		cancel()
	}()
}

func CommandWait(wg *sync.WaitGroup, name string, arg ...string) *exec.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	cancelOnTermination(cancel, wg)

	cmd := exec.CommandContext(ctx, name, arg...)
	return cmd
}

func ColorString(i int, str string) string {
	return fmt.Sprintf("\033[1;%dm%s\033[0m", i, str)
}

func main() {
	colors := []int{32, 33, 34, 35, 36}
	var wg sync.WaitGroup
	c := make(chan string)

	procfile := ParseProcfile(os.Args[1])
	for i, proc := range procfile {
		wg.Add(1)
		go func(proc Proc, i int) {
			cmd := CommandWait(&wg, "/bin/sh", "-c", proc.cmd)
			stdoutIn, _ := cmd.StdoutPipe()
			stderrIn, _ := cmd.StderrPipe()

			label := ColorString(colors[i%len(colors)], proc.name)
			PipeReaderOutputToChan(stdoutIn, label, c)
			PipeReaderOutputToChan(stderrIn, label, c)

			go OutputLog(c)

			err := cmd.Start()
			if err != nil {
				log.Fatalf("cmd.Start() failed with '%s'\n", err)
			}

			fmt.Printf("Booting %v: %v\n", label, proc.cmd)
		}(proc, i)
	}
	wg.Wait()
	fmt.Println("exiting")
}
