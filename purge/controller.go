package purge

import (
	"slices"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

var (
	durationDay = 24 * time.Hour
)

type Controller struct {
	purges map[snowflake.ID]*Purge
}

func NewController() *Controller {
	return &Controller{
		purges: make(map[snowflake.ID]*Purge),
	}
}

func (c *Controller) Purge(channelID snowflake.ID) *Purge {
	return c.purges[channelID]
}

func (c *Controller) CreatePurge(channelID, userID snowflake.ID) {
	c.purges[channelID] = &Purge{
		UserID: userID,
	}
}

func (c *Controller) SetBulkLimit(channelID snowflake.ID, limit int) {
	c.purges[channelID].BulkLimit = limit
}

func (c *Controller) SetStartID(purge *Purge, messageID snowflake.ID) bool {
	if time.Now().Sub(messageID.Time()) > 14*durationDay {
		return false
	}
	purge.StartID = messageID
	return true
}

func (c *Controller) SetEndID(purge *Purge, messageID snowflake.ID) bool {
	if messageID > purge.StartID {
		if messageID.Time().Sub(purge.StartID.Time()) > 14*durationDay {
			return false
		}
		purge.Forwards = true
	} else if purge.StartID.Time().Sub(messageID.Time()) > 14*durationDay {
		return false
	}
	purge.EndID = messageID
	return true
}

func (c *Controller) ExcludeMessage(purge *Purge, messageID snowflake.ID) bool {
	if slices.Contains(purge.exclude, messageID) {
		return false
	}
	purge.exclude = append(purge.exclude, messageID)
	return true
}

func (c *Controller) IncludeMessage(purge *Purge, messageID snowflake.ID) bool {
	if !slices.Contains(purge.exclude, messageID) {
		return false
	}
	purge.include = append(purge.include, messageID)
	return true
}

func (c *Controller) RemovePurge(channelID snowflake.ID) {
	delete(c.purges, channelID)
}
