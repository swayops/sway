package instagram

type Instagram struct {
	Id              string
	LikesPerPost    int
	CommentsPerPost int
	Followers       int
	LastLocation    string //TBD
}

func New(id, endpoint string) (*Instagram, error) {
	in := &Instagram{
		Id: id,
	}
	err := in.UpdateData(endpoint)
	return in, err
}

func (in *Instagram) UpdateData(endpoint string) error {
	// Used by an eventual ticker to update stats
	if in.Id != "" {
		if likes, err := getLikes(in.Id, endpoint); err == nil {
			in.LikesPerPost = likes
		} else {
			return err
		}

		if cm, err := getComments(in.Id, endpoint); err == nil {
			in.CommentsPerPost = cm
		} else {
			return err
		}

		if fl, err := getFollowers(in.Id, endpoint); err == nil {
			in.Followers = fl
		} else {
			return err
		}
	}
	return nil
}

func getLikes(id, endpoint string) (int, error) {
	return 0, nil
}

func getComments(id, endpoint string) (int, error) {
	return 0, nil
}

func getFollowers(id, endpoint string) (int, error) {
	return 0, nil
}
