package admin

import (
	"fmt"
	"github.com/TicketsBot/common/permission"
	"github.com/TicketsBot/worker/bot/command"
	"github.com/TicketsBot/worker/bot/utils"
	"github.com/rxdn/gdl/rest"
)

type AdminSeedCommand struct {
}

func (AdminSeedCommand) Properties() command.Properties {
	return command.Properties{
		Name:            "seed",
		Description:     "Seeds the cache with members",
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		AdminOnly:       true,
	}
}

func (AdminSeedCommand) Execute(ctx command.CommandContext) {
	var guilds []uint64
	guilds = []uint64{ctx.GuildId}

	ctx.SendEmbedRaw(utils.Green, "Admin", fmt.Sprintf("Seeding %d guild(s)", len(guilds)))

	// retrieve all guild members
	var seeded int
	for _, guildId := range guilds {
		moreAvailable := true
		after := uint64(0)

		for moreAvailable {
			// calling this func will cache for us
			members, _ := ctx.Worker.ListGuildMembers(guildId, rest.ListGuildMembersData{
				Limit: 1000,
				After: after,
			})

			if len(members) < 1000 {
				moreAvailable = false
			}

			if len(members) > 0 {
				after = members[len(members) - 1].User.Id
			}
		}

		seeded++

		if seeded % 10 == 0 {
			ctx.SendEmbedRaw(utils.Green, "Admin", fmt.Sprintf("Seeded %d / %d guilds", seeded, len(guilds)))
		}
	}

	ctx.SendEmbedRaw(utils.Green, "Admin", "Seeding complete")
}
