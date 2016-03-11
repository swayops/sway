package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

type ItemType string

// update this as we add new item types
const (
	CampaignItem         ItemType = "camp"
	TalentAgencyItem     ItemType = "talent"
	AdvertiserAgencyItem ItemType = "adv"
)

func (a *Auth) SetOwnerTx(tx *bolt.Tx, itemType ItemType, itemId, userId string) error {
	b := misc.GetBucket(tx, a.cfg.Bucket.Ownership)
	return b.Put(getOwnersKey(itemType, itemId), []byte(userId))
}

func (a *Auth) IsOwner(tx *bolt.Tx, itemType ItemType, itemId, userId string) bool {
	b := misc.GetBucket(tx, a.cfg.Bucket.Ownership)
	return string(b.Get(getOwnersKey(itemType, itemId))) == userId
}

func (a *Auth) DelOwnedItem(tx *bolt.Tx, itemType ItemType, itemId string) error {
	b := misc.GetBucket(tx, a.cfg.Bucket.Ownership)
	return b.Delete(getOwnersKey(itemType, itemId))
}
