package reporting

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

var (
	ErrCampaignNotFound = errors.New("Campaign not found!")
)

func GenerateCampaignReport(res http.ResponseWriter, db *bolt.DB, cid string, from, to time.Time, cfg *config.Config) error {
	cmp := common.GetCampaign(cid, db, cfg)
	if cmp == nil {
		return ErrCampaignNotFound
	}

	// NOTE: report is inclusive of "from" and "to"
	st, err := GetCampaignStats(cid, db, cfg, from, to, false)
	if err != nil {
		return err
	}

	xf := misc.NewXLSXFile(cfg.JsonXlsxPath)
	setHighLevelSheet(xf, cmp, from, to, st.Total)
	setChannelLevelSheet(xf, from, to, st.Channel)
	setInfluencerLevelSheet(xf, from, to, st.Influencer)
	setContentLevelSheet(xf, from, to, st.Post)

	res.Header().Set("Content-Type", misc.XLSTContentType)
	if _, err := xf.WriteTo(res); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func setHighLevelSheet(xf misc.Sheeter, cmp *common.Campaign, from, to time.Time, tot *Totals) {
	sheet := xf.AddSheet("High Level Stats")
	sheet.AddHeader("Sway Stats")

	sheet.AddRow("Campaign Name", cmp.Name)
	sheet.AddRow("Report Timeframe",
		fmt.Sprintf("%d-%02d-%02d to %d-%02d-%02d",
			from.Year(),
			from.Month(),
			from.Day(),
			to.Year(),
			to.Month(),
			to.Day(),
		),
	)

	sheet.AddRow("")

	channels := ""
	if cmp.Instagram {
		channels += "Instagram"
	}
	if cmp.Facebook {
		if channels == "" {
			channels += "Facebook"
		} else {
			channels += ", Facebook"
		}
	}

	if cmp.Twitter {
		if channels == "" {
			channels += "Twitter"
		} else {
			channels += ", Twitter"
		}
	}

	if cmp.YouTube {
		if channels == "" {
			channels += "YouTube"
		} else {
			channels += ", YouTube"
		}
	}

	sheet.AddRow("Channels", channels)

	if tot != nil {
		sheet.AddRow("Total Influencers", tot.Influencers)
		sheet.AddRow("Total Engagements Generated", tot.Engagements)
		sheet.AddRow("Total Est Views", tot.Views)
		sheet.AddRow("Total Clicks", tot.Clicks)

		sheet.AddRow("")

		sheet.AddRow("Total spent", fmt.Sprintf("$%0.2f", tot.Spent))
	}
}

func setChannelLevelSheet(xf misc.Sheeter, from, to time.Time, channel map[string]*ReportStats) {
	sheet := xf.AddSheet("Channel Level")
	sheet.AddHeader(
		"Channel Name",
		"Likes",
		"Comments",
		"Shares",
		"Clicks",
		"Est Views",
		"Spent",
		"% of Total Engagements",
	)

	var totalEng float64
	for _, st := range channel {
		totalEng += float64(st.Likes + st.Comments + st.Shares + st.Clicks)
	}

	for platform, st := range channel {
		eng := (float64(st.Likes+st.Comments+st.Shares+st.Clicks) / totalEng) * 100
		sheet.AddRow(
			platform,
			st.Likes,
			st.Comments,
			st.Shares,
			st.Clicks,
			st.Views,
			fmt.Sprintf("$%0.2f", st.Spent),
			getPerc(eng),
		)
	}
}

func setInfluencerLevelSheet(xf misc.Sheeter, from, to time.Time, influencer map[string]*ReportStats) {
	sheet := xf.AddSheet("Influencer Level")
	sheet.AddHeader(
		"Social Network ID",
		"Social Network",
		"Influencer ID",
		"Likes",
		"Comments",
		"Shares",
		"Clicks",
		"Est Views",
		"Spent",
		"% of Total Engagements",
	)

	var totalEng float64
	for _, st := range influencer {
		totalEng += float64(st.Likes + st.Comments + st.Shares + st.Clicks)
	}

	for inf, st := range influencer {
		eng := (float64(st.Likes+st.Comments+st.Shares+st.Clicks) / totalEng) * 100
		sheet.AddRow(
			inf,
			st.Network,
			st.InfluencerId,
			st.Likes,
			st.Comments,
			st.Shares,
			st.Clicks,
			st.Views,
			fmt.Sprintf("$%0.2f", st.Spent),
			getPerc(eng),
		)
	}
}

func setContentLevelSheet(xf misc.Sheeter, from, to time.Time, content map[string]*ReportStats) {
	sheet := xf.AddSheet("Content Level")
	sheet.AddHeader(
		"Content",
		"Created",
		"Social Network ID",
		"Influencer ID",
		"Likes",
		"Comments",
		"Shares",
		"Clicks",
		"Est Views",
		"Spent",
		"% of Total Engagements",
	)

	var totalEng float64
	for _, st := range content {
		totalEng += float64(st.Likes + st.Comments + st.Shares + st.Clicks)
	}

	for url, st := range content {
		eng := (float64(st.Likes+st.Comments+st.Shares+st.Clicks) / totalEng) * 100
		sheet.AddRow(
			url,
			st.Published,
			st.PlatformId,
			st.InfluencerId,
			st.Likes,
			st.Comments,
			st.Shares,
			st.Clicks,
			st.Views,
			fmt.Sprintf("$%0.2f", st.Spent),
			getPerc(eng),
		)
	}
}

func getPerc(val float64) string {
	if val < 1 {
		return "<1%"
	}
	return fmt.Sprintf("%d", int32(val)) + "%"
}
