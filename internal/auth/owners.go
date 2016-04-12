package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

type ItemType string

// update this as we add new item types
const (
	AdvertiserAgencyItem ItemType = "advAgency"
	AdvertiserItem       ItemType = "adv"
	CampaignItem         ItemType = "camp"
	TalentAgencyItem     ItemType = "talentAgency"
	InfluencerItem       ItemType = `influencer`
)

func (a *Auth) SetOwnerTx(tx *bolt.Tx, itemType ItemType, itemId, userId string) error {
	b := misc.GetBucket(tx, a.cfg.AuthBucket.Ownership)
	return b.Put(getOwnersKey(itemType, itemId), []byte(userId))
}

func (a *Auth) IsOwnerTx(tx *bolt.Tx, itemType ItemType, itemId, userId string) bool {
	return a.GetOwnerTx(tx, itemType, itemId) == userId
}

func (a *Auth) GetOwnerTx(tx *bolt.Tx, itemType ItemType, itemId string) string {
	b := misc.GetBucket(tx, a.cfg.AuthBucket.Ownership)
	return string(b.Get(getOwnersKey(itemType, itemId)))
}

func (a *Auth) DelOwnedItem(tx *bolt.Tx, itemType ItemType, itemId string) error {
	b := misc.GetBucket(tx, a.cfg.AuthBucket.Ownership)
	return b.Delete(getOwnersKey(itemType, itemId))
}
