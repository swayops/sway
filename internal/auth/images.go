package auth

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

	"github.com/swayops/sway/config"

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

func SaveImageToDisk(fileNameBase, data, id, suffix string, minWidth, minHeight int) (string, error) {
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

func GetImageUrl(cfg *config.Config, bucket, typ, filename string, addDomain bool) string {
	var base string
	switch typ {
	case "dash":
		if addDomain {
			base = cfg.DashURL
		}

	case "inf":
		if addDomain {
			base = cfg.InfAppURL
		}
	default:
		log.Panicf("invalid type: %v", typ)
	}
	return base + "/" + filepath.Join(cfg.ImageUrlPath, bucket, filename)
}

// saveUserImage saves the user image to disk and sets User.ImageURL to the url for it if the image is a data:image/
func SaveUserImage(cfg *config.Config, u *User) error {
	if strings.HasPrefix(u.ImageURL, "data:image/") {
		filename, err := SaveImageToDisk(filepath.Join(cfg.ImagesDir, cfg.Bucket.User, u.ID), u.ImageURL, u.ID, "", 300, 300)
		if err != nil {
			return err
		}

		u.ImageURL = GetImageUrl(cfg, cfg.Bucket.User, "dash", filename, false)
	}

	if strings.HasPrefix(u.CoverImageURL, "data:image/") {
		filename, err := SaveImageToDisk(filepath.Join(cfg.ImagesDir, cfg.Bucket.User, u.ID),
			u.CoverImageURL, u.ID, "-cover", 300, 300)
		if err != nil {
			return err
		}

		u.CoverImageURL = GetImageUrl(cfg, cfg.Bucket.User, "dash", filename, false)
	}

	return nil
}
