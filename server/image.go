package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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

func saveImageToDisk(fileNameBase, data, id, suffix string, minWidth, minHeight int) (string, error) {
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
		return "", fmt.Errorf("Invalid size (%dx%d), min size is %dx%d!", imgCfg.Width, imgCfg.Height, minWidth, minHeight)
	}

	fileName := fileNameBase + suffix + "." + fm
	err = ioutil.WriteFile(fileName, buff.Bytes(), 0644)

	return id + suffix + "." + fm, err
}

func getImageUrl(s *Server, bucket, typ, filename string, addDomain bool) string {
	var base string
	switch typ {
	case "dash":
		if addDomain {
			base = s.Cfg.DashURL
		}

	case "inf":
		if addDomain {
			base = s.Cfg.InfAppURL
		}
	default:
		log.Panicf("invalid type: %v", typ)
	}
	return base + "/" + filepath.Join(s.Cfg.ImageUrlPath, bucket, filename)
}
