package server

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/pdf"
)

const (
	UPDATE         = 30 * time.Minute
	THIRTY_MINUTES = 30 * 60 // 30 mins in secs
)

type Forecasts struct {
	m map[string]ForecastValue
	l sync.RWMutex
}

type ForecastValue struct {
	Users     []ForecastUser
	Reach     int64
	Timestamp int64
}

func NewForecasts() *Forecasts {
	c := &Forecasts{m: make(map[string]ForecastValue)}
	c.clean()
	return c
}

func (s *Forecasts) Set(users []ForecastUser, reach int64) (token string) {
	token = misc.PseudoUUID()

	s.l.Lock()
	s.m[token] = ForecastValue{users, reach, time.Now().Unix()}
	s.l.Unlock()

	return
}

func (s *Forecasts) Get(token string, start, results int) ([]ForecastUser, int, int64, bool) {
	s.l.RLock()
	value, ok := s.m[token]
	s.l.RUnlock()

	return index(value.Users, start, results), len(value.Users), value.Reach, ok
}

func (s *Forecasts) Delete(token string) {
	s.l.Lock()
	delete(s.m, token)
	s.l.Unlock()
}

func (s *Forecasts) clean() {
	// Every 30 minutes clear out any values that are older than 30 minutes
	ticker := time.NewTicker(UPDATE)
	go func() {
		for range ticker.C {
			now := time.Now().Unix()
			s.l.Lock()
			for key, ts := range s.m {
				if now > ts.Timestamp+THIRTY_MINUTES {
					delete(s.m, key)
				}
			}
			s.l.Unlock()
		}
	}()
}

type ForecastUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`

	ProfilePicture  string `json:"profilePicture"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	Followers       int64  `json:"followers"`
	StringFollowers string `json:"stringFollowers"`
	AvgEngs         int64  `json:"avgEngs"`

	// Float representation of max yield
	Rate float64 `json:"rate"`
	// Used for display purposes in reports
	MaxYield   string `json:"maxYield"`
	Geo        string `json:"geo"`
	Gender     string `json:"gender"`
	Categories string `json:"categories"`

	HasTwitter      bool   `json:"hasTwitter"`
	TwitterUsername string `json:"twUsername"`

	HasInsta      bool   `json:"hasInsta"`
	InstaUsername string `json:"instaUsername"`

	HasYoutube      bool   `json:"hasYoutube"`
	YoutubeUsername string `json:"ytUsername"`

	HasFacebook      bool   `json:"hasFacebook"`
	FacebookUsername string `json:"fbUsername"`
}

func (user *ForecastUser) IsProfilePictureActive() bool {
	if user.ProfilePicture == "" || (user.ProfilePicture != "" && misc.Ping(user.ProfilePicture) != nil) {
		return false
	}
	return true
}

func getForecastForCmp(s *Server, cmp common.Campaign, sortBy, incomingToken string, indexStart, maxResults int) (influencers []ForecastUser, total int, reach int64, token string) {
	if incomingToken != "" {
		// If a token was passed and we have a value for it.. lets return that!
		if infs, total, r, ok := s.Forecasts.Get(incomingToken, indexStart, maxResults); ok {
			return infs, total, r, incomingToken
		}
	}
	// Some easy bail outs
	if !cmp.Instagram && !cmp.Twitter && !cmp.YouTube && !cmp.Facebook {
		return
	}

	if !cmp.Male && !cmp.Female {
		return
	}

	// Lite version of the original GetAVailableDeals just for forecasting
	// spendable := budget.GetProratedBudget(cmp.Budget)

	// Calculate max deals for this campaign
	// var maxDeals int
	// if cmp.Perks != nil {
	// 	maxDeals = cmp.Perks.Count
	// } else if len(cmp.Whitelist) > 0 {
	// 	maxDeals = len(cmp.Whitelist)
	// } else {
	// 	if cmp.Goal > 0 {
	// 		if cmp.Goal > spendable {
	// 			maxDeals = 1
	// 		} else {
	// 			maxDeals = int(spendable / cmp.Goal)
	// 		}
	// 	} else {
	// 		// Default goal of spendable divided by 5 (so 5 deals per campaign)
	// 		maxDeals = int(spendable / 5)
	// 	}
	// }

	// target := spendable / float64(maxDeals)
	// margin := 0.3 * target

	// Pre calculate target yield
	// min, max := target-margin, target+margin
	infs := s.auth.Influencers.GetAll()
	for _, inf := range infs {
		if inf.IsBanned() {
			continue
		}

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

			// Audience check!
			if !catFound {
				for _, targetAudience := range cmp.Audiences {
					if s.Audiences.IsAllowed(targetAudience, inf.EmailAddress) {
						// There was an audience target and they're in it
						catFound = true
						break
					}
				}
			}

			if !catFound {
				continue
			}
		}

		if len(cmp.Keywords) > 0 {
			kwFound := false
		L2:
			for _, kw := range cmp.Keywords {
				for _, infKw := range inf.Keywords {
					if common.IsExactMatch(kw, infKw) {
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
				continue
			}
		}

		if !geo.IsGeoMatch(cmp.Geos, inf.GetLatestGeo()) {
			continue
		}

		// Gender check
		if !cmp.Male && cmp.Female && !inf.Female {
			// Only want females
			continue
		} else if cmp.Male && !cmp.Female && !inf.Male {
			// Only want males
			continue
		} else if !cmp.Male && !cmp.Female {
			continue
		}

		_, ok := cmp.Whitelist[inf.EmailAddress]
		if ok {
			// This person is already in the campaign!
			continue
		}

		// MAX YIELD
		maxYield := influencer.GetMaxYield(&cmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)
		// if !cmp.IsProductBasedBudget() && len(cmp.Whitelist) == 0 && !s.Cfg.Sandbox {
		// 	// NOTE: Skip this for whitelisted campaigns!

		// 	// OPTIMIZATION: Goal is to distribute funds evenly
		// 	// given what the campaign's influencer goal is and how
		// 	// many funds we have left
		// 	if maxYield < min || maxYield > max || maxYield == 0 {
		// 		continue
		// 	}
		// }

		user := ForecastUser{
			ID:          inf.Id,
			Name:        strings.Title(inf.Name),
			Email:       inf.EmailAddress,
			AvgEngs:     inf.GetAvgEngs(),
			Followers:   inf.GetFollowers(),
			Description: inf.GetDescription(),
			MaxYield:    fmt.Sprintf("$%0.2f", maxYield),
			Rate:        maxYield,
			Geo:         "N/A",
			Gender:      "N/A",
			Categories:  "N/A",
		}
		user.StringFollowers = common.Commanize(user.Followers)

		if geo := inf.GetLatestGeo(); geo != nil {
			if geo.State != "" && geo.Country != "" {
				user.Geo = geo.State + ", " + geo.Country
			} else if geo.State == "" && geo.Country != "" {
				user.Geo = geo.Country
			}
		}

		if inf.Male {
			user.Gender = "M"
		} else if inf.Female {
			user.Gender = "F"
		}

		if len(inf.Categories) > 0 {
			user.Categories = strings.Join(inf.Categories, ", ")
		}

		// Social Media Checks
		socialMediaFound := false
		if cmp.YouTube && inf.YouTube != nil {
			socialMediaFound = true
			if inf.YouTube.ProfilePicture != "" {
				user.ProfilePicture = inf.YouTube.ProfilePicture
			}
			user.URL = inf.YouTube.GetProfileURL()
			user.HasYoutube = true
			user.YoutubeUsername = inf.YouTube.UserName
		}

		if cmp.Instagram && inf.Instagram != nil {
			socialMediaFound = true
			if inf.Instagram.ProfilePicture != "" {
				user.ProfilePicture = inf.Instagram.ProfilePicture
			}
			user.URL = inf.Instagram.GetProfileURL()
			user.HasInsta = true
			user.InstaUsername = inf.Instagram.UserName
		}

		if cmp.Twitter && inf.Twitter != nil {
			socialMediaFound = true
			if inf.Twitter.ProfilePicture != "" {
				user.ProfilePicture = inf.Twitter.ProfilePicture
			}
			user.URL = inf.Twitter.GetProfileURL()
			user.HasTwitter = true
			user.TwitterUsername = inf.Twitter.Id
		}

		if cmp.Facebook && inf.Facebook != nil {
			socialMediaFound = true

			if inf.Facebook.ProfilePicture != "" {
				user.ProfilePicture = inf.Facebook.ProfilePicture
			}
			user.URL = inf.Facebook.GetProfileURL()
			user.HasFacebook = true
			user.FacebookUsername = inf.Facebook.Id
		}

		if !socialMediaFound {
			continue
		}

		// Lets check to see a match on eng, follower, price targeting now that
		// we have those values
		// NOTE: For scraps this is done within the Match func
		if cmp.EngTarget != nil && !cmp.EngTarget.InRange(user.AvgEngs) {
			continue
		}

		if cmp.FollowerTarget != nil && !cmp.FollowerTarget.InRange(user.Followers) {
			continue
		}

		// Lets see if max yield falls into target range for the campaign
		if cmp.PriceTarget != nil && cmp.PriceTarget.InRange(maxYield) {
			continue
		}

		influencers = append(influencers, user)
	}

	// Get budget store beforehand because we'll only be touching this one campaign
	tmpStore, _ := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)

	scrapUsers := []ForecastUser{}
	for _, sc := range s.Scraps.GetStore() {
		if sc.Match(cmp, s.Audiences, s.db, s.Cfg, tmpStore, true) {
			_, ok := cmp.Whitelist[sc.EmailAddress]
			if ok {
				// This person is already in the campaign!
				continue
			}

			user := ForecastUser{
				ID:          "sc-" + sc.Id,
				Name:        strings.Title(sc.Name),
				Email:       sc.EmailAddress,
				AvgEngs:     sc.GetAvgEngs(),
				Followers:   sc.GetFollowers(),
				Description: sc.GetDescription(),
				Geo:         "N/A",
				Gender:      "N/A",
				Categories:  "N/A",
			}
			user.Rate = influencer.GetMaxYield(&cmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)
			user.MaxYield = fmt.Sprintf("$%0.2f", user.Rate)
			user.StringFollowers = common.Commanize(user.Followers)

			if geo := sc.Geo; geo != nil {
				if geo.State != "" && geo.Country != "" {
					user.Geo = geo.State + ", " + geo.Country
				} else if geo.State == "" && geo.Country != "" {
					user.Geo = geo.Country
				}
			}

			if sc.Male {
				user.Gender = "M"
			} else if sc.Female {
				user.Gender = "F"
			}

			if len(sc.Categories) > 0 {
				user.Categories = strings.Join(sc.Categories, ", ")
			}

			if sc.FBData != nil {
				if sc.FBData.ProfilePicture != "" {
					user.ProfilePicture = sc.FBData.ProfilePicture
				}
				user.URL = sc.FBData.GetProfileURL()
				user.HasFacebook = true
				user.FacebookUsername = sc.FBData.Id
			}

			if sc.InstaData != nil {
				if sc.InstaData.ProfilePicture != "" {
					user.ProfilePicture = sc.InstaData.ProfilePicture
				}
				user.URL = sc.InstaData.GetProfileURL()
				user.HasInsta = true
				user.InstaUsername = sc.InstaData.UserName
			}

			if sc.TWData != nil {
				if sc.TWData.ProfilePicture != "" {
					user.ProfilePicture = sc.TWData.ProfilePicture
				}
				user.URL = sc.TWData.GetProfileURL()
				user.HasTwitter = true
				user.TwitterUsername = sc.TWData.Id
			}

			if sc.YTData != nil {
				if sc.YTData.ProfilePicture != "" {
					user.ProfilePicture = sc.YTData.ProfilePicture
				}
				user.URL = sc.YTData.GetProfileURL()
				user.HasYoutube = true
				user.YoutubeUsername = sc.YTData.UserName
			}

			scrapUsers = append(scrapUsers, user)
		}
	}

	// Shuffle the scrap users IF theres no sort by
	if sortBy == "" {
		for i := range scrapUsers {
			j := rand.Intn(i + 1)
			scrapUsers[i], scrapUsers[j] = scrapUsers[j], scrapUsers[i]
		}
	}

	// Now that we POTENTIALLY shuffled scrap users.. add them to the influencers array
	influencers = append(influencers, scrapUsers...)

	// Get reach
	for _, i := range influencers {
		reach += i.Followers
	}

	// Lets calculate count and reach now
	switch sortBy {
	case "engagements":
		sort.Slice(influencers, func(i int, j int) bool {
			return influencers[i].AvgEngs > influencers[j].AvgEngs
		})
	case "followers":
		sort.Slice(influencers, func(i int, j int) bool {
			return influencers[i].Followers > influencers[j].Followers
		})
	}

	// Lets save this in the cache for later use!
	token = s.Forecasts.Set(influencers, reach)

	total = len(influencers)

	// Lets factor in the start and results index that may be passed in
	influencers = index(influencers, indexStart, maxResults)

	return
}

func getForecast(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Gets influencer count and reach for an incoming campaign struct
		// NOTE: Ignores budget values

		if deleteToken := c.Query("deleteToken"); deleteToken != "" {
			// IF the UI is asking us to delete a token which is no longer in use.. delete it!
			s.Forecasts.Delete(deleteToken)
		}

		var (
			cmp common.Campaign
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&cmp); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		start := int64(-1)
		results := int64(-1)
		if st := c.Query("start"); st != "" {
			start, _ = strconv.ParseInt(st, 10, 64)
		}

		if rs := c.Query("results"); rs != "" {
			results, _ = strconv.ParseInt(rs, 10, 64)
		}

		influencers, total, reach, token := getForecastForCmp(s, cmp, c.Query("sortBy"), c.Query("token"), int(start), int(results))
		if start != -1 && results != -1 { // keep the old behaviour
			influencers = filterForecast(influencers, int(results))
			misc.WriteJSON(c, 200, gin.H{"influencers": total, "reach": reach, "breakdown": influencers, "token": token})
		} else {
			// Default to totals
			misc.WriteJSON(c, 200, gin.H{"influencers": total, "reach": reach, "token": token})
		}
	}
}

func getForecastExport(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&cmp); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		influencers, _, _, _ := getForecastForCmp(s, cmp, "", "", 0, 0)

		if len(cmp.Whitelist) == 0 {
			// If no whitelist cap at 50
			bd := 50
			if bd > len(influencers) {
				bd = len(influencers)
			}

			influencers = filterForecast(influencers, bd)
		}

		var numInfs int
		if cmp.Perks != nil && cmp.Perks.Count != 0 {
			numInfs = cmp.Perks.Count
		} else if len(cmp.Whitelist) != 0 {
			numInfs = len(cmp.Whitelist)
		} else {
			// Calculate based on avg influencer yield in our platform
			yield := s.auth.Influencers.GetAvgYield()
			numInfs = int(cmp.Budget / yield)
			if numInfs < 3 && cmp.Budget > 100 {
				numInfs = 3
			}
		}

		load := map[string]interface{}{
			"Influencers":         influencers,
			"NumberOfInfluencers": strconv.Itoa(numInfs),
			"LikelyEngagements":   fmt.Sprintf("%0.2f", cmp.Budget/(budget.INSTA_LIKE)),
			"Budget":              fmt.Sprintf("$%0.2f", cmp.Budget),
			"TwitterIcon":         TwitterIcon,
			"YoutubeIcon":         YoutubeIcon,
			"InstaIcon":           InstaIcon,
			"FacebookIcon":        FacebookIcon,
		}
		tmpl := templates.ForecastExport.Render(load)

		c.Header("Content-type", "application/octet-stream")
		c.Header("Content-Disposition", fmt.Sprintf("attachment;Filename=%s.pdf", cmp.Name+"_forecast"))

		if err := pdf.ConvertHTMLToPDF(tmpl, c.Writer, s.Cfg); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
		}
	}
}

func filterForecast(infs []ForecastUser, bd int) (out []ForecastUser) {
	for _, inf := range infs {
		if len(out) >= bd {
			return
		}

		if inf.IsProfilePictureActive() {
			out = append(out, inf)
		}
	}
	return
}

func index(users []ForecastUser, start, results int) []ForecastUser {
	if start == -1 || results == -1 || start > len(users) { // we're over the list, return empty
		return nil
	}

	end := start + results
	if end > len(users) {
		end = len(users)
	}

	//log.Println(len(users), len(users[start:end]), start, end, results)
	return users[start:end]
}
