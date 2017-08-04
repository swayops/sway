package reporting

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/pdf"
)

var (
	ErrCampaignNotFound = errors.New("Campaign not found!")
)

type ReportInfluencer struct {
	Name, Date, Picture, Link, Caption     string
	Views, Likes, Comments, Shares, Clicks int32
}

func GenerateCampaignReport(c *gin.Context, auth *auth.Auth, db *bolt.DB, cid string, from, to time.Time, isPDF bool, cfg *config.Config) error {
	cmp := common.GetCampaign(cid, db, cfg)
	if cmp == nil {
		return ErrCampaignNotFound
	}

	// NOTE: report is inclusive of "from" and "to"
	st, err := GetCampaignStats(cid, db, cfg, from, to, false)
	if err != nil {
		return err
	}

	if isPDF {
		load := make(map[string]interface{})
		load["Campaign Name"] = cmp.Name
		if st.Total != nil {
			load["Spent"] = fmt.Sprintf("$%0.2f", st.Total.Spent)
			load["Views"] = st.Total.Views
			load["Engagements"] = st.Total.Engagements
		}
		type ReportInfluencer struct {
			Name, Date, Picture, Link, Caption     string
			Views, Likes, Comments, Shares, Clicks int32
		}
		if st.Influencer != nil {
			infs := []*ReportInfluencer{}
			for _, stats := range st.Influencer {
				deal, ok := cmp.Deals[stats.DealID]
				if !ok {
					continue
				}

				inf, ok := auth.Influencers.Get(stats.InfluencerId)
				if !ok {
					continue
				}

				picture := inf.GetProfilePicture()
				if dealPic := deal.Picture(); dealPic != "" {
					picture = dealPic
				}

				caption := deal.Caption()
				if len(caption) > 200 {
					caption = caption[:200] + "..."
				}
				rptInf := &ReportInfluencer{
					Name:     inf.Name,
					Date:     time.Unix(int64(deal.Published()), 0).Format(time.RFC1123),
					Picture:  picture,
					Link:     deal.PostUrl,
					Caption:  caption,
					Views:    stats.Views,
					Likes:    stats.Likes,
					Comments: stats.Comments,
					Shares:   stats.Shares,
					Clicks:   stats.Clicks,
				}
				infs = append(infs, rptInf)
			}
			load["Influencers"] = infs
		}

		tmpl := templates.CampaignReportExport.Render(load)

		c.Header("Content-type", "application/octet-stream")
		c.Header("Content-Disposition", fmt.Sprintf("attachment;Filename=%s.pdf", cmp.Name+"_report"))

		if err := pdf.ConvertHTMLToPDF(tmpl, c.Writer, cfg); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
		}
	} else {
		xf := misc.NewXLSXFile(cfg.JsonXlsxPath)
		setHighLevelSheet(xf, cmp, from, to, st.Total)
		setChannelLevelSheet(xf, from, to, st.Channel)
		setInfluencerLevelSheet(xf, from, to, st.Influencer)
		setContentLevelSheet(xf, from, to, st.Post)

		c.Header("Content-Type", misc.XLSTContentType)
		if _, err := xf.WriteTo(c.Writer); err != nil {
			log.Println(err)
			return err
		}
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
		sheet.AddRow("Total Unique Clicks", tot.Uniques)
		sheet.AddRow("Total Conversions", tot.Conversions)

		sheet.AddRow("")

		sheet.AddRow("CPM", fmt.Sprintf("$%0.2f", getCPM(tot.Spent, float64(tot.Views))))
		sheet.AddRow("CPE (Cost per Engagement)", fmt.Sprintf("$%0.2f", getCPE(tot.Spent, float64(tot.Engagements))))
		sheet.AddRow("CPV (Cost per View)", fmt.Sprintf("$%0.2f", getCPV(tot.Spent, float64(tot.Views))))

		sheet.AddRow("")

		sheet.AddRow("Total Spent", fmt.Sprintf("$%0.2f", tot.Spent))
	}
}

func setChannelLevelSheet(xf misc.Sheeter, from, to time.Time, channel map[string]*ReportStats) {
	sheet := xf.AddSheet("Channel Level")
	sheet.AddHeader(
		"Channel Name",
		"Likes",
		"Comments",
		"Shares",
		"Clicks / Uniques",
		"Est Views",
		"Conversions",
		"Spent",
		"CPM",
		"CPE",
		"CPV",
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
			fmt.Sprintf("%d / %d", st.Clicks, st.Uniques),
			st.Views,
			st.Conversions,
			fmt.Sprintf("$%0.2f", st.Spent),
			fmt.Sprintf("$%0.2f", getCPM(st.Spent, float64(st.Views))),
			fmt.Sprintf("$%0.2f", getCPE(st.Spent, float64(getEngagementsFromReport(st)))),
			fmt.Sprintf("$%0.2f", getCPV(st.Spent, float64(st.Views))),
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
		"Clicks / Uniques",
		"Est Views",
		"Conversions",
		"Spent",
		"CPM",
		"CPE",
		"CPV",
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
			fmt.Sprintf("%d / %d", st.Clicks, st.Uniques),
			st.Views,
			st.Conversions,
			fmt.Sprintf("$%0.2f", st.Spent),
			fmt.Sprintf("$%0.2f", getCPM(st.Spent, float64(st.Views))),
			fmt.Sprintf("$%0.2f", getCPE(st.Spent, float64(getEngagementsFromReport(st)))),
			fmt.Sprintf("$%0.2f", getCPV(st.Spent, float64(st.Views))),
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
		"Clicks / Uniques",
		"Est Views",
		"Conversions",
		"Spent",
		"CPM",
		"CPE",
		"CPV",
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
			fmt.Sprintf("%d / %d", st.Clicks, st.Uniques),
			st.Views,
			st.Conversions,
			fmt.Sprintf("$%0.2f", st.Spent),
			fmt.Sprintf("$%0.2f", getCPM(st.Spent, float64(st.Views))),
			fmt.Sprintf("$%0.2f", getCPE(st.Spent, float64(getEngagementsFromReport(st)))),
			fmt.Sprintf("$%0.2f", getCPV(st.Spent, float64(st.Views))),
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
