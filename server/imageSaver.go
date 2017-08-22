package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/boltdb/bolt"

	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

func imageSaver(srv *Server) {
	// Saves completed deal post pics
	for _, inf := range srv.auth.Influencers.GetAll() {
		var updated bool
		for _, deal := range inf.CompletedDeals {
			// If the url contains swayops.. means its been saved!
			if deal.Instagram != nil && deal.Instagram.Thumbnail != "" && !strings.Contains(deal.Instagram.Thumbnail, "swayops") && misc.Ping(deal.Instagram.Thumbnail) == nil {
				url, err := saveImageFromURL(srv, deal.Instagram.Thumbnail, deal)
				if err != nil {
					srv.Alert(fmt.Sprintf("Error saving image for %s: %s", inf.Id, deal.Instagram.Thumbnail), err)
					continue
				}
				deal.Instagram.Thumbnail = url
				updated = true
			} else if deal.YouTube != nil && deal.YouTube.Thumbnail != "" && !strings.Contains(deal.YouTube.Thumbnail, "swayops") && misc.Ping(deal.YouTube.Thumbnail) == nil {
				url, err := saveImageFromURL(srv, deal.YouTube.Thumbnail, deal)
				if err != nil {
					srv.Alert(fmt.Sprintf("Error saving image for %s: %s", inf.Id, deal.YouTube.Thumbnail), err)
					continue
				}
				deal.YouTube.Thumbnail = url
				updated = true
			}
		}

		if updated {
			// save influencer
			if err := srv.db.Update(func(tx *bolt.Tx) error {
				// Save the influencer since we just updated it's URL
				if err := saveInfluencer(srv, tx, inf); err != nil {
					log.Println("Errored saving influencer", err)
					return err
				}
				return nil
			}); err != nil {
				srv.Alert(fmt.Sprintf("Error saving image for %s", inf.Id), err)
			}
		}
	}
}

func saveImageFromURL(srv *Server, thumbnail string, deal *common.Deal) (string, error) {
	l := strings.Split(thumbnail, ".")
	if len(l) == 1 {
		return "", fmt.Errorf("Invalid URL")
	}

	suffix := l[len(l)-1]
	name := deal.Id + "-" + deal.InfluencerId + "." + suffix

	var url string
	response, err := http.Get(thumbnail)
	if err != nil {
		return url, err
	}

	defer response.Body.Close()

	filename := srv.Cfg.ImagesDir + "deal/" + name

	file, err := os.Create(filename)
	if err != nil {
		return url, err
	}

	_, err = io.Copy(file, response.Body)
	if err != nil {
		return url, err
	}
	file.Close()

	url = fmt.Sprintf("%s/%sdeal/%s", srv.Cfg.DashURL, srv.Cfg.ImageUrlPath, name)
	return url, nil
}
