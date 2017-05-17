package influencer

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/imagga"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

const (
	TimeoutDays = 40
)

var (
	ErrAgency     = errors.New("No talent agency defined! Please contact engage@swayops.com")
	ErrInviteCode = errors.New("Invite code passed in not found. Please verify URL with the talent agency or contact engage@swayops.com")
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

	BrandSafe string `json:"brandSafe,omitempty"`

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

	BrandSafe string `json:"brandSafe,omitempty"`

	Address *lob.AddressLoad `json:"address,omitempty"`

	// Agency this influencer belongs to
	AgencyId string `json:"agencyId,omitempty"`

	// References to the social media accounts this influencer owns
	Facebook         *facebook.Facebook   `json:"facebook,omitempty"`
	Instagram        *instagram.Instagram `json:"instagram,omitempty"`
	Twitter          *twitter.Twitter     `json:"twitter,omitempty"`
	YouTube          *youtube.YouTube     `json:"youtube,omitempty"`
	LastSocialUpdate int32                `json:"lastUpdate,omitempty"`

	// Used for the API exclusively (set in the influencer's Clean method)
	// Used for the getAllInfluencers* methods to provide concise structs
	FbUsername      string `json:"fbUsername,omitempty"`
	InstaUsername   string `json:"instaUsername,omitempty"`
	TwitterUsername string `json:"twitterUsername,omitempty"`
	YTUsername      string `json:"youtubeUsername,omitempty"`

	// Set and created by the IP
	Geo *geo.GeoRecord `json:"geo,omitempty"`
	// For privilidged users who skip the geo check for certain campaigns!
	SkipGeo bool `json:"skipGeo,omitempty"`
	// List of campaigns ofr which to skip geo
	GeoSkips []string `json:"geoSkips,omitempty"`

	Male   bool `json:"male,omitempty"`
	Female bool `json:"female,omitempty"`

	// Influencer inputted category they belong to
	Categories []string `json:"categories,omitempty"`
	// Extracted from Imagga
	Keywords []string `json:"keywords,omitempty"`

	Strikes []*Strike `json:"strikes,omitempty"`

	// Active accepted deals by the influencer that have not yet been completed
	ActiveDeals []*common.Deal `json:"activeDeals,omitempty"`
	// Completed and approved deals by the influencer
	CompletedDeals []*common.Deal `json:"completedDeals,omitempty"`

	// Array of campaigns this influencer has cancelled for
	Cancellations []string `json:"cancel,omitempty"`
	// Array of campaigns this influencer has timed out for
	Timeouts []string `json:"tos,omitempty"`

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

	// Set only in getInfluencersByAgency to save us a stats endpoint hit (tmp fields)
	AgencySpend     float64 `json:"agSpend,omitempty"`
	InfluencerSpend float64 `json:"infSpend,omitempty"`
	Followers       int64   `json:"followers,omitempty"`

	// Stores whether the influencer has already been notified once about
	// their profile going from public to private
	PrivateNotify int32 `json:"private,omitempty"`
}

type Strike struct {
	CampaignID string `json:"campaignID,omitempty"`
	Reasons    string `json:"reasons,omitempty"`
	TS         int64  `json:"ts,omitempty"`
}

func New(id, name, twitterId, instaId, fbId, ytId string, m, f bool, inviteCode, defAgencyID, email, ip, brandSafe string, cats []string, address *lob.AddressLoad, created int32, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id:           id,
		Name:         name,
		Male:         m,
		Female:       f,
		Categories:   cats,
		DealPing:     true, // Deal ping is true by default!
		EmailAddress: misc.TrimEmail(email),
		CreatedAt:    created,
		BrandSafe:    brandSafe,
	}

	if address != nil && address.AddressOne != "" {
		addr, err := lob.VerifyAddress(address, cfg)
		if err != nil {
			return nil, err
		}
		inf.Address = addr
	}

	var agencyId string
	if inviteCode == "" {
		// No invite code passed in
		agencyId = defAgencyID
	} else {
		// There was an invite code passed in
		agencyId = common.GetIDFromInvite(inviteCode)
		if agencyId == "" {
			return nil, ErrInviteCode
		}
	}

	inf.AgencyId = agencyId

	err := inf.NewInsta(instaId, []string{}, cfg)
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

	err = inf.NewYouTube(ytId, []string{}, cfg)
	if err != nil {
		return inf, err
	}

	if ip != "" {
		inf.Geo = geo.GetGeoFromIP(cfg.GeoDB, ip)
	}

	// Assign automated keywords
	keywords, err := imagga.GetKeywords(inf.GetImages(cfg), cfg.Sandbox)
	if err == nil {
		inf.Keywords = keywords
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

func (inf *Influencer) NewInsta(id string, keywords []string, cfg *config.Config) error {
	if len(id) > 0 {
		insta, err := instagram.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Instagram = insta

		if len(keywords) > 0 {
			// Transferred from scrap
			inf.Keywords = keywords
		} else {
			keywords, err := imagga.GetKeywords(inf.GetImages(cfg), cfg.Sandbox)
			if err == nil {
				inf.Keywords = keywords
			}
		}
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

func (inf *Influencer) NewYouTube(id string, keywords []string, cfg *config.Config) error {
	if len(id) > 0 {
		yt, err := youtube.New(id, cfg)
		if err != nil {
			return err
		}
		inf.YouTube = yt

		if len(keywords) > 0 {
			// Transferred from scrap
			inf.Keywords = keywords
		} else {
			keywords, err := imagga.GetKeywords(inf.GetImages(cfg), cfg.Sandbox)
			if err == nil {
				inf.Keywords = keywords
			}

		}
	}
	return nil
}

func (inf *Influencer) UpdateAll(cfg *config.Config) (private bool, err error) {
	if inf.IsBanned() {
		return false, nil
	}

	inf.setSwayRep()

	// Used by sway engine to periodically update influencer data

	// Always allow updates if they have an active deal
	// i.e. skip this if statement

	// If the profile picture has become inactive lets update as well!
	if len(inf.ActiveDeals) == 0 && inf.IsProfilePictureActive() && (inf.Instagram == nil || inf.Instagram.Bio != "") {
		// If you've been updated in the last 7-11 days and
		// have no active deals.. screw you!
		// NO SOUP FOR YOU!
		// NOTE: The random integer is so that we don't update
		// a large group of influencers at the same time (they become
		// in sync due to the same len check).
		if misc.WithinLast(inf.LastSocialUpdate, 24*misc.Random(7, 11)) {
			// Clear out posts from platforms since they're of no use
			// and are just taking up storage
			if inf.Facebook != nil {
				inf.Facebook.LatestPosts = nil
			}
			if inf.Instagram != nil {
				inf.Instagram.LatestPosts = nil
			}
			if inf.Twitter != nil {
				inf.Twitter.LatestTweets = nil
			}
			if inf.YouTube != nil {
				inf.YouTube.LatestPosts = nil
			}
			return false, nil
		}
	}

	// We only traverse over latest posts when
	// looking for a satisfied deal.. so if we're just getting
	// stats for influencer who have no active deals..
	// we don't need to save all those massive post lists..
	// hence why we have the savePosts bool!
	savePosts := len(inf.ActiveDeals) > 0

	if inf.Instagram != nil {
		if err = inf.Instagram.UpdateData(cfg, savePosts); err != nil {
			if inf.Instagram.Followers > 0 && instagram.Status(cfg) {
				// This means we've gotten data on this user before.. but can't
				// now!
				// NOTE: Also checking if key is active so we don't email
				// influencers just because our key is down
				private = true
			}
			return private, err
		}
	}

	if inf.Twitter != nil {
		if err = inf.Twitter.UpdateData(cfg, savePosts); err != nil {
			if inf.Twitter.Followers > 0 && twitter.Status(cfg) {
				// This means we've gotten data on this user before.. but can't
				// now!
				// NOTE: Also checking if key is active so we don't email
				// influencers just because our key is down
				private = true
			}
			return private, err
		}
	}

	if inf.YouTube != nil {
		if err = inf.YouTube.UpdateData(cfg, savePosts); err != nil {
			return private, err
		}
	}

	if inf.Facebook != nil {
		if err = inf.Facebook.UpdateData(cfg, savePosts); err != nil {
			return private, err
		}
	}

	inf.LastSocialUpdate = int32(time.Now().Unix())

	return private, nil
}

func (inf *Influencer) ForceUpdate(cfg *config.Config) (err error) {
	if inf.Banned {
		return nil
	}

	if inf.Facebook != nil {
		if err = inf.Facebook.UpdateData(cfg, true); err != nil {
			return err
		}
	}
	if inf.Instagram != nil {
		if err = inf.Instagram.UpdateData(cfg, true); err != nil {
			return err
		}
	}
	if inf.Twitter != nil {
		if err = inf.Twitter.UpdateData(cfg, true); err != nil {
			return err
		}
	}
	if inf.YouTube != nil {
		if err = inf.YouTube.UpdateData(cfg, true); err != nil {
			return err
		}
	}

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
			// Update deals that aren't active anymore once in a blue moon
			// just to save some requests
			// if misc.Random(0, 100) > 10 {
			// 90% of the time bail!
			continue
			// }
		}

		if deal.Tweet != nil {
			if ban, err = deal.Tweet.UpdateData(cfg); err != nil {
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

		// Lets update bonus deals too!
		if deal.Bonus != nil {
			for _, tw := range deal.Bonus.Tweet {
				if _, err = tw.UpdateData(cfg); err != nil {
					return err
				}
			}

			for _, post := range deal.Bonus.Facebook {
				if err = post.UpdateData(cfg); err != nil {
					return err
				}
			}

			for _, post := range deal.Bonus.Instagram {
				if _, err = post.UpdateData(cfg); err != nil {
					return err
				}
			}

			for _, post := range deal.Bonus.YouTube {
				if err = post.UpdateData(cfg); err != nil {
					return err
				}
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

func (inf *Influencer) GetImages(cfg *config.Config) []string {
	var urls []string
	if inf.Instagram != nil {
		if len(inf.Instagram.Images) > 0 {
			// This user has saved images
			urls = append(urls, inf.Instagram.Images...)
		} else {
			// This person has an insta but no images.. that
			// means they probably have not had their social media
			// info updated since we started storing images
			// LETS ACCOUNT FOR THAT!
			savePosts := len(inf.ActiveDeals) > 0
			inf.Instagram.UpdateData(cfg, savePosts)
			urls = append(urls, inf.Instagram.Images...)
		}
	}

	if inf.YouTube != nil {
		if len(inf.YouTube.Images) > 0 {
			urls = append(urls, inf.YouTube.Images...)
		} else {
			// This person has an insta but no images.. that
			// means they probably have not had their social media
			// info updated since we started storing images
			// LETS ACCOUNT FOR THAT!
			savePosts := len(inf.ActiveDeals) > 0
			inf.YouTube.UpdateData(cfg, savePosts)
			urls = append(urls, inf.YouTube.Images...)
		}
	}

	return urls
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

func (inf *Influencer) GetProfilePicture() string {
	if inf.Facebook != nil && inf.Facebook.ProfilePicture != "" {
		return inf.Facebook.ProfilePicture
	}
	if inf.Instagram != nil && inf.Instagram.ProfilePicture != "" {
		return inf.Instagram.ProfilePicture
	}
	if inf.Twitter != nil && inf.Twitter.ProfilePicture != "" {
		return inf.Twitter.ProfilePicture
	}
	if inf.YouTube != nil && inf.YouTube.ProfilePicture != "" {
		return inf.YouTube.ProfilePicture
	}
	return ""
}

func (inf *Influencer) IsProfilePictureActive() bool {
	// Checks to see if any of the profile pictures are returning
	// a 404

	// We can skip this check if the influencer has no completed
	// deals (since we only show profile picture and need to check
	// it's validity when it shows up in an advertiser conten feed)
	if len(inf.CompletedDeals) == 0 {
		return true
	}

	if inf.Facebook != nil && inf.Facebook.ProfilePicture != "" {
		if misc.Ping(inf.Facebook.ProfilePicture) != nil {
			return false
		}
	}
	if inf.Instagram != nil && inf.Instagram.ProfilePicture != "" {
		if misc.Ping(inf.Instagram.ProfilePicture) != nil {
			return false
		}
	}
	if inf.Twitter != nil && inf.Twitter.ProfilePicture != "" {
		if misc.Ping(inf.Twitter.ProfilePicture) != nil {
			return false
		}
	}
	if inf.YouTube != nil && inf.YouTube.ProfilePicture != "" {
		if misc.Ping(inf.YouTube.ProfilePicture) != nil {
			return false
		}
	}

	return true
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

	rep = degradeRep(len(inf.Timeouts), rep)
	rep = degradeRep(len(inf.Cancellations), rep)

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

func (inf *Influencer) IsAmerican() bool {
	if inf.Address == nil {
		return false
	}

	cy := strings.ToLower(inf.Address.Country)
	if cy == "us" || cy == "usa" {
		return true
	}

	return false
}

func (inf *Influencer) GetAvailableDeals(campaigns *common.Campaigns, audiences *common.Audiences, db *bolt.DB, forcedCampaign, forcedDeal string, location *geo.GeoRecord, query bool, cfg *config.Config) []*common.Deal {
	// Iterates over all available deals in the system and matches them
	// with the given influencer
	// NOTE: The campaigns being passed only has campaigns with active
	// advertisers and agencies
	var (
		infDeals []*common.Deal
	)

	if inf.IsBanned() {
		return infDeals
	}

	// If the influencer hasn't been updated in the last 25 days.. ignore em!
	if !misc.WithinLast(inf.LastSocialUpdate, 24*25) {
		return infDeals
	}

	if !inf.Audited() && !cfg.Sandbox {
		// If the user has no categories or gender.. this means
		// the assign game hasn't gotten to them yet
		// NOTE: Allowing sandbox because tests don't do the
		// assign game
		return infDeals
	}

	if location == nil {
		location = inf.GetLatestGeo()
	}

	var store map[string]common.Campaign
	if forcedCampaign != "" {
		store = campaigns.GetCampaignAsStore(forcedCampaign)
	} else {
		store = campaigns.GetStore()
	}

	// Used purely for debugging.. may want to do something with it
	// eventually
	rejections := make(map[string]string)
	for _, cmp := range store {
		// Store only contains campaigns with active advertisers with proper
		// subscriptions!

		// Check for advertiser eligibility!
		if !subscriptions.CanInfluencerRun(cmp.AgencyId, cmp.Plan, inf.GetFollowers()) {
			rejections[cmp.Id] = "INVALID_SUBSCRIPTION"
			continue
		}

		targetDeal := &common.Deal{}
		dealFound := false
		if !cmp.IsValid() {
			rejections[cmp.Id] = "INVALID"
			continue
		}

		for _, deal := range cmp.Deals {
			// Query is only passed in from getDeal so an influencer can view deals they're
			// currently assigned to
			if (query || (deal.IsAvailable())) && !dealFound {
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
			rejections[cmp.Id] = "NO_ACTIVE_DEALS"
			continue
		}

		// Filter Checks
		if len(cmp.Categories) > 0 || len(cmp.Audiences) > 0 {
			catFound := false
		L1:
			for _, cat := range cmp.Categories {
				for _, infCat := range inf.Categories {
					if infCat == cat {
						catFound = true
						break L1
					}
				}
			}

			if !catFound {
				for _, targetAudience := range cmp.Audiences {
					if audiences.IsAllowed(targetAudience, inf.EmailAddress) {
						catFound = true
						break
					}
				}
			}

			if !catFound {
				rejections[cmp.Id] = "CAT_NOT_FOUND"
				continue
			}
		}

		if len(cmp.Keywords) > 0 {
			kwFound := false
		L2:
			for _, kw := range cmp.Keywords {
				for _, infKw := range inf.Keywords {
					if kw == infKw {
						kwFound = true
						break L2
					}
				}

				if inf.Instagram != nil && inf.Instagram.Bio != "" {
					if common.IsExactMatch(inf.Instagram.Bio, kw) {
						kwFound = true
						break L2
					}
				}
			}
			if !kwFound {
				rejections[cmp.Id] = "KW_NOT_FOUND"
				continue
			}
		}

		// If you already have a/have done deal for this campaign, screw off
		dealFound = false
		if !query {
			// With the query flag beign used by getDeal,
			// we may be looking for details on an assigned deal
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

			for _, cid := range inf.Timeouts {
				if cid == targetDeal.CampaignId {
					dealFound = true
				}
			}

			for _, cid := range inf.Cancellations {
				if cid == targetDeal.CampaignId {
					dealFound = true
				}
			}
		}

		if dealFound {
			rejections[cmp.Id] = "DEAL_FOUND"
			continue
		}

		// Match Campaign Geo Targeting with Influencer Geo //
		if !inf.SkipGeo && !misc.Contains(inf.GeoSkips, cmp.Id) && !geo.IsGeoMatch(cmp.Geos, location) && !query {
			rejections[cmp.Id] = "GEO_MATCH"
			continue
		}

		// Gender check
		if !query {
			// If you are both (i.e. not applicable) and the campaign wants just one..
			// skip!
			if inf.Male && inf.Female && (!cmp.Male || !cmp.Female) {
				rejections[cmp.Id] = "GENDER_UNI"
				continue
			}

			if !cmp.Male && cmp.Female && !inf.Female {
				// Only want females
				rejections[cmp.Id] = "GENDER_F"
				continue
			} else if cmp.Male && !cmp.Female && !inf.Male {
				// Only want males
				rejections[cmp.Id] = "GENDER_M"
				continue
			} else if !cmp.Male && !cmp.Female {
				rejections[cmp.Id] = "GENDER"
				continue
			}
		}

		// Whitelisting is done at the campaign level.. but
		// lets check the advertiser blacklist!
		_, ok := cmp.Blacklist[inf.Id]
		if ok {
			// We found this influencer in the blacklist!
			rejections[cmp.Id] = "CMP_BLACKLIST"
			continue
		}

		// Whitelist check!
		if len(cmp.Whitelist) > 0 {
			_, ok = cmp.Whitelist[inf.EmailAddress]
			if !ok {
				// There was a whitelist and they're not in it!
				rejections[cmp.Id] = "CMP_WHITELIST"
				continue
			}
		}

		// Brand safety check
		// If the campaign just wants brand safe and the influencer isn't brand safe..
		if cmp.BrandSafe && !query {
			if inf.BrandSafe != "t" || inf.GetFollowers() < 10000 {
				rejections[cmp.Id] = "BRAND_SAFETY"
				continue
			}
		}

		// Fill in and check available spendable
		budgetStore, err := budget.GetCampaignStoreFromDb(db, cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || budgetStore == nil {
			log.Println("Error when opening store", err)
			continue
		}

		if budgetStore.IsClosed(&cmp) {
			if !query {
				// Influencer may query for their assigned deal.. but we don't want to
				// hide the deal if there's no spendable.. we just want to tell them that
				// there's 0 spendable
				rejections[cmp.Id] = "BUDGET"
				continue
			}
		}

		maxYield := GetMaxYield(&cmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)

		// Subtract default margins to give influencers an accurate likely earning value
		// NOTE: Mimics logic in depleteBudget functionality of sway engine
		_, _, _, infPayout := budget.GetMargins(maxYield, -1, -1, -1)

		// Note: For query lookups, the most up-to date value is shown to influencers.
		// However, we only save the LikelyEarnings value that is saved when the influencer
		// hit "/assignDeal"
		if budgetStore != nil && !cmp.IsProductBasedBudget() {
			// Generate likely earnings for the influencer
			targetDeal.LikelyEarnings = misc.TruncateFloat(infPayout, 2)

			// Generate pending spend (based on deals that are assigned
			// and how much they should spend)
			pendingSpend, _ := cmp.GetPendingDetails()

			// Subtract pending spend remaining spendable
			availSpend := budgetStore.Spendable - pendingSpend

			// If we are expected to spend all of spendable
			// given the people who are in the deal.. lets bail (unless
			// it's a query.. in which case we shoudl always show the deal)
			if availSpend <= 10 && !cfg.Sandbox && !query {
				rejections[cmp.Id] = "AVAIL_SPEND"
				continue
			}

			if targetDeal.LikelyEarnings > availSpend && !query {
				if availSpend/targetDeal.LikelyEarnings < 0.2 && !cfg.Sandbox {
					// If likely earnings are more than available spend.. and
					// available spend is <20% of what you're supposed to be making..
					// lets not offer the deal out of fear of insulting the influencer
					rejections[cmp.Id] = "INSULT_SPEND"
					continue
				}

				// This is to ensure we don't have a situation where we display
				// likely earnings as being over the "Total" value when the influencer
				// queries for assigned deals
				targetDeal.LikelyEarnings = availSpend * 0.7
			}

			targetDeal.Spendable = misc.TruncateFloat(budgetStore.Spendable, 2)
			if !query && !cfg.Sandbox && len(cmp.Whitelist) == 0 && cmp.Perks != nil && cmp.Perks.GetType() == "Product" {
				// NOTE: Skip this for whitelisted campaigns and non-product perk campaigns!

				// OPTIMIZATION: Goal is to distribute products and funds evenly
				// given what the campaign's product perk count is and how
				// many funds we have left
				min, max := cmp.GetTargetYield(targetDeal.Spendable)
				if maxYield < min || maxYield > max || maxYield == 0 {
					rejections[cmp.Id] = "MAX_YIELD"
					continue
				}
			}
		}

		if !query {
			// If it's a perk campaign and there's no perks to give.. BAIL!
			if cmp.Perks != nil && cmp.Perks.Count == 0 {
				rejections[cmp.Id] = "NO_PERKS"
				continue
			}
		}

		// Social Media Checks
		if cmp.YouTube && inf.YouTube != nil {
			if !common.IsInList(targetDeal.Platforms, platform.YouTube) {
				targetDeal.Platforms = append(targetDeal.Platforms, platform.YouTube)
			}
		}

		if cmp.Instagram && inf.Instagram != nil {
			if !common.IsInList(targetDeal.Platforms, platform.Instagram) {
				targetDeal.Platforms = append(targetDeal.Platforms, platform.Instagram)
			}
		}

		if cmp.Twitter && inf.Twitter != nil {
			if !common.IsInList(targetDeal.Platforms, platform.Twitter) {
				targetDeal.Platforms = append(targetDeal.Platforms, platform.Twitter)
			}
		}

		if cmp.Facebook && inf.Facebook != nil {
			if !common.IsInList(targetDeal.Platforms, platform.Facebook) {
				targetDeal.Platforms = append(targetDeal.Platforms, platform.Facebook)
			}
		}

		// Add deal that has approved platform
		if len(targetDeal.Platforms) > 0 {
			targetDeal.Tags = cmp.Tags
			targetDeal.Mention = cmp.Mention
			targetDeal.Task = cmp.Task
			if cmp.Perks != nil && inf.Id != "385" {
				var code string
				if targetDeal.Perk != nil {
					code = targetDeal.Perk.Code
				}
				targetDeal.Perk = &common.Perk{
					Name:         cmp.Perks.Name,
					Instructions: cmp.Perks.Instructions,
					Category:     cmp.Perks.GetType(),
					Code:         code,
					Count:        1}
			}

			if targetDeal.Link == "" {
				// getDeal queries for an active deal so it already has
				// a link set!
				targetDeal.Link = cmp.Link
			}

			// Add some display attributes..
			// These will be saved permanently if they accept deal!
			targetDeal.CampaignName = cmp.Name
			targetDeal.CampaignImage = cmp.ImageURL
			targetDeal.Company = cmp.Company

			infDeals = append(infDeals, targetDeal)
		}
	}

	return infDeals
}

var (
	ErrEmail   = errors.New("Error sending email!")
	ErrTimeout = errors.New("Last email too early")
)

func (inf *Influencer) Email(campaigns *common.Campaigns, audiences *common.Audiences, budgetDb *bolt.DB, cfg *config.Config) (bool, error) {
	// Depending on the emails they've gotten already..
	// send them a follow up email

	// If we sent this influencer a deal within the last 7 days..
	// skip!
	if misc.WithinLast(inf.LastEmail, 24*7) {
		return false, nil
	}

	deals := inf.GetAvailableDeals(campaigns, audiences, budgetDb, "", "", nil, false, cfg)
	if len(deals) == 0 {
		return false, nil
	}

	ordered := OrderedDeals(deals)
	sort.Sort(ordered)
	if len(ordered) > 5 {
		ordered = ordered[0:5]
	}

	if cfg.Sandbox {
		return true, nil
	}

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

	var cids []string
	for _, d := range deals {
		cids = append(cids, d.CampaignId)
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal newsletter",
		"id":   inf.Id,
		"cids": cids,
	}); err != nil {
		log.Println("Failed to log newsletter!", inf.Id, cids)
	}

	return true, nil
}

func (inf *Influencer) EmailDeal(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	// If we sent this influencer a deal within the last 7 days..
	// skip!
	if misc.WithinLast(inf.LastEmail, 24*7) {
		return ErrTimeout
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

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal email",
		"id":   inf.Id,
		"cids": []string{deal.CampaignId},
	}); err != nil {
		log.Println("Failed to log email!", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) DealHeadsUp(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.InfluencerHeadsUpEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("You have 7 days to complete the deal for %s!", deal.Company), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal heads up",
		"id":   inf.Id,
		"cids": []string{deal.CampaignId},
	}); err != nil {
		log.Println("Failed to log deal heads up!", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) DealTimeout(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

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

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal timeout",
		"id":   inf.Id,
		"cids": []string{deal.CampaignId},
	}); err != nil {
		log.Println("Failed to log deal timeout!", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) DealCompletion(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.DealCompletionEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Congratulations! Your deal for %s has been approved!", deal.Company), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal completion",
		"id":   inf.Id,
		"cids": []string{deal.CampaignId},
	}); err != nil {
		log.Println("Failed to log deal completion", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) DealPickedUp(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.PickedUpEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your post for %s has been picked up!", deal.Company), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	// Tmp Check
	cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your post for %s has been picked up!", deal.Company), "shahzil@swayops.com", inf.Name,
		[]string{""})

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal picked up",
		"id":   inf.Id,
		"cids": []string{deal.CampaignId},
	}); err != nil {
		log.Println("Failed to log deal picked up!", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) DealUpdate(cmp *common.Campaign, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.CampaignStatusEmail.Render(map[string]interface{}{"Name": firstName, "Company": cmp.Company})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your deal for %s is no longer available!", cmp.Company), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal update",
		"id":   inf.Id,
		"cids": []string{cmp.Id},
	}); err != nil {
		log.Println("Failed to log deal udpate!", inf.Id, cmp.Id)
	}

	return nil
}

func (inf *Influencer) DealInstructions(cmp *common.Campaign, deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	perks := "N/A"
	instructions := "N/A"
	coupon := "N/A"
	hasPerks := false
	hasCoupon := false

	if cmp.Perks != nil {
		hasPerks = true
		perks = cmp.Perks.Name
		if cmp.Perks.Instructions != "" {
			instructions = cmp.Perks.Instructions
		}
		if deal.Perk.Code != "" {
			coupon = deal.Perk.Code
			hasCoupon = true
		}
	}

	tags := []string{"#ad"}
	for _, cmpTag := range deal.Tags {
		tags = append(tags, "#"+cmpTag)
	}

	email := templates.DealInstructionsEmail.Render(map[string]interface{}{"Networks": strings.Join(deal.Platforms, ", "), "Link": deal.ShortenedLink, "Tags": strings.Join(tags, ", "), "Mentions": deal.Mention, "Name": firstName, "Campaign": cmp.Name, "Task": cmp.Task, "Instructions": instructions, "Perks": perks, "Image": cmp.ImageURL, "CouponCode": coupon, "HasPerks": hasPerks, "HasCoupon": hasCoupon, "Timeout": TimeoutDays})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Instructions for completing your Sway deal for %s", cmp.Name), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	resp, err = cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Instructions for completing your Sway deal for %s", cmp.Name), "shahzil@swayops.com", inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":  "deal instructions",
		"id":   inf.Id,
		"cids": []string{cmp.Id},
	}); err != nil {
		log.Println("Failed to log deal instructions!", inf.Id, cmp.Id)
	}

	return nil
}

func (inf *Influencer) DealRejection(reason, postURL string, deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox || reason == "" {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.DealRejectionEmail.Render(map[string]interface{}{"Name": firstName, "reason": reason, "url": postURL})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your post for %s is missing a required item!", deal.Company), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag":    "deal rejection",
		"id":     inf.Id,
		"cids":   []string{deal.CampaignId},
		"reason": reason,
		"url":    postURL,
	}); err != nil {
		log.Println("Failed to log deal rejection!", inf.Id, deal.CampaignId)
	}

	return nil
}

func (inf *Influencer) PrivateEmail(cfg *config.Config) error {
	// Turn this off for now
	return nil

	if cfg.Sandbox {
		return nil
	}

	if inf.PrivateNotify > 0 {
		// They've already been told about this!
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.PrivateEmail.Render(map[string]interface{}{"Name": firstName})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your profile has become inaccessible!"), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag": "private email",
		"id":  inf.Id,
	}); err != nil {
		log.Println("Failed to log private email!", inf.Id)
	}

	return nil
}

func (inf *Influencer) CheckEmail(check *lob.Check, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	var delivery string
	if inf.IsAmerican() {
		delivery = "4 - 6"
	} else {
		delivery = "9 - 13"
	}

	strPayout := strconv.FormatFloat(check.Payout, 'f', 2, 64)

	email := templates.CheckEmail.Render(map[string]interface{}{"Name": firstName, "Delivery": delivery, "Payout": strPayout})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your check has been mailed!"), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag": "check",
		"id":  inf.Id,
	}); err != nil {
		log.Println("Failed to log check email!", inf.Id)
	}

	return nil
}

func (inf *Influencer) PerkNotify(deal *common.Deal, cfg *config.Config) error {
	if cfg.Sandbox {
		return nil
	}

	if cfg.ReplyMailClient() == nil {
		return ErrEmail
	}

	parts := strings.Split(inf.Name, " ")
	var firstName string
	if len(parts) > 0 {
		firstName = parts[0]
	}

	email := templates.PerkMailEmail.Render(map[string]interface{}{"Name": firstName, "Company": deal.Company})
	resp, err := cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your perk has been mailed!"), inf.EmailAddress, inf.Name,
		[]string{""})
	if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		return ErrEmail
	}

	if err := cfg.Loggers.Log("email", map[string]interface{}{
		"tag": "perk notify",
		"id":  inf.Id,
	}); err != nil {
		log.Println("Failed to log perk notify email!", inf.Id)
	}

	return nil
}

func (inf *Influencer) Audited() bool {
	return len(inf.Categories) > 0 && (inf.Male || inf.Female) && inf.BrandSafe != ""
}

func (inf *Influencer) IsBanned() bool {
	return inf.Banned || len(inf.Strikes) >= 3
}

func (inf *Influencer) IsViral(tweet *twitter.Tweet, instaPost *instagram.Post, fbPost *facebook.Post, ytPost *youtube.Post) bool {
	if tweet != nil && inf.Twitter != nil {
		return isViral(tweet.Favorites, inf.Twitter.AvgLikes) ||
			isViral(tweet.Retweets, inf.Twitter.AvgRetweets)
	} else if fbPost != nil && inf.Facebook != nil {
		return isViral(fbPost.Likes, inf.Facebook.AvgLikes) ||
			isViral(fbPost.Comments, inf.Facebook.AvgComments) ||
			isViral(fbPost.Shares, inf.Facebook.AvgShares)

	} else if instaPost != nil && inf.Instagram != nil {
		return isViral(instaPost.Likes, inf.Instagram.AvgLikes) ||
			isViral(instaPost.Comments, inf.Instagram.AvgComments)
	} else if ytPost != nil && inf.YouTube != nil {
		return isViral(ytPost.Likes, inf.YouTube.AvgLikes) ||
			isViral(ytPost.Comments, inf.YouTube.AvgComments) ||
			isViral(ytPost.Dislikes, inf.YouTube.AvgDislikes) ||
			isViral(ytPost.Views, inf.YouTube.AvgViews)
	}
	return false
}

func isViral(val, avg float64) bool {
	// If this post made >30% higher than average likes
	// it's viral
	return val > 0 && val/avg > 1.3
}
