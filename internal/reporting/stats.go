package reporting

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

// Structure of Reporting DB:
// Stores all transactions by day at the post level
// {
//     "1": { // Campaign ID
//         "2016-10-30||1||JennaMarbles||Instagram||http://www.instagram.com/post": { // Date::Name::Platform::postUrl
//             "payout": 3.4,
//             "likes": 0,
//             "comments": 0,
//             "shares": 0,
//             "views": 30,
//             "dislikes": 45,
//             "perks": 2
//         },
//         "2016-10-30||2||NigaHiga||Facebook||facebook.com/post": { // Date::InfId::Platform::postUrl
//             "payout": 3.4,
//             "likes": 0,
//             "comments": 0,
//             "shares": 0,
//             "views": 30,
//             "dislikes": 45,
//             "perks": 2
//         }
//     }
// }

var (
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
)

type Stats struct {
	Payout    float32 `json:"payout,omitempty"`
	Likes     int32   `json:"likes,omitempty"`
	Dislikes  int32   `json:"dislikes,omitempty"`
	Comments  int32   `json:"comments,omitempty"`
	Shares    int32   `json:"shares,omitempty"`
	Views     int32   `json:"views,omitempty"`
	Perks     int32   `json:"perks,omitempty"`  // Number of perks sent out
	Published int32   `json:"posted,omitempty"` // Epoch ts
}

func GetStats(deal *common.Deal, db *bolt.DB, cfg *config.Config, platformId string) (*Stats, string, error) {
	// Retrieves stats for this influencer id and deal for TODAY
	// If there is any stats key missing.. it will create and save!
	var (
		v   []byte
		rd  map[string]*Stats
		err error
	)
	if err := db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.ReportingBucket)).Get([]byte(deal.CampaignId))
		return nil
	}); err != nil {
		log.Println("Error retrieving reporting data", err)
	}

	if len(v) == 0 {
		// No reporting data for this campaign? Lets create a new campaign key!
		if rd, err = createStatsKey(deal.CampaignId, db, cfg); err != nil {
			log.Println("Error creating stats key for campaign!")
			return nil, "", err
		}
	} else {
		// CID has some reporting data.. this campaign has been accessed before!
		if err = json.Unmarshal(v, &rd); err != nil {
			log.Println("Error unmarshalling stats", err)
			return nil, "", err
		}
	}

	// Get a key specific to today and the deal's details!
	key := getStatsKey(deal, platformId)
	data, ok := rd[key]
	if !ok {
		// No data for this post/deal today!
		// We'll use fresh Stats and increment those for today!
		return &Stats{}, key, nil
	}

	// Stats key is returned so that in case day flips between the Get
	// and the Save.. we maintain the correct day's data!
	return data, key, nil
}

func GetStatsByCampaign(cid string, db *bolt.DB, cfg *config.Config) (map[string]*Stats, error) {
	// Retrieves all stats by campaign ID
	var (
		v   []byte
		err error
	)
	rd := make(map[string]*Stats)
	if err := db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.ReportingBucket)).Get([]byte(cid))
		return nil
	}); err != nil {
		log.Println("Error retrieving reporting data", err)
	}

	if len(v) == 0 {
		return rd, nil
	} else {
		if err = json.Unmarshal(v, &rd); err != nil {
			log.Println("Error unmarshalling stats", err)
			return rd, nil
		}
	}

	return rd, nil
}

func SaveStats(stats *Stats, deal *common.Deal, db *bolt.DB, cfg *config.Config, keyOverride, platformId string) error {
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.ReportingBucket)).Get([]byte(deal.CampaignId))

		var st map[string]*Stats
		if len(b) == 0 {
			// First save of this campaign!
			st = make(map[string]*Stats)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		// keyOverride is used to make sure that the key that we did the Get on
		// is the same one we save to
		key := keyOverride
		if key == "" {
			key = getStatsKey(deal, platformId)
		}
		st[key] = stats
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.ReportingBucket, deal.CampaignId, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when saving store", err)
		return err
	}
	return nil
}

func createStatsKey(cid string, db *bolt.DB, cfg *config.Config) (map[string]*Stats, error) {
	rd := make(map[string]*Stats)
	// Creates a key for the campaign
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var (
			b []byte
		)

		b = tx.Bucket([]byte(cfg.ReportingBucket)).Get([]byte(cid))

		if b, err = json.Marshal(&rd); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.ReportingBucket, cid, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when creating reporting key", err)
		return rd, err
	}
	return rd, nil
}

type TargetStats struct {
	Total      *Totals                 `json:"total"`
	Channel    map[string]*ReportStats `json:"channel"`
	Influencer map[string]*ReportStats `json:"influencer"`
	Post       map[string]*ReportStats `json:"post"`
}

type Totals struct {
	Influencers int32   `json:"infs,omitempty"`
	Engagements int32   `json:"eng,omitempty"`
	Likes       int32   `json:"likes,omitempty"`
	Views       int32   `json:"views,omitempty"`
	Spent       float32 `json:"spent,omitempty"`
}

type ReportStats struct {
	Likes        int32   `json:"likes,omitempty"`
	Comments     int32   `json:"comments,omitempty"`
	Shares       int32   `json:"shares,omitempty"`
	Views        int32   `json:"views,omitempty"`
	Spent        float32 `json:"spent,omitempty"`
	Perks        int32   `json:"perks,omitempty"`      // Perks sent
	PlatformId   string  `json:"platformId,omitempty"` // Screen name for the platform used for the deal
	Published    string  `json:"posted,omitempty"`     // Pretty string of date post was made
	InfluencerId string  `json:"infId,omitempty"`
	Network      string  `json:"network,omitempty"` // Social Network
}

// Generates TargetStats which will be used to generate reports
func getTargetStats(cid string, db *bolt.DB, cfg *config.Config, from, to time.Time) (*TargetStats, error) {
	tg := &TargetStats{}

	stats, err := GetStatsByCampaign(cid, db, cfg)
	if err != nil {
		return tg, err
	}

	// Retrieve the dates that this request requires
	dates := getDateRange(from, to)
	for k, st := range stats {
		for _, d := range dates {
			if strings.HasPrefix(k, d) {
				// This value falls in our target range!

				eng := st.Likes + st.Dislikes + st.Comments + st.Shares //+ st.Views

				var views int32
				if st.Views == 0 {
					// There are no concrete views so lets gueestimate!
					// Assume engagement rate is 4% of views!
					views = int32(float32(eng) / 0.04)
				} else {
					views += st.Views
				}

				if tg.Total == nil {
					tg.Total = &Totals{
						Engagements: eng,
						Likes:       st.Likes,
						Views:       views,
						Spent:       st.Payout,
					}
				} else {
					tg.Total.Engagements += eng
					tg.Total.Likes += st.Likes
					tg.Total.Views += views
					tg.Total.Spent += st.Payout
				}

				infId, platformId, channel, postUrl := getElementsFromKey(k)

				if tg.Channel == nil || len(tg.Channel) == 0 {
					tg.Channel = make(map[string]*ReportStats)
				}

				fillReportStats(channel, tg.Channel, st, views, infId, channel)

				if tg.Influencer == nil || len(tg.Influencer) == 0 {
					tg.Influencer = make(map[string]*ReportStats)
				}

				fillReportStats(platformId, tg.Influencer, st, views, infId, channel)

				if tg.Post == nil || len(tg.Post) == 0 {
					tg.Post = make(map[string]*ReportStats)
				}

				fillContentLevelStats(postUrl, platformId, st.Published, tg.Post, st, views, infId)

				continue
			}
		}
	}
	if tg.Total != nil && tg.Total.Influencers == 0 {
		tg.Total.Influencers = int32(len(tg.Influencer))
	}
	return tg, nil
}

func fillReportStats(key string, data map[string]*ReportStats, st *Stats, views int32, infId, channel string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		data[key] = &ReportStats{
			Likes:        st.Likes,
			Comments:     st.Comments,
			Shares:       st.Shares,
			Views:        views,
			Spent:        st.Payout,
			Perks:        st.Perks,
			InfluencerId: infId,
			Network:      channel,
		}
	} else {
		stats.Likes += st.Likes
		stats.Comments += st.Comments
		stats.Shares += st.Shares
		stats.Views += views
		stats.Spent += st.Payout
		stats.Perks += st.Perks
		stats.InfluencerId = infId
		stats.Network = channel
		data[key] = stats
	}
	return data
}

func fillContentLevelStats(key, platformId string, ts int32, data map[string]*ReportStats, st *Stats, views int32, infId string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		data[key] = &ReportStats{
			Likes:        st.Likes,
			Comments:     st.Comments,
			Shares:       st.Shares,
			Views:        views,
			Spent:        st.Payout,
			Perks:        st.Perks,
			PlatformId:   platformId,
			Published:    getPostDate(st.Published),
			InfluencerId: infId,
		}
	} else {
		stats.Likes += st.Likes
		stats.Comments += st.Comments
		stats.Shares += st.Shares
		stats.Views += views
		stats.Spent += st.Payout
		stats.Perks += st.Perks
		stats.PlatformId = platformId
		stats.Published = getPostDate(st.Published)
		stats.InfluencerId = infId
		data[key] = stats
	}
	return data
}
