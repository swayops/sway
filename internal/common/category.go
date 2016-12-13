package common

// Categories to keywords map
var CATEGORIES = map[string][]string{
	"arts and entertainment": []string{"movie", "dance", "music", "film", "entertain", "entertainment", "art"},
	"automotive":             []string{"racecar", "automotive", "auto", "car", "cars", "convertible", "truck"},
	"business":               []string{"business", "company"},
	"career":                 []string{"education", "business", "school", "work"},
	"family":                 []string{"family", "baby", "kids", "couple", "siblings", "love", "kid", "child", "children", "babies"},
	"health":                 []string{"fitness", "run", "gym", "running", "outdoor", "swim", "bike", "biking", "lift", "athelete"},
	"food and drink":         []string{"food", "wine", "drink", "cup", "burger", "meat", "sandwich"},
	"hobbies":                []string{"bike", "run", "paint", "art", "biking", "motorcycle", "surf", "surfing", "basketball", "soccer", "football", "hockey", "horse", "athelete", "scuba", "snowboarding", "skiier", "ski", "skiing", "climbing", "climber", "skate", "skateboarder", "skateboarding", "craft", "game", "gaming"},
	"home":                   []string{"cook", "cooking", "decoration", "decorating", "home", "garden", "flower", "baby", "kid", "child", "family"},
	"law and politics":       []string{"law", "politics", "vote", "lawyer", "politician", "political", "court"},
	"news":                   []string{"news", "article", "writing", "writer", "editor", "editorial"},
	"personal finance":       []string{"finance", "money", "cash", "debt", "loan", "credit", "card"},
	"dating and marriage":    []string{"couple", "dating", "married", "wedding", "date", "dating"},
	"science":                []string{"universe", "space", "sky", "tech", "technology", "computer"},
	"pets":                   []string{"dog", "cat", "dogs", "cats", "terrier", "hound", "pet", "pets", "puppy", "animals", "animal", "horse", "turtle", "bunny", "snake", "lizard", "ferret", "pig"},
	"sports":                 []string{"race", "racecar", "fight", "fighting", "bike", "run", "paint", "art", "biking", "motorcycle", "surf", "surfing", "basketball", "soccer", "football", "hockey", "horse", "athelete", "scuba", "snowboarding", "skiier", "ski", "skiing", "climbing", "climber", "skate", "skateboarder", "skateboarding", "craft", "game", "gaming"},
	"fashion":                []string{"fashion", "attractive", "model", "clothes", "bikini"},
	"technology":             []string{"computer", "tech", "technology", "program", "programming", "phone"},
	"travel":                 []string{"beach", "sky", "ocean", "mountain", "plane", "flying", "airplane", "bikini"},
	"real estate":            []string{"listing", "home", "garden", "flower", "baby", "kid", "child", "family"},
	"shopping":               []string{"shop", "mall", "buy", "bags", "clothes", "attractive", "women", "model"},
	"spirituality":           []string{"god", "jesus", "church", "bible"},
}

func KwToCategories(kws []string) (cats []string) {
	for _, kw := range kws {
		for cat, mappedCats := range CATEGORIES {
			for _, catKw := range mappedCats {
				if kw == catKw && !IsInList(cats, cat) {
					cats = append(cats, cat)
				}
			}
		}
	}
	return
}
