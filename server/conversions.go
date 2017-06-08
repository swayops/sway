package server

import (
	"fmt"
	"log"
	"time"

	"github.com/swayops/converter/pixel"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

func fillConversions(srv *Server) error {
	// Traverses active deals in our system and checks
	// to see whether they have been satisfied or have timed out
	completedDeals, err := common.GetAllDeals(srv.db, srv.Cfg, false, true)
	if err != nil {
		return err
	}

	// Get a list of advertisers that HAVE atleast one conversion
	// That way we can ignore ones that don't

	for _, deal := range completedDeals {
		// Traverse over completed deals in the system
		// and fill in their conversion stats
		stats := deal.TotalStats()
		if stats.GetUniqueClicks() == 0 {
			continue
		}

		// Get conversions for the last 30 days
		convs, err := getConversions(deal, srv.Cfg.ConverterURL)
		if err != nil {
			srv.Alert("Error querying for conversions for "+deal.Id+":"+deal.CampaignId, err)
			continue
		}

		if len(convs) == 0 {
			continue
		}

		inf, ok := srv.auth.Influencers.Get(deal.InfluencerId)
		if !ok {
			log.Println("Missing influencer!", deal.InfluencerId)
			continue
		}

		for _, cDeal := range inf.CompletedDeals {
			if cDeal.Id == deal.Id {
				cDeal.AddConversions(convs)
			}

			// Save the deal in influencers and campaigns
			if err := saveAllCompletedDeals(srv, inf); err != nil {
				// Insert file informant notification
				srv.Alert("Failed to save completed deals", err)
			}
		}

	}
	return nil
}

func getConversions(deal *common.Deal, endpoint string) ([]pixel.Conversion, error) {
	now := time.Now()
	end := now.Unix()
	start := now.AddDate(0, 0, -30).Unix()

	var conversions []pixel.Conversion
	err := misc.Request("GET", fmt.Sprintf("%s/%s/%s/%s/%s/%d/%d", "stats", deal.Id, deal.CampaignId, deal.AdvertiserId, start, end), "", &conversions)
	if err != nil {
		return conversions, err
	}

	return conversions, nil
}
