module instantdeploy/backend

go 1.22.0

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.7.1
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.7.0
	golang.org/x/crypto v0.27.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace (
	golang.org/x/crypto         => github.com/golang/crypto v0.27.0
	golang.org/x/sync           => github.com/golang/sync v0.8.0
	golang.org/x/sys            => github.com/golang/sys v0.25.0
	golang.org/x/text           => github.com/golang/text v0.18.0
	google.golang.org/protobuf  => github.com/protocolbuffers/protobuf-go v1.34.2
	gopkg.in/yaml.v3            => github.com/go-yaml/yaml/v3 v3.0.1
	gopkg.in/check.v1           => github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e
)
