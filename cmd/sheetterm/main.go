package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	sheet "github.com/knusbaum/9sheet"
	"github.com/olekukonko/tablewriter"
)

type cfg struct {
	editMode bool
}

func doCommand(st *sheet.Sheet, cfg *cfg, s *bufio.Scanner) (string, error) {
	if !s.Scan() {
		return "", io.EOF
	}

	cmd := strings.SplitN(s.Text(), " ", 3)
	if len(cmd) == 0 {
		return "", nil
	}
	switch cmd[0] {
	case "SET":
		if len(cmd) < 3 {
			return "", fmt.Errorf("SET expects 2 arguments - SET [address] [value]")
		}
		err := st.SetContent(cmd[1], cmd[2])
		if err != nil {
			return "", err
		}
	case "EDIT":
		cfg.editMode = !cfg.editMode
		return fmt.Sprintf("EDITMODE = %t", cfg.editMode), nil
	default:
		return "", fmt.Errorf("Unknown command %s", cmd[0])
	}
	return "OK", nil
}

func writeSheet(s *sheet.Sheet, c *cfg) {
	var b bytes.Buffer
	s.WriteCSV2(&b, true, c.editMode)
	t, err := tablewriter.NewCSVReader(os.Stdout, csv.NewReader(&b), false)
	if err != nil {
		fmt.Printf("Failed to create table: %s\n", err)
		return
	}
	t.Render()
}

func main() {
	s := sheet.NewSheet()
	s.SetContent("A1", "1")
	s.SetContent("B2", "1")
	s.SetContent("C3", "1")
	a, _ := sheet.CellAddr("A1")
	s.WriteRange(a, s.MaxAddr(), os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	var c cfg
	writeSheet(s, &c)
	for {

		// 		var b bytes.Buffer
		// 		s.WriteCSV2(&b, true, c.editMode)
		// 		t, err := tablewriter.NewCSVReader(os.Stdout, csv.NewReader(&b), false)
		// 		if err != nil {
		// 			fmt.Printf("Failed to create table: %s\n", err)
		// 			return
		// 		}
		// 		t.Render()
		var (
			response string
			err      error
		)
		for {
			fmt.Printf("STARTING INPUT LOOP.\n")
			fmt.Printf("9sheet > ")
			response, err = doCommand(s, &c, scanner)
			if err == io.EOF {
				return
			} else if err != nil {
				fmt.Printf("%s\n", err)
			} else {

				break
			}
		}
		writeSheet(s, &c)
		fmt.Println(response)
	}
}
