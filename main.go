package main

import (
	"fmt"
	"strings"
	"bufio"
	
	"github.com/knusbaum/go9p"
	"github.com/knusbaum/go9p/fs"
)

func main() {
	sheetFS := fs.NewFS("glenda", "glenda", 0555)

	outputStream := fs.NewStream(100, false)
	updates := fs.NewStreamFile(sheetFS.NewStat("updates", "glenda", "glenda", 0444), outputStream)
	sheetFS.Root.AddChild(updates)

	inputStream := fs.NewStream(100, false)
	ctl := fs.NewStreamFile(sheetFS.NewStat("ctl", "glenda", "glenda", 0222), inputStream)
	sheetFS.Root.AddChild(ctl)

	s := NewSheet()
	s.OnCellUpdated = func(addr string, c *Cell) {
		content, _ := c.Content()
		outputStream.Write([]byte(fmt.Sprintf("%s %d %s\n", addr, len(content), content)))
	}

	err := s.Read(strings.NewReader("A4 11 Hello World\n"))
	fmt.Printf("%v\n", []byte("A4 11 Hello World\n"))
	if err != nil {
		fmt.Printf("DDDDD %s\n", err)
	}

	go func() {
		r := inputStream.AddReader()
		br := bufio.NewReader(r)
		for {
			err := s.Read(br)
			if err != nil {
				fmt.Printf("Failed to read: %s\n", err)
			}
		}
	}()

//	r := inputStream.AddReader()
//	go func() {
//		bs := make([]byte, 1000)
//		for {
//			n, err := r.Read(bs)
//			if err != nil {
//				fmt.Printf("ERROR: %s\n", err)
//			} else {
//				fmt.Printf("%s", string(bs[:n]))
//				fmt.Printf("%v\n", bs[:n])
//			}
//		}
//	}()

	

	// Listen on port 9999
	go9p.PostSrv("sheetfs", sheetFS.Server())
}