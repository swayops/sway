package sharpspring

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"encoding/json"

	"github.com/swayops/sway/config"
)

const apiURL = "https://api.sharpspring.com/pubapi/v1/"

func Post(cfg *config.Config, req interface{}) error {
	j, err := json.Marshal(req)
	if err != nil {
		return err
	}
	qs := "?accountID=" + cfg.SharpSpring.AccountID + "&secretKey=" + cfg.SharpSpring.APIKey

	if debug {
		log.Printf("%s\n%s", j, apiURL+qs)
	}

	resp, err := http.Post(apiURL+qs, "application/json", bytes.NewReader(j))
	if err != nil {
		return err
	}
	return parseResponse(resp.Body)
}

func CreateLead(cfg *config.Config, aid, oid, name, email, desc string) error {
	ll := NewLeads(aid, "createLeads", []*Lead{
		NewLead(aid, oid, name, email, desc),
	})
	return Post(cfg, ll)
}

// not working
// func DeleteLead(cfg *config.Config, aid, email string) error {
// 	ll := NewLeads(aid, "deleteLeads", []Lead{{ID: aid, Email: email}})
// 	return Post(cfg, ll)
// }

type Lead struct {
	ID      string `json:"id,omitempty"`
	OwnerID string `json:"ownerID,omitempty"`
	Name    string `json:"companyName,omitempty"`
	Email   string `json:"emailAddress,omitempty"`
	Status  string `json:"leadStatus,omitempty"`
	Desc    string `json:"description,omitempty"`
	TS      int64  `json:"updateTimestamp,omitempty"`
}

func NewLead(aid, oid, name, email, desc string) *Lead {
	return &Lead{
		ID:      aid,
		OwnerID: oid,
		Name:    name,
		Email:   email,
		Status:  "open",
		Desc:    desc,
		TS:      time.Now().Unix() * 1000,
	}
}

type Leads struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params struct {
		Objects interface{} `json:"objects"`
	} `json:"params"`
}

func NewLeads(id, method string, obj interface{}) *Leads {
	l := &Leads{
		ID:     method + ":" + id,
		Method: method,
	}
	l.Params.Objects = obj
	return l
}
