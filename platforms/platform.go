package platform

const (
	Twitter   = "twitter"
	Facebook  = "facebook"
	Instagram = "instagram"
	YouTube   = "youtube"
)

var ALL_PLATFORMS = map[string]struct{}{
	Twitter:   struct{}{},
	Facebook:  struct{}{},
	Instagram: struct{}{},
	YouTube:   struct{}{},
}
