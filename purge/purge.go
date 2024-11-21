package purge

import (
	"slices"

	"github.com/disgoorg/snowflake/v2"
)

type Purge struct {
	UserID snowflake.ID

	BulkLimit int
	StartID   snowflake.ID
	EndID     snowflake.ID

	Forwards bool

	exclude []snowflake.ID
	include []snowflake.ID

	Running bool
}

func (p *Purge) Excluded() []snowflake.ID {
	if len(p.include) == 0 {
		return p.exclude
	}
	return slices.DeleteFunc(p.exclude, func(id snowflake.ID) bool {
		return slices.Contains(p.include, id)
	})
}
