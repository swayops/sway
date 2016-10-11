package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type UploadImage struct {
	Data     string `json:"data,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
}

var (
	ErrBucket       = errors.New("Invalid bucket!")
	ErrInvalidImage = errors.New("Invalid image!")
)

func saveImageToDisk(fileNameBase, data, id string, minWidth, minHeight int) (string, error) {
	idx := strings.Index(data, ";base64,")
	if idx < 0 {
		return "", ErrInvalidImage
	}

	os.MkdirAll(filepath.Dir(fileNameBase), 0755)

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data[idx+8:]))
	buff := bytes.Buffer{}
	_, err := buff.ReadFrom(reader)
	if err != nil {
		return "", err
	}

	imgCfg, fm, err := image.DecodeConfig(bytes.NewReader(buff.Bytes()))
	if err != nil {
		return "", err
	}

	if imgCfg.Width < minWidth || imgCfg.Height < minHeight {
		return "", fmt.Errorf("Invalid size, min size is %dx%d!", minWidth, minHeight)
	}

	fileName := fileNameBase + "." + fm
	err = ioutil.WriteFile(fileName, buff.Bytes(), 0644)

	return id + "." + fm, err
}

func getImageUrl(s *Server, bucket, filename string) string {
	return s.Cfg.ServerURL + "/" + filepath.Join(s.Cfg.ImageUrlPath, bucket, filename)
}
