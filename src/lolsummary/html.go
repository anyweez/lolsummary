package main

import (
	// "bufio"
	"bytes"
	// "fmt"
	"html/template"
	"log"
	"os"
	// "time"
)

const TEMPLATE_NAME = "template.html"

func WriteMetrics(metrics map[int]OutputRecord, filename string) error {
	// Load the template.
	tmpl, err := template.New(TEMPLATE_NAME).ParseFiles(TEMPLATE_NAME)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	buffer := bytes.NewBufferString("")
	tmpl.Execute(buffer, metrics)
	// fmt.Println(buffer.String())

	// Open an output file.
	fp, err := os.Create(filename)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	fp.Write(buffer.Bytes())
	fp.Close()

	return nil
}
