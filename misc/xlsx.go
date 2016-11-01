package misc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

const (
	XLSTContentType = `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
	JSONContentType = `application/json`
	ZipContentType  = `application/zip`
)

var (
	ErrEmptyDocument = errors.New("empty file")
)

type XLSXFile struct {
	Sheets []*Sheet
	j2x    string
}

type Sheet struct {
	Name   string          `json:"name"`
	Header []string        `json:"header"`
	Rows   [][]interface{} `json:"rows"`
}

func (s *Sheet) AddHeader(h ...string) {
	s.Header = h
}

func (s *Sheet) AddRow(values ...interface{}) {
	for i, v := range values {
		switch rv := v.(type) {
		case fmt.Stringer:
			values[i] = rv.String()
		}
	}
	s.Rows = append(s.Rows, values)
}

func NewXLSXFile(json2xlsx string) *XLSXFile {
	return &XLSXFile{j2x: json2xlsx}
}

func (x *XLSXFile) WriteTo(w io.Writer) (n int64, err error) {
	if len(x.Sheets) == 0 {
		return 0, ErrEmptyDocument
	}
	var (
		cmd = exec.Command("python3", x.j2x)
		j   []byte
	)
	if j, err = json.Marshal(x.Sheets); err != nil {
		return
	}
	cmd.Stdin = bytes.NewReader(j)
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
	}
	return
}

func (x *XLSXFile) AddSheet(name string) (s *Sheet) {
	s = &Sheet{Name: name}
	x.Sheets = append(x.Sheets, s)
	return
}

type Sheeter interface {
	io.WriterTo
	AddSheet(name string) (s *Sheet)
}
