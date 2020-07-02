module github.com/TicketsBot/worker

go 1.14

require (
	github.com/TicketsBot/archiverclient v0.0.0-20200702201017-5511592e1e71
	github.com/TicketsBot/common v0.0.0-20200702195837-7afe5e77d1df
	github.com/TicketsBot/database v0.0.0-20200622220825-872a8f13fc38
	github.com/TicketsBot/logarchiver v0.0.0-20200425163447-199b93429026 // indirect
	github.com/elliotchance/orderedmap v1.2.1
	github.com/go-redis/redis v6.15.8+incompatible
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/jackc/pgtype v1.3.0
	github.com/jackc/pgx/v4 v4.6.0
	github.com/klauspost/compress v1.10.10 // indirect
	github.com/rxdn/gdl v0.0.0-20200618155240-edb9ae72ef6e
	github.com/sirupsen/logrus v1.5.0
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	gopkg.in/alexcesaro/statsd.v2 v2.0.0
)
