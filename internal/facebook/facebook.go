package facebook

type Facebook struct {
	Id              string
	LikesPerPost    int
	CommentsPerPost int
	Followers       int
}

func New(id, endpoint string) (*Facebook, error) {
	fb := &Facebook{
		Id: id,
	}
	err := fb.UpdateData(endpoint)
	return fb, err
}

func (fb *Facebook) UpdateData(endpoint string) error {
	if fb.Id != "" {
		if likes, err := getLikes(fb.Id, endpoint); err == nil {
			fb.LikesPerPost = likes
		} else {
			return err
		}

		if cm, err := getComments(fb.Id, endpoint); err == nil {
			fb.CommentsPerPost = cm
		} else {
			return err
		}

		if fl, err := getFollowers(fb.Id, endpoint); err == nil {
			fb.Followers = fl
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
