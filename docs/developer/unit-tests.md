# Unit tests

go test ./x/gravity/migrations/v2/... -v --count=1
go test ./x/gravity/migrations/v3/... -v --count=1
go test ./x/gravity/migrations/v4/... -v --count=1
go test ./x/gravity/keeper/... -v --count=1