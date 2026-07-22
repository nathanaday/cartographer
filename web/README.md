# Cartographer Web

Read-only canvas over the Cartographer store.

## Build (single binary)

    cd web/ui && npm install && npm run build
    cd ../.. && go build -o web/cartographer-web ./web

## Run

    CARTOGRAPHER_STORE_DIR=~/.cartographer/store ./web/cartographer-web --addr :8099

Open http://localhost:8099.

## Dev (hot reload)

    cd web/ui && npm run dev        # Vite on :5173, proxies /api to :8099
    go run ./web                    # API on :8099
