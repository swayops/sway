package influencer

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

// The json struct accepted by the putInfluencer method
type InfluencerLoad struct {
	InstagramId string `json:"instagram,omitempty"`
	FbId        string `json:"facebook,omitempty"`
	TwitterId   string `json:"twitter,omitempty"`
	YouTubeId   string `json:"youtube,omitempty"`

	InviteCode string         `json:"inviteCode,omitempty"` // Encoded string showing talent agency id
	Geo        *geo.GeoRecord `json:"geo,omitempty"`        // User inputted geo via app

	Male   bool `json:"male,omitempty"`
	Female bool `json:"female,omitempty"`

	Categories []string `json:"categories,omitempty"`

	Address *lob.AddressLoad `json:"address,omitempty"`

	IP string `json:"ip,omitempty"` // Used to generate location

	// DealPing bool `json:"dealPing,omitempty"` // If true.. send influencer deals every 24 hours
}

type Influencer struct {
	Id string `json:"id,omitempty"`

	// Full name of the influencer
	Name         string `json:"name,omitempty"`
	EmailAddress string `json:"email,omitempty"`
	CreatedAt    int32  `json:"createdAt,omitempty"`

	// Banned if this person has been seen deleting a completed
	// deal post
	Banned bool `json:"banned,omitempty"`

	Address *lob.AddressLoad `json:"address,omitempty"`

	// Agency this influencer belongs to
	AgencyId string `json:"agencyId,omitempty"`

	// References to the social media accounts this influencer owns
	Facebook         *facebook.Facebook   `json:"facebook,omitempty"`
	Instagram        *instagram.Instagram `json:"instagram,omitempty"`
	Twitter          *twitter.Twitter     `json:"twitter,omitempty"`
	YouTube          *youtube.YouTube     `json:"youtube,omitempty"`
	LastSocialUpdate int32                `json:"lastSocialUpdate,omitempty"`

	// Used for the API exclusively (set in the influencer's Clean method)
	// Used for the getAllInfluencers* methods to provide concise structs
	FbUsername      string `json:"fbUsername,omitempty"`
	InstaUsername   string `json:"instaUsername,omitempty"`
	TwitterUsername string `json:"twitterUsername,omitempty"`
	YTUsername      string `json:"youtubeUsername,omitempty"`

	// Set and created by the IP
	Geo *geo.GeoRecord `json:"geo,omitempty"`

	Male   bool `json:"male,omitempty"`
	Female bool `json:"female,omitempty"`

	// Influencer inputted category they belong to
	Categories []string `json:"categories,omitempty"`

	// Active accepted deals by the influencer that have not yet been completed
	ActiveDeals []*common.Deal `json:"activeDeals,omitempty"`
	// Completed and approved deals by the influencer
	CompletedDeals []*common.Deal `json:"completedDeals,omitempty"`

	// Number of times the influencer has unassigned themself from a deal
	Cancellations int32 `json:"cancellations,omitempty"`
	// Number of times the influencer has timed out on a deal
	Timeouts int32 `json:"timeouts,omitempty"`

	// Sway Rep scores by date
	Rep        map[string]float64 `json:"historicRep,omitempty"`
	CurrentRep float64            `json:"rep,omitempty"`

	PendingPayout  float64 `json:"pendingPayout,omitempty"`
	RequestedCheck int32   `json:"requestedCheck,omitempty"`
	// Last check that was mailed
	LastCheck int32 `json:"lastCheck,omitempty"`
	// Lob check ids mailed out to this influencer
	Payouts []*lob.Check `json:"payouts,omitempty"`

	// Tax Information
	SignatureId  string `json:"sigId,omitempty"`
	HasSigned    bool   `json:"hasSigned,omitempty"`
	RequestedTax int32  `json:"taxRequest,omitempty"`

	// If true.. send influencer deals every 24 hours + campaign emails
	DealPing  bool  `json:"dealPing,omitempty"`
	LastEmail int32 `json:"lastEmail,omitempty"`

	// Set only in getInfluencersByAgency to save us a stats endpoint hit
	AgencySpend     float64 `json:"agSpend,omitempty"`
	InfluencerSpend float64 `json:"infSpend,omitempty"`
}

func New(id, name, twitterId, instaId, fbId, ytId string, m, f bool, inviteCode, defAgencyID, email, ip string, cats []string, address *lob.AddressLoad, created int32, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id:           id,
		Name:         name,
		Male:         m,
		Female:       f,
		Categories:   cats,
		DealPing:     true, // Deal ping is true by default!
		EmailAddress: misc.TrimEmail(email),
		CreatedAt:    created,
	}

	if address != nil {
		addr, err := lob.VerifyAddress(address, cfg.Sandbox)
		if err != nil {
			return nil, err
		}
		inf.Address = addr
	}

	agencyId := common.GetIDFromInvite(inviteCode)
	if agencyId == "" {
		agencyId = defAgencyID
	}

	inf.AgencyId = agencyId

	err := inf.NewInsta(instaId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewFb(fbId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewTwitter(twitterId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewYouTube(ytId, cfg)
	if err != nil {
		return inf, err
	}

	if ip != "" {
		inf.Geo = geo.GetGeoFromIP(cfg.GeoDB, ip)
	}

	inf.setSwayRep()
	inf.LastSocialUpdate = int32(time.Now().Unix())
	return inf, nil
}

func (inf *Influencer) NewFb(id string, cfg *config.Config) error {
	if len(id) > 0 {
		fb, err := facebook.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Facebook = fb
	}
	return nil
}

func (inf *Influencer) NewInsta(id string, cfg *config.Config) error {
	if len(id) > 0 {
		insta, err := instagram.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Instagram = insta
	}
	return nil
}

func (inf *Influencer) NewTwitter(id string, cfg *config.Config) error {
	if len(id) > 0 {
		tw, err := twitter.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Twitter = tw
	}
	return nil
}

func (inf *Influencer) NewYouTube(id string, cfg *config.Config) error {
	if len(id) > 0 {
		yt, err := youtube.New(id, cfg)
		if err != nil {
			return err
		}
		inf.YouTube = yt
	}
	return nil
}

func (inf *Influencer) UpdateAll(cfg *config.Config) (err error) {
	if inf.Banned {
		return nil
	}

	// Used by sway engine to periodically update influencer data

	// Always allow updates if they have an active deal
	// i.e. skip this if statement
	if len(inf.ActiveDeals) == 0 {
		// If you've been updated in the last 14-22 days and
		// have no active deals.. screw you!
		// NO SOUP FOR YOU!
		// NOTE: The random integer is so that we don't update
		// a large group of influencers at the same time (they become
		// in sync due to the same len check).
		if misc.WithinLast(inf.LastSocialUpdate, 24*misc.Random(14, 21)) {
			return nil
		}
	}

	// We only traverse over latest posts when
	// looking for a satisfied deal.. so if we're just getting
	// stats for influencer who have no active deals..
	// we don't need to save all those massive post lists..
	// hence why we have the savePosts bool!
	savePosts := len(inf.ActiveDeals) > 0
	if inf.Facebook != nil {
		if err = inf.Facebook.UpdateData(cfg, savePosts); err != nil {
			return err
		}
	}
	if inf.Instagram != nil {
		if err = inf.Instagram.UpdateData(cfg, savePosts); err != nil {
			return err
		}
	}
	if inf.Twitter != nil {
		if err = inf.Twitter.UpdateData(cfg, savePosts); err != nil {
			return err
		}
	}
	if inf.YouTube != nil {
		if err = inf.YouTube.UpdateData(cfg, savePosts); err != nil {
			return err
		}
	}

	inf.setSwayRep()
	inf.LastSocialUpdate = int32(time.Now().Unix())

	return nil
}

func (inf *Influencer) UpdateCompletedDeals(cfg *config.Config, activeCampaigns map[string]common.Campaign) (err error) {
	// Update data for all completed deal posts
	var (
		ok  bool
		ban error
	)

	for _, deal := range inf.CompletedDeals {
		if _, ok = activeCampaigns[deal.CampaignId]; !ok {
			// Don't update deals for campaigns that aren't
			// active anymore! NO POINT!
			continue
		}

		if deal.Tweet != nil {
			if ban, err = deal.Tweet.UpdateData(cfg); err != nil {
				log.Println("Tweet erred out", err)
				return err
			}
		} else if deal.Facebook != nil {
			if err = deal.Facebook.UpdateData(cfg); err != nil {
				return err
			}
		} else if deal.Instagram != nil {
			if ban, err = deal.Instagram.UpdateData(cfg); err != nil {
				return err
			}
		} else if deal.YouTube != nil {
			if err = deal.YouTube.UpdateData(cfg); err != nil {
				return err
			}
		}

		if ban != nil {
			// This person deleted a deal!
			// Insert into BAN.log and let admin
			// decide!
			if err := cfg.Loggers.Log("ban", map[string]string{
				"infId":      inf.Id,
				"dealId":     deal.Id,
				"campaignId": deal.CampaignId,
			}); err != nil {
				log.Println("Failed to log banned user!", inf.Id, deal.CampaignId)
			}
		}
	}
	return nil
}

func (inf *Influencer) GetPlatformId(deal *common.Deal) string {
	// Gets the user id for the platform based on the deal
	if deal.Tweet != nil && inf.Twitter != nil {
		return inf.Twitter.Id
	} else if deal.Facebook != nil && inf.Facebook != nil {
		return inf.Facebook.Id
	} else if deal.Instagram != nil && inf.Instagram != nil {
		return inf.Instagram.UserName
	} else if deal.YouTube != nil && inf.YouTube != nil {
		return inf.YouTube.UserName
	}
	return ""
}

func (inf *Influencer) GetFollowers() int64 {
	var fw int64
	if inf.Facebook != nil {
		fw += int64(inf.Facebook.Followers)
	}
	if inf.Instagram != nil {
		fw += int64(inf.Instagram.Followers)
	}
	if inf.Twitter != nil {
		fw += int64(inf.Twitter.Followers)
	}
	if inf.YouTube != nil {
		fw += int64(inf.YouTube.Subscribers)
	}
	return fw
}

func (inf *Influencer) GetNetworks() []string {
	var networks []string
	if inf.Facebook != nil {
		networks = append(networks, "Facebook")
	}
	if inf.Instagram != nil {
		networks = append(networks, "Instagram")
	}
	if inf.Twitter != nil {
		networks = append(networks, "Twitter")
	}
	if inf.YouTube != nil {
		networks = append(networks, "YouTube")
	}
	return networks
}

func (inf *Influencer) GetPostURLs(ts int32) []string {
	var urls []string

	for _, deal := range inf.CompletedDeals {
		if ts != 0 && deal.Completed < ts {
			continue
		}
		if deal.Tweet != nil {
			urls = append(urls, deal.Tweet.PostURL)
		} else if deal.Facebook != nil {
			urls = append(urls, deal.Facebook.PostURL)
		} else if deal.Instagram != nil {
			urls = append(urls, deal.Instagram.PostURL)
		} else if deal.YouTube != nil {
			urls = append(urls, deal.YouTube.PostURL)
		}
	}

	return urls
}

func (inf *Influencer) setSwayRep() {
	// Considers the following and returns a sway rep score:
	// - Averages per post (likes, comments, shares etc)
	// - Followers
	// - Completed deals
	// - Timeouts
	// - Cancellations

	var rep float64
	if inf.Facebook != nil {
		rep += inf.Facebook.GetScore()
	}
	if inf.Instagram != nil {
		rep += inf.Instagram.GetScore()
	}
	if inf.Twitter != nil {
		rep += inf.Twitter.GetScore()
	}
	if inf.YouTube != nil {
		rep += inf.YouTube.GetScore()
	}

	rep = rep * (1 + float64(len(inf.CompletedDeals))*float64(0.5))

	rep = degradeRep(inf.Timeouts, rep)
	rep = degradeRep(inf.Cancellations, rep)

	if inf.Rep == nil {
		inf.Rep = make(map[string]float64)
	}

	inf.Rep[getDate()] = rep
	inf.CurrentRep = rep
}

func (inf *Influencer) CleanAssignedDeals() []*common.Deal {
	var (
		cleanDeals []*common.Deal
	)

	for _, deal := range inf.ActiveDeals {
		deal.Platforms = []string{}
		deal.Spendable = 0
		cleanDeals = append(cleanDeals, deal)
	}

	return cleanDeals
}

func (inf *Influencer) CleanCompletedDeals() []*common.Deal {
	var (
		cleanDeals []*common.Deal
	)

	for _, deal := range inf.CompletedDeals {
		if deal.Tweet != nil {
			deal.Tweet = nil
		} else if deal.Facebook != nil {
			deal.Facebook = nil
		} else if deal.Instagram != nil {
			deal.Instagram = nil
		} else if deal.YouTube != nil {
			deal.YouTube = nil
		}
		deal.Platforms = []string{}
		deal.Spendable = 0
		cleanDeals = append(cleanDeals, deal)
	}

	return cleanDeals
}

func (inf *Influencer) Clean() *Influencer {
	if inf.Facebook != nil {
		inf.FbUsername = inf.Facebook.Id
		inf.Facebook = nil
	}
	if inf.Instagram != nil {
		inf.InstaUsername = inf.Instagram.UserName
		inf.Instagram = nil
	}
	if inf.Twitter != nil {
		inf.TwitterUsername = inf.Twitter.Id
		inf.Twitter = nil
	}
	if inf.YouTube != nil {
		inf.YTUsername = inf.YouTube.UserName
		inf.YouTube = nil
	}
	inf.Rep = nil

	return inf
}

func (inf *Influencer) GetLatestGeo() *geo.GeoRecord {
	if inf.Instagram != nil && inf.Instagram.LastLocation != nil {
		return inf.Instagram.LastLocation
	} else if inf.Twitter != nil && inf.Twitter.LastLocation != nil {
		return inf.Twitter.LastLocation
	}

	if inf.Address != nil {
		// Validity already been checked for state
		// and country in the setAddress handler
		return &geo.GeoRecord{
			State:   inf.Address.State,
			Country: inf.Address.Country,
			Source:  "address",
		}
	}

	if inf.Geo != nil {
		return inf.Geo
	}

	return nil
}

func (inf *Influencer) GetAvailableDeals(campaigns *common.Campaigns, budgetDb *bolt.DB, forcedCampaign, forcedDeal string, location *geo.GeoRecord, skipGeo bool, cfg *config.Config) []*common.Deal {
	// Iterates over all available deals in the system and matches them
	// with the given influencer
	// NOTE: The campaigns being passed only has campaigns with active
	// advertisers and agencies
	var (
		infDeals []*common.Deal
	)

	if inf.Banned {
		return infDeals
	}

	if location == nil && !skipGeo {
		location = inf.GetLatestGeo()
	}

	var store map[string]common.Campaign
	if forcedCampaign != "" {
		store = campaigns.GetCampaignAsStore(forcedCampaign)
	} else {
		store = campaigns.GetStore()
	}

	for _, cmp := range store {
		targetDeal := &common.Deal{}
		dealFound := false
		if !cmp.IsValid() {
			continue
		}

		for _, deal := range cmp.Deals {
			if deal.Assigned == 0 && deal.Completed == 0 && deal.InfluencerId == "" && !dealFound {
				if forcedDeal != "" && deal.Id != forcedDeal {
					continue
				}
				// Make a copy of the deal rather than assign the pointer
				*targetDeal = *deal
				dealFound = true
			}
		}

		if !dealFound {
			// This campaign has no active deals
			continue
		}

		// Filter Checks
		if len(cmp.Categories) > 0 {
			catFound := false
			for _, cat := range cmp.Categories {
				for _, infCat := range inf.Categories {
					if infCat == cat {
						catFound = true
						break
					}
				}
			}
			if !catFound {
				continue
			}
		}

		// If you already have a/have done deal for this campaign, screw off
		dealFound = false
		for _, d := range inf.ActiveDeals {
			if d.CampaignId == targetDeal.CampaignId {
				dealFound = true
			}
		}
		for _, d := range inf.CompletedDeals {
			if d.CampaignId == targetDeal.CampaignId {
				dealFound = true
			}
		}
		if dealFound {
			continue
		}

		// Match Campaign Geo Targeting with Influencer Geo //
		if !geo.IsGeoMatch(cmp.Geos, location) {
			continue
		}

		// Gender check
		if (inf.Male && !cmp.Male) || (inf.Female && !cmp.Female) {
			continue
		}

		// Whitelisting is done at the campaign level.. but
		// lets check the advertiser blacklist!
		_, ok := cmp.Blacklist[inf.Id]
		if ok {
			// We found this influencer in the blacklist!
			continue
		}

		// Whitelist check!
		if len(cmp.Whitelist) > 0 {
			_, ok = cmp.Whitelist[inf.EmailAddress]
			if !ok {
				// There was a whitelist and they're not in it!
				continue
			}
		}

		// Social Media Checks
		if cmp.Twitter && inf.Twitter != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Twitter)
		}

		if cmp.Facebook && inf.Facebook != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Facebook)
		}

		if cmp.Instagram && inf.Instagram != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Instagram)
		}

		if cmp.YouTube && inf.YouTube != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.YouTube)

		}

		// Add deal that has approved platform
		if len(targetDeal.Platforms) > 0 {
			targetDeal.Tags = cmp.Tags
			targetDeal.Mention = cmp.Mention
			targetDeal.Link = cmp.Link
			targetDeal.Task = cmp.Task
			if cmp.Perks != nil {
				targetDeal.Perk = &common.Perk{
					Name:     cmp.Perks.Name,
					Category: cmp.Perks.Category,
					Count:    1}
			}

			// Add some display attributes..
			// These will be saved permanently if they accept deal!
			targetDeal.CampaignName = cmp.Name
			targetDeal.CampaignImage = cmp.ImageURL
			targetDeal.Company = cmp.Company

			infDeals = append(infDeals, targetDeal)
		}
	}

	// Fill in available spendables now
	// This also makes sure that only campaigns with spendable are the
	// only ones eligible for deals
	filtered := make([]*common.Deal, 0, len(infDeals))
	for _, deal := range infDeals {
		store, err := budget.GetBudgetInfo(budgetDb, cfg, deal.CampaignId, "")
		if err == nil && store != nil && store.Spendable > 0 && store.Spent < store.Budget {
			deal.Spendable = misc.TruncateFloat(store.Spendable, 2)
			filtered = append(filtered, deal)
		}
	}

	return filtered
}

var ErrEmail = errors.New("Error sending email!")

func (inf *Influencer) Email(campaigns *common.Campaigns, budgetDb *bolt.DB, cfg *config.Config) (bool, error) {
	// Depending on the emails they've gotten already..
	// send them a follow up email

	// If we sent this influencer a deal within the last 7 days..
	// skip!
	if misc.WithinLast(inf.LastEmail, 24*7) {
		return false, nil
	}
	deals := inf.GetAvailableDeals(campaigns, budgetDb, "", "", nil, false, cfg)
	if len(deals) == 0 {
		return false, nil
	}

	ordered := OrderedDeals(deals)
	sort.Sort(ordered)
	if len(ordered) > 5 {
		ordered = ordered[0:5]
	}

	if !cfg.Sandbox {
		if cfg.ReplyMailClient() == nil {
			return false, ErrEmail
		}

		parts := strings.Split(inf.Name, " ")
		var firstName string
		if len(parts) > 0 {
			firstName = parts[0]
		}

		email := templates.InfluencerEmail.Render(map[string]interface{}{"Name": firstName, "deal": OrderedDeals(ordered)})
		if resp, err := cfg.ReplyMailClient().SendMessage(email, "Sway Brands requesting you!", inf.EmailAddress, inf.Name,
			[]string{}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			return false, ErrEmail
		}
	}
	return true, nil
}

func (inf *Influencer) EmailDeal(deal *common.Deal, cfg *config.Config) error {
	if !cfg.Sandbox {
		if cfg.ReplyMailClient() == nil {
			return ErrEmail
		}

		parts := strings.Split(inf.Name, " ")
		var firstName string
		if len(parts) > 0 {
			firstName = parts[0]
		}

		email := templates.InfluencerCmpEmail.Render(map[string]interface{}{"Name": firstName, "deal": []*common.Deal{deal}})
		resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("%s is requesting you!", deal.Company), inf.EmailAddress, inf.Name,
			[]string{""})
		if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			return ErrEmail
		}
	}
	return nil
}

func (inf *Influencer) DealHeadsUp(deal *common.Deal, cfg *config.Config) error {
	if !cfg.Sandbox {
		if cfg.ReplyMailClient() == nil {
			return ErrEmail
		}

		parts := strings.Split(inf.Name, " ")
		var firstName string
		if len(parts) > 0 {
			firstName = parts[0]
		}

		email := templates.InfluencerHeadsUpEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
		resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("You have 4 days to complete the deal for %s!", deal.Company), inf.EmailAddress, inf.Name,
			[]string{""})
		if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			return ErrEmail
		}
	}
	return nil
}

func (inf *Influencer) DealTimeout(deal *common.Deal, cfg *config.Config) error {
	if !cfg.Sandbox {
		if cfg.ReplyMailClient() == nil {
			return ErrEmail
		}

		parts := strings.Split(inf.Name, " ")
		var firstName string
		if len(parts) > 0 {
			firstName = parts[0]
		}

		email := templates.InfluencerTimeoutEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
		resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your deal for %s has expired!", deal.Company), inf.EmailAddress, inf.Name,
			[]string{""})
		if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			return ErrEmail
		}
	}
	return nil
}

func (inf *Influencer) IsViral(deal *common.Deal, stats *common.Stats) bool {
	if deal.Tweet != nil && inf.Twitter != nil {
		return isViral(stats.Likes, inf.Twitter.AvgLikes)
	} else if deal.Facebook != nil && inf.Facebook != nil {
		return isViral(stats.Likes, inf.Facebook.AvgLikes)
	} else if deal.Instagram != nil && inf.Instagram != nil {
		return isViral(stats.Likes, inf.Instagram.AvgLikes)
	} else if deal.YouTube != nil && inf.YouTube != nil {
		return isViral(stats.Likes, inf.YouTube.AvgLikes)
	}
	return false
}

func isViral(likes int32, avg float64) bool {
	// If this post made >200% of average likes
	// it's viral
	return likes > 0 && float64(likes)/avg > 2
}
