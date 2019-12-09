# Server

## With Docker
```bash
docker build -t shortnersearcher:server .
docker run -d -p 8080:8080 shortnersearcher:server
```

## Without Docker
```bash
go get github.com/gorilla/websocket
go run server/main.go
```

## Adding permutations
Assuming a file called permutations with one url per line called permutations.txt:
```bash
curl localhost:8080/permutations -F "file=@permutations.txt"
```

Adding a single permutation:
```bash
curl localhost:8080/permutations?url=https://bit.ly/maroonlake
```

# Client

## With Docker
```bash
docker build -t shortnersearcher:client .
docker run -it --network="host" -e CLIENT_HOST=localhost -e CLIENT_HOSTNAME=a shortnersearcher:client
```

## Without Docker
```bash
go get github.com/gorilla/websocket
go run client/main.go --hostname a --host localhost
```